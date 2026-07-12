# Partial backend config consumed via:
#   terraform init -backend-config=backend.hcl
#
# Values come from the bootstrap module's outputs
# (infrastructure/terraform/bootstrap) — copy them in after the one-time
# bootstrap apply. Not templated with real values here since the actual
# bucket name includes an account-specific unique suffix chosen at
# bootstrap time, and this file is committed to the repo (bucket/table
# names aren't secrets, so committing them is fine and lets `terraform init`
# work the same way for every engineer without a manual step beyond this file).

bucket         = "REPLACE-WITH-bootstrap.state_bucket_name"
region         = "us-east-1"
dynamodb_table = "REPLACE-WITH-bootstrap.lock_table_name"
