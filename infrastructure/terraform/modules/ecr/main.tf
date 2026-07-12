locals {
  name_prefix = "${var.project_name}-${var.environment}"

  mandatory_tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
    Project     = var.project_name
  }
  tags = merge(local.mandatory_tags, var.tags)

  repository_set = toset(var.repository_names)
}

# ---------------------------------------------------------------------------
# KMS key for repository encryption at rest (only created when
# encryption_type = "KMS", the default — see variables.tf rationale).
# ---------------------------------------------------------------------------

resource "aws_kms_key" "ecr" {
  count = var.encryption_type == "KMS" ? 1 : 0

  description             = "Encryption key for ${local.name_prefix} ECR repositories."
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = merge(local.tags, {
    Name = "${local.name_prefix}-ecr"
  })
}

resource "aws_kms_alias" "ecr" {
  count = var.encryption_type == "KMS" ? 1 : 0

  name          = "alias/${local.name_prefix}-ecr"
  target_key_id = aws_kms_key.ecr[0].key_id
}

# ---------------------------------------------------------------------------
# One ECR repository per entry in var.repository_names (M1 §2: "ECR repo per
# service"). Starts with just auth-service; other services are added to the
# list, not pre-created now, per ADR-6 reference-service-first ordering.
#
# prevent_destroy = true per M1 §5 blast-radius table: "Deleting the ECR
# module from state would delete image repos (and images)" is an accepted
# risk mitigated by this lifecycle guard. Removing a repository from
# var.repository_names, or destroying this module, requires the two-step
# manual process documented in the runbook (temporarily lift the guard,
# confirm intent, then remove) — never a one-step `terraform apply`.
# ---------------------------------------------------------------------------

resource "aws_ecr_repository" "this" {
  for_each = local.repository_set

  name                 = "${var.project_name}/${each.value}"
  image_tag_mutability = var.image_tag_mutability

  image_scanning_configuration {
    scan_on_push = var.scan_on_push
  }

  encryption_configuration {
    encryption_type = var.encryption_type
    kms_key         = var.encryption_type == "KMS" ? aws_kms_key.ecr[0].arn : null
  }

  # force_delete stays false (default): a repository still holding images
  # must be emptied deliberately, not wiped as a side effect of a Terraform
  # apply that happens to touch this resource.
  tags = merge(local.tags, {
    Name    = "${local.name_prefix}-${each.value}"
    Service = each.value
  })

  lifecycle {
    prevent_destroy = true
  }
}

# ---------------------------------------------------------------------------
# Lifecycle policy per repository: expire untagged images after N days
# (they have no manifest-repo reference pointing at them — M1 §5/§6),
# retain the most recent N tagged images (git-SHA tags from M3 §2) so a
# rollback (M4 §7) can reference an older, still-present SHA.
#
# Rule evaluation order matters: untagged expiry is rulePriority 1 so it's
# evaluated first: ECR applies rules in priority order and each image is
# only matched by the first rule it satisfies.
# ---------------------------------------------------------------------------

resource "aws_ecr_lifecycle_policy" "this" {
  for_each = local.repository_set

  repository = aws_ecr_repository.this[each.value].name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images after ${var.untagged_image_expiry_days} days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = var.untagged_image_expiry_days
        }
        action = {
          type = "expire"
        }
      },
      {
        rulePriority = 2
        description  = "Keep only the most recent ${var.tagged_image_count_to_keep} tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["*"] # matches any tag — CI tags images directly with git SHA (M3 §2), no fixed prefix scheme
          countType     = "imageCountMoreThan"
          countNumber   = var.tagged_image_count_to_keep
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
