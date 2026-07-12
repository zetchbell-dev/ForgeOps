# Partial backend config consumed via:
#   terraform init -backend-config=backend.hcl
#
# Values come from the bootstrap module's outputs
# (infrastructure/terraform/bootstrap) — same bucket and lock table as every
# other environment (one shared backend, one state file per environment via
# the `key` in versions.tf). Copy in the real bucket/table names after the
# one-time bootstrap apply.

bucket         = "REPLACE-WITH-bootstrap.state_bucket_name"
region         = "us-east-1"
dynamodb_table = "REPLACE-WITH-bootstrap.lock_table_name"
