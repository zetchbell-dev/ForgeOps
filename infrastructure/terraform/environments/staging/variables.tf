variable "aws_region" {
  description = "AWS region for the staging environment. Same region as dev/prod in this design — cross-region is not in scope per M0 §2 (multi-region deferred)."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used in resource naming and tagging across every module."
  type        = string
  default     = "enterprise-cicd-platform"
}

variable "environment" {
  description = "Fixed at \"staging\" for this root module."
  type        = string
  default     = "staging"

  validation {
    condition     = var.environment == "staging"
    error_message = "This root module is environments/staging — environment must be \"staging\"."
  }
}

variable "state_bucket_name" {
  description = "Name of the shared Terraform state bucket (from the bootstrap module's output), used to read environments/dev's state for ECR repository info. No default — must match the bucket in this environment's own backend.hcl, and there's no safe placeholder to default to."
  type        = string
}

# --- VPC ---------------------------------------------------------------

variable "vpc_cidr_block" {
  description = "CIDR block for the staging VPC. Distinct /16 from dev's 10.0.0.0/16 — no VPC peering exists between environments in this design (each is fully isolated per M1 §5 blast-radius reasoning), so overlap wouldn't cause a conflict today, but keeping ranges distinct avoids future surprises if peering or Transit Gateway is ever introduced."
  type        = string
  default     = "10.1.0.0/16"
}

variable "az_count" {
  description = "3 AZs for staging — mirrors prod's AZ footprint per M4 §3 (\"staging mirrors prod path for realistic pre-prod validation\"), unlike dev's 2."
  type        = number
  default     = 3
}

variable "single_nat_gateway" {
  description = "false for staging — one NAT Gateway per AZ, mirroring prod's HA posture rather than dev's accepted single-NAT SPOF, consistent with staging's purpose as a realistic pre-prod validation environment."
  type        = bool
  default     = false
}

# --- EKS -----------------------------------------------------------------

variable "cluster_name" {
  description = "EKS cluster name for staging."
  type        = string
  default     = "enterprise-cicd-platform-staging"
}

variable "cluster_version" {
  description = "Same Kubernetes version as dev/prod — pinned identically across environments per M1 §7, only tfvars differ."
  type        = string
  default     = "1.29"
}

variable "endpoint_public_access_cidrs" {
  description = "CIDR blocks allowed to reach the public EKS API endpoint in staging: CI runner range and break-glass admin IPs (M1 §6). No default, same reasoning as environments/dev."
  type        = list(string)
}

variable "node_instance_types" {
  description = "Staging node group instance types — same family as prod's default but the caller may size down; kept as its own variable (not hard-coded equal to prod) since \"mirrors prod\" is a validation-realism goal, not a requirement that staging cost exactly as much as prod."
  type        = list(string)
  default     = ["m6i.large"]
}

variable "node_desired_size" {
  description = "Staging node group desired size."
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Staging node group minimum size."
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Staging node group maximum size."
  type        = number
  default     = 4
}

variable "tags" {
  description = "Additional tags merged into every resource, on top of each module's own mandatory Environment/ManagedBy/Project tags."
  type        = map(string)
  default     = {}
}
