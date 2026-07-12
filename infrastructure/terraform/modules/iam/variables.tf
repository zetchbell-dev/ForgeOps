variable "environment" {
  description = "Environment name (dev, staging, prod). Used for naming/tagging only."
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
  description = "Name of the EKS cluster these IRSA roles are scoped to. Passed in from module.eks.cluster_name — this module does not create the cluster."
  type        = string
}

variable "oidc_provider_arn" {
  description = "ARN of the EKS cluster's IAM OIDC provider. Passed in from module.eks.oidc_provider_arn. This module takes it as an input rather than creating it, so module.iam and module.eks are decoupled at authoring time — Terraform's dependency graph handles apply ordering from the reference in environments/<env>/main.tf."
  type        = string
}

variable "oidc_provider_url" {
  description = "URL of the EKS cluster's OIDC issuer (without the https:// prefix), e.g. oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE. Needed to construct the trust policy condition keys, since those are string-matched against this exact value."
  type        = string
}

variable "cluster_autoscaler_namespace" {
  description = "Kubernetes namespace the cluster-autoscaler service account runs in."
  type        = string
  default     = "kube-system"
}

variable "cluster_autoscaler_service_account_name" {
  description = "Name of the cluster-autoscaler Kubernetes service account, must match the ServiceAccount annotated with this role's ARN via eks.amazonaws.com/role-arn."
  type        = string
  default     = "cluster-autoscaler"
}

variable "tags" {
  description = "Additional tags to merge into every resource, on top of mandatory Environment/ManagedBy/Project tags."
  type        = map(string)
  default     = {}
}
