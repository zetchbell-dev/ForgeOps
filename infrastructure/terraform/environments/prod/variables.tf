variable "aws_region" {
  description = "AWS region for the prod environment. Same region as dev/staging in this design — cross-region is not in scope per M0 §2 (multi-region deferred)."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used in resource naming and tagging across every module."
  type        = string
  default     = "enterprise-cicd-platform"
}

variable "environment" {
  description = "Fixed at \"prod\" for this root module."
  type        = string
  default     = "prod"

  validation {
    condition     = var.environment == "prod"
    error_message = "This root module is environments/prod — environment must be \"prod\"."
  }
}

variable "state_bucket_name" {
  description = "Name of the shared Terraform state bucket (from the bootstrap module's output), used to read environments/dev's state for ECR repository info. No default — must match the bucket in this environment's own backend.hcl, and there's no safe placeholder to default to."
  type        = string
}

# --- VPC ---------------------------------------------------------------

variable "vpc_cidr_block" {
  description = "CIDR block for the prod VPC. Distinct /16 from dev's 10.0.0.0/16 and staging's 10.1.0.0/16 — each environment is fully isolated per M1 §5 blast-radius reasoning, and keeping ranges distinct avoids future surprises if peering or Transit Gateway is ever introduced."
  type        = string
  default     = "10.2.0.0/16"
}

variable "az_count" {
  description = "3 AZs for prod — the HA baseline every other environment's az_count is set relative to (M1 §4)."
  type        = number
  default     = 3
}

variable "single_nat_gateway" {
  description = "false for prod — one NAT Gateway per AZ for HA (M1 §4), unlike dev's accepted single-NAT SPOF."
  type        = bool
  default     = false
}

# --- EKS -----------------------------------------------------------------

variable "cluster_name" {
  description = "EKS cluster name for prod."
  type        = string
  default     = "enterprise-cicd-platform-prod"
}

variable "cluster_version" {
  description = "Same Kubernetes version as dev/staging — pinned identically across environments per M1 §7, only tfvars differ."
  type        = string
  default     = "1.29"
}

variable "endpoint_public_access_cidrs" {
  description = "CIDR blocks allowed to reach the public EKS API endpoint in prod: CI runner range and break-glass admin IPs only (M1 §6 — private + restricted public access, never fully public). No default, same reasoning as environments/dev and environments/staging."
  type        = list(string)
}

variable "node_instance_types" {
  description = "Prod node group instance types — sized up from staging's m6i.large to carry real production traffic."
  type        = list(string)
  default     = ["m6i.xlarge"]
}

variable "node_desired_size" {
  description = "Prod node group desired size."
  type        = number
  default     = 3
}

variable "node_min_size" {
  description = "Prod node group minimum size. Never below 3 (one per AZ) so a single node's PodDisruptionBudget-respecting drain doesn't drop below quorum capacity for a 3-AZ deployment."
  type        = number
  default     = 3
}

variable "node_max_size" {
  description = "Prod node group maximum size."
  type        = number
  default     = 6
}

variable "tags" {
  description = "Additional tags merged into every resource, on top of each module's own mandatory Environment/ManagedBy/Project tags."
  type        = map(string)
  default     = {}
}
