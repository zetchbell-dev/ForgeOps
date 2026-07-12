output "vpc_id" {
  description = "Dev VPC ID."
  value       = module.vpc.vpc_id
}

output "cluster_name" {
  description = "Dev EKS cluster name — used by `aws eks update-kubeconfig` and by ArgoCD's cluster registration (M4)."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "Dev EKS API server endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "Dev cluster CA data, for building a kubeconfig from CI without an interactive `aws eks update-kubeconfig` call."
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "oidc_provider_arn" {
  description = "Dev cluster's OIDC provider ARN — needed by any future module adding an IRSA role beyond cluster-autoscaler."
  value       = module.eks.oidc_provider_arn
}

output "cluster_autoscaler_role_arn" {
  description = "IRSA role ARN to annotate onto the cluster-autoscaler ServiceAccount (done at the Helm/K8s layer, M6+)."
  value       = module.iam.cluster_autoscaler_role_arn
}

output "repository_urls" {
  description = "Map of service name -> ECR repository URL. Shared across dev/staging/prod (see main.tf comment on module \"ecr\") — this is the one output from this root module that staging/prod read via terraform_remote_state rather than recomputing."
  value       = module.ecr.repository_urls
}

output "repository_arns" {
  description = "Map of service name -> ECR repository ARN, for scoping the CI OIDC role's push permission (M3 §4)."
  value       = module.ecr.repository_arns
}
