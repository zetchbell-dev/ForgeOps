output "vpc_id" {
  description = "Prod VPC ID."
  value       = module.vpc.vpc_id
}

output "cluster_name" {
  description = "Prod EKS cluster name — used by ArgoCD's cluster registration (M4) and by `aws eks update-kubeconfig`."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "Prod EKS API server endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "Prod cluster CA data."
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "oidc_provider_arn" {
  description = "Prod cluster's OIDC provider ARN."
  value       = module.eks.oidc_provider_arn
}

output "cluster_autoscaler_role_arn" {
  description = "IRSA role ARN to annotate onto prod's cluster-autoscaler ServiceAccount."
  value       = module.iam.cluster_autoscaler_role_arn
}

output "repository_urls" {
  description = "ECR repository URLs, passed through from dev's state (see data.tf) — prod deploys the same images dev built and staging validated, never its own separately-built ones (M4 §4 promotion flow)."
  value       = data.terraform_remote_state.dev.outputs.repository_urls
}
