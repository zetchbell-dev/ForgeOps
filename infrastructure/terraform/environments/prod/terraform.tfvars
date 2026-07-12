# Explicit per-environment values (M1 §4: explicit files per environment,
# not a single tfvars with conditionals). Sizing/scoping is the HA baseline
# every other environment is set relative to (3 AZs, per-AZ NAT).

aws_region   = "us-east-1"
project_name = "enterprise-cicd-platform"
environment  = "prod"

# Must match the bootstrap module's state_bucket_name output — used by
# data.tf to read environments/dev's state (ECR repository info) via
# terraform_remote_state.
state_bucket_name = "REPLACE-WITH-bootstrap.state_bucket_name"

# --- VPC ---
vpc_cidr_block     = "10.2.0.0/16"
az_count           = 3
single_nat_gateway = false # one NAT per AZ, HA posture required for prod

# --- EKS ---
cluster_name    = "enterprise-cicd-platform-prod"
cluster_version = "1.29"

# REQUIRED — replace with your actual CI runner IP range(s) and break-glass
# admin IP(s) before applying. No default in variables.tf, same reasoning
# as environments/dev and environments/staging. Keep this list as tight as
# possible for prod (M1 §6: private + restricted public access, not fully
# public).
endpoint_public_access_cidrs = [
  "203.0.113.0/24", # example — replace with the real CI runner range
  "198.51.100.5/32" # example — replace with a real break-glass admin IP
]

node_instance_types = ["m6i.xlarge"]
node_desired_size   = 3
node_min_size       = 3
node_max_size       = 6

# --- Tags ---
tags = {
  Team = "platform"
}
