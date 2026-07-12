variable "aws_region" {
  description = "AWS region for the dev environment."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used in resource naming and tagging across every module."
  type        = string
  default     = "enterprise-cicd-platform"
}

variable "environment" {
  description = "Fixed at \"dev\" for this root module — environments/staging and environments/prod are separate root modules with their own state, per M1 §3 (one state file per environment, not conditionals in a shared config)."
  type        = string
  default     = "dev"

  validation {
    condition     = var.environment == "dev"
    error_message = "This root module is environments/dev — environment must be \"dev\". Use environments/staging or environments/prod for other environments."
  }
}

# --- VPC ---------------------------------------------------------------

variable "vpc_cidr_block" {
  description = "CIDR block for the dev VPC. /16 per M1 §4 sizing convention."
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of AZs for dev. 2 is the EKS minimum; dev doesn't need prod's 3-AZ HA posture."
  type        = number
  default     = 2
}

variable "single_nat_gateway" {
  description = "Single shared NAT Gateway for dev — accepted SPOF per M1 §4/§7 cost tradeoff (prod sets this false)."
  type        = bool
  default     = true
}

# --- EKS -----------------------------------------------------------------

variable "cluster_name" {
  description = "EKS cluster name for dev."
  type        = string
  default     = "enterprise-cicd-platform-dev"
}

variable "cluster_version" {
  description = "Kubernetes version, pinned identically across dev/staging/prod per M1 §7 (only tfvars differ, not the version)."
  type        = string
  default     = "1.29"
}

variable "endpoint_public_access_cidrs" {
  description = "CIDR blocks allowed to reach the public EKS API endpoint in dev: CI runner IP range and break-glass admin IPs (M1 §6). No default — every deployer must supply their own, real ranges rather than inherit a placeholder that looks safe but isn't scoped to them."
  type        = list(string)
}

variable "node_instance_types" {
  description = "Dev node group instance types — smaller than prod's, since dev doesn't carry production traffic."
  type        = list(string)
  default     = ["t3.large"]
}

variable "node_desired_size" {
  description = "Dev node group desired size."
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Dev node group minimum size."
  type        = number
  default     = 1
}

variable "node_max_size" {
  description = "Dev node group maximum size."
  type        = number
  default     = 3
}

# --- ECR -------------------------------------------------------------------

variable "repository_names" {
  description = "Services with an ECR repository. Starts with just auth-service per ADR-6 reference-service-first ordering. ECR is an account/region-level registry resource, not a per-environment one, so module.ecr is instantiated here in environments/dev only — see the comment above module \"ecr\" in main.tf for why staging/prod don't repeat it."
  type        = list(string)
  default     = ["auth-service"]
}

variable "tags" {
  description = "Additional tags merged into every resource across every module, on top of each module's own mandatory Environment/ManagedBy/Project tags."
  type        = map(string)
  default     = {}
}
