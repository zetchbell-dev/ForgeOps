locals {
  mandatory_tags = {
    ManagedBy = "terraform"
    Project   = var.project_name
  }
  tags = merge(local.mandatory_tags, var.tags)
}

# ---------------------------------------------------------------------------
# This module is the one deliberate exception to "every apply goes through
# CI" (M1 §7 risk table): it provisions the state backend CI's own Terraform
# runs depend on, so it cannot itself be run against that backend — chicken
# and egg. It is applied exactly once, manually, with local state, by
# whoever bootstraps a new AWS account/region for this project, and then
# left alone. Its own state file (terraform.tfstate, local) should be stored
# somewhere durable outside of Git (e.g. a password manager or a separate,
# already-existing bucket) — not committed to the repo, since it contains
# resource IDs but not secrets, yet is still operationally important not to
# lose track of.
# ---------------------------------------------------------------------------

resource "aws_s3_bucket" "state" {
  bucket = var.state_bucket_name

  tags = merge(local.tags, {
    Name = var.state_bucket_name
  })
}

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id

  versioning_configuration {
    status = "Enabled" # M1 §3: versioning enabled — a bad apply's prior state is recoverable, not overwritten
  }
}

resource "aws_kms_key" "state" {
  description             = "Encryption key for the ${var.project_name} Terraform state bucket."
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = merge(local.tags, {
    Name = "${var.project_name}-tfstate"
  })
}

resource "aws_kms_alias" "state" {
  name          = "alias/${var.project_name}-tfstate"
  target_key_id = aws_kms_key.state.key_id
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms" # M1 §3: SSE-KMS encryption
      kms_master_key_id = aws_kms_key.state.arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket = aws_s3_bucket.state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Bucket policy restricting write access to the CI role only (M1 §3). Every
# other principal in the account can still read (needed for `terraform plan`
# review tooling and human debugging of state), but only the named CI role
# can PutObject/DeleteObject — enforced here rather than left as a convention,
# per the same "guardrail by construction, not discipline" principle M1 §5
# applies to per-environment state isolation.
data "aws_iam_policy_document" "state_bucket_policy" {
  statement {
    sid    = "DenyUnencryptedTransport"
    effect = "Deny"
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.state.arn, "${aws_s3_bucket.state.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }

  dynamic "statement" {
    for_each = var.ci_role_arn != "" ? [1] : []
    content {
      sid    = "DenyWriteFromNonCIRole"
      effect = "Deny"
      principals {
        type        = "*"
        identifiers = ["*"]
      }
      actions   = ["s3:PutObject", "s3:DeleteObject"]
      resources = ["${aws_s3_bucket.state.arn}/*"]
      condition {
        test     = "StringNotEquals"
        variable = "aws:PrincipalArn"
        values   = [var.ci_role_arn]
      }
    }
  }
}

resource "aws_s3_bucket_policy" "state" {
  bucket = aws_s3_bucket.state.id
  policy = data.aws_iam_policy_document.state_bucket_policy.json
}

# ---------------------------------------------------------------------------
# DynamoDB lock table — on-demand billing per M1 §3 ("low, predictable cost
# for a lock table"). PK is LockID, the fixed attribute name Terraform's S3
# backend requires.
# ---------------------------------------------------------------------------

resource "aws_dynamodb_table" "lock" {
  name         = var.lock_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  server_side_encryption {
    enabled = true
  }

  tags = merge(local.tags, {
    Name = var.lock_table_name
  })
}
