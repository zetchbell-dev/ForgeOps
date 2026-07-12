# Explicit per-environment values (M1 §4: explicit files per environment,
# not a single tfvars with conditionals). Sizing/scoping mirrors prod's
# topology (3 AZs, per-AZ NAT) rather than dev's, per M4 §3.

aws_region   = "us-east-1"
project_name = "enterprise-cicd-platform"
environment  = "staging"

# Must match the bootstrap module's state_bucket_name output — used by
# data.tf to read environments/dev's state (ECR repository info) via
# terraform_remote_state.
state_bucket_name = "REPLACE-WITH-bootstrap.state_bucket_name"

# --- VPC ---
vpc_cidr_block     = "10.1.0.0/16"
az_count           = 3
single_nat_gateway = false # one NAT per AZ, mirrors prod HA posture

# --- EKS ---
cluster_name    = "enterprise-cicd-platform-staging"
cluster_version = "1.29"

# REQUIRED — replace with your actual CI runner IP range(s) and break-glass
# admin IP(s) before applying. No default in variables.tf, same reasoning
# as environments/dev.
endpoint_public_access_cidrs = [
  "203.0.113.0/24", # example — replace with the real CI runner range
  "198.51.100.5/32" # example — replace with a real break-glass admin IP
]

node_instance_types = ["m6i.large"]
node_desired_size   = 2
node_min_size       = 2
node_max_size       = 4

# --- Tags ---
tags = {
  Team = "platform"
}
