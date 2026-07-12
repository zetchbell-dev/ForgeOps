# Explicit per-environment values (M1 §4: explicit files per environment,
# not a single tfvars with conditionals). Only sizing/scoping differs from
# staging/prod — module logic itself never branches on environment name.

aws_region   = "us-east-1"
project_name = "enterprise-cicd-platform"
environment  = "dev"

# --- VPC ---
vpc_cidr_block     = "10.0.0.0/16"
az_count           = 2
single_nat_gateway = true # accepted SPOF for dev, per M1 §4/§7 cost tradeoff

# --- EKS ---
cluster_name    = "enterprise-cicd-platform-dev"
cluster_version = "1.29"

# REQUIRED — replace with your actual CI runner IP range(s) and break-glass
# admin IP(s) before applying. Left unset (no default in variables.tf) so a
# copy-pasted example CIDR can't silently become a real, overly-broad grant.
endpoint_public_access_cidrs = [
  "203.0.113.0/24", # example — replace with the real CI runner range
  "198.51.100.5/32" # example — replace with a real break-glass admin IP
]

node_instance_types = ["t3.large"]
node_desired_size   = 2
node_min_size       = 1
node_max_size       = 3

# --- ECR ---
repository_names = ["auth-service"]

# --- Tags ---
tags = {
  Team = "platform"
}
