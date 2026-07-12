# Staging does not instantiate module.ecr (see environments/dev/main.tf and
# README for why: ECR is an account/region-scoped registry resource shared
# across every environment, not per-environment). Instead it reads dev's
# already-applied outputs read-only, via the same S3 backend every
# environment's state lives in.
#
# This is a read of dev's *state file*, not a dependency on dev's module
# graph being re-evaluated — staging's plan/apply does not re-run anything
# in modules/ecr, it only reads two output values that were already computed.

data "terraform_remote_state" "dev" {
  backend = "s3"

  config = {
    bucket = var.state_bucket_name
    key    = "envs/dev/terraform.tfstate"
    region = var.aws_region
  }
}
