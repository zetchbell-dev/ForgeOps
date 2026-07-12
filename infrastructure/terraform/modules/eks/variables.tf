variable "environment" {
  description = "Environment name (dev, staging, prod). Used for naming/tagging only — module logic does not branch on this per M1 §4."
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

variable "cluster_name" {
  description = "Name of the EKS cluster. Also used as the tag value ASGs must carry (k8s.io/cluster-autoscaler/<cluster_name> = owned) for module.iam's cluster-autoscaler policy condition to match."
  type        = string
}

variable "cluster_version" {
  description = "Kubernetes control plane version. Pinned explicitly (not left to provider default) so dev/staging/prod stay on the same version by construction per M1 §7 — only tfvars differ, not module behavior."
  type        = string
  default     = "1.29"

  validation {
    condition     = can(regex("^1\\.(2[7-9]|3[0-9])$", var.cluster_version))
    error_message = "cluster_version must be a supported EKS minor version in the form \"1.NN\" (e.g. \"1.29\")."
  }
}

variable "vpc_id" {
  description = "VPC ID from module.vpc. The cluster's security group and node ENIs are provisioned inside this VPC."
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs from module.vpc (module.vpc.private_subnet_ids). Both the control plane's cross-account ENIs and the managed node group are placed here exclusively — nodes never run in a public subnet (M1 §6)."
  type        = list(string)

  validation {
    condition     = length(var.private_subnet_ids) >= 2
    error_message = "private_subnet_ids must contain at least 2 subnet IDs — EKS requires the control plane to span at least 2 AZs."
  }
}

variable "endpoint_public_access" {
  description = "Whether the EKS API server endpoint is reachable from outside the VPC at all. Per M1 §6 the endpoint is private + restricted public access, not fully public — this flag exists so a fully air-gapped environment could disable public access entirely, but the default keeps it on, gated by endpoint_public_access_cidrs."
  type        = bool
  default     = true
}

variable "endpoint_public_access_cidrs" {
  description = "CIDR blocks allowed to reach the public API endpoint when endpoint_public_access is true — intended to be the CI runner IP range and break-glass admin IPs only (M1 §6), never 0.0.0.0/0. Enforced via validation below rather than left as a convention."
  type        = list(string)
  default     = []

  validation {
    condition     = !contains(var.endpoint_public_access_cidrs, "0.0.0.0/0")
    error_message = "endpoint_public_access_cidrs must not include 0.0.0.0/0 — the design doc requires the public endpoint restricted to CI runner and break-glass admin IPs, not the open internet."
  }
}

variable "enabled_cluster_log_types" {
  description = "EKS control plane log types shipped to CloudWatch Logs. All five enabled by default (api, audit, authenticator, controllerManager, scheduler) — audit and authenticator logs in particular are what an incident investigation needs to answer \"who did what to the API server\", which a bare metrics/dashboard story (M5) doesn't cover on its own."
  type        = list(string)
  default     = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
}

variable "log_retention_days" {
  description = "Retention period for the EKS control plane CloudWatch log group."
  type        = number
  default     = 90

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.log_retention_days)
    error_message = "log_retention_days must be a value CloudWatch Logs accepts (see aws_cloudwatch_log_group docs)."
  }
}

variable "enable_secrets_encryption" {
  description = "If true, Kubernetes Secrets are envelope-encrypted at rest with a dedicated customer-managed KMS key, in addition to the etcd-level encryption EKS provides by default. Default true per AWS Well-Architected / OWASP guidance — Auth Service's credential-adjacent secrets (M2 §5) are exactly the kind of data this protects."
  type        = bool
  default     = true
}

variable "node_instance_types" {
  description = "EC2 instance types for the managed node group, in priority order. Multiple types recommended for Spot-backed groups (capacity diversification); a single type is fine for the on-demand default."
  type        = list(string)
  default     = ["m6i.large"]

  validation {
    condition     = length(var.node_instance_types) > 0
    error_message = "node_instance_types must contain at least one instance type."
  }
}

variable "node_capacity_type" {
  description = "Capacity type for the managed node group: ON_DEMAND or SPOT. ON_DEMAND is the default — Spot is a per-environment tradeoff (fine for dev, riskier for prod without PodDisruptionBudgets already in place), left to the caller rather than hard-coded."
  type        = string
  default     = "ON_DEMAND"

  validation {
    condition     = contains(["ON_DEMAND", "SPOT"], var.node_capacity_type)
    error_message = "node_capacity_type must be either ON_DEMAND or SPOT."
  }
}

variable "node_disk_size" {
  description = "Root EBS volume size (GiB) for each worker node."
  type        = number
  default     = 50

  validation {
    condition     = var.node_disk_size >= 20
    error_message = "node_disk_size must be at least 20 GiB."
  }
}

variable "node_desired_size" {
  description = "Desired worker node count. Autoscaler (module.iam's cluster-autoscaler role) adjusts this within [node_min_size, node_max_size] at runtime; this is only the initial/Terraform-managed value."
  type        = number
  default     = 2

  validation {
    condition     = var.node_desired_size >= 1
    error_message = "node_desired_size must be at least 1."
  }
}

variable "node_min_size" {
  description = "Minimum worker node count the autoscaler will not scale below."
  type        = number
  default     = 1

  validation {
    condition     = var.node_min_size >= 1
    error_message = "node_min_size must be at least 1."
  }
}

variable "node_max_size" {
  description = "Maximum worker node count the autoscaler will not scale above."
  type        = number
  default     = 4

  validation {
    condition     = var.node_max_size >= 1
    error_message = "node_max_size must be at least 1."
  }
}

variable "node_max_unavailable" {
  description = "Max nodes unavailable at once during a managed node group rolling update (M1 §5: rolling replacement causes a brief capacity dip — this bounds how big that dip is)."
  type        = number
  default     = 1

  validation {
    condition     = var.node_max_unavailable >= 1
    error_message = "node_max_unavailable must be at least 1."
  }
}

variable "tags" {
  description = "Additional tags to merge into every resource, on top of the mandatory Environment/ManagedBy/Project tags applied by this module."
  type        = map(string)
  default     = {}
}
