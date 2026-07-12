terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # Partial backend configuration (M1 §3: per-environment state file, S3 +
  # DynamoDB lock). The bucket/table names come from `terraform init
  # -backend-config=backend.hcl` rather than being hard-coded here, so the
  # same versions.tf works unmodified across dev/staging/prod — only
  # backend.hcl's `key` (and, if using a single shared bucket across
  # environments, nothing else) differs per environment.
  backend "s3" {
    key     = "envs/dev/terraform.tfstate"
    encrypt = true
  }
}
