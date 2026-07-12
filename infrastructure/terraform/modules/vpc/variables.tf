variable "environment" {
  description = "Environment name (dev, staging, prod). Used for tagging and naming, never for branching module logic."
  type        = string

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "project_name" {
  description = "Project name used in resource naming and tagging."
  type        = string
  default     = "enterprise-cicd-platform"
}

variable "cidr_block" {
  description = "CIDR block for the VPC. Sized /16 per environment per M1 design doc to leave room for subnet growth without requiring a CIDR change (which would force a full environment rebuild)."
  type        = string

  validation {
    condition     = can(cidrhost(var.cidr_block, 0))
    error_message = "cidr_block must be a valid CIDR block."
  }
}

variable "az_count" {
  description = "Number of Availability Zones to spread subnets across. Minimum 2 for EKS control plane HA requirements."
  type        = number
  default     = 2

  validation {
    condition     = var.az_count >= 2
    error_message = "az_count must be at least 2 — EKS requires subnets in at least 2 AZs."
  }
}

variable "single_nat_gateway" {
  description = "If true, provision a single shared NAT Gateway (accepted single point of failure for non-prod, per M1 cost tradeoff). If false, one NAT Gateway per AZ for HA. Must be false in prod."
  type        = bool
  default     = true
}

variable "enable_dns_hostnames" {
  description = "Enable DNS hostnames in the VPC. Required for EKS."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Additional tags to merge into every resource, on top of the mandatory Environment/ManagedBy/Project tags applied by this module per M1 tagging convention."
  type        = map(string)
  default     = {}
}
