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
  # DynamoDB lock). Bucket/table names come from `terraform init
  # -backend-config=backend.hcl`, same pattern as environments/dev and
  # environments/staging — only the `key` differs per environment.
  backend "s3" {
    key     = "envs/prod/terraform.tfstate"
    encrypt = true
  }
}
