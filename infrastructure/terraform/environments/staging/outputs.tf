output "vpc_id" {
  description = "Staging VPC ID."
  value       = module.vpc.vpc_id
}

output "cluster_name" {
  description = "Staging EKS cluster name."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "Staging EKS API server endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "Staging cluster CA data."
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "oidc_provider_arn" {
  description = "Staging cluster's OIDC provider ARN."
  value       = module.eks.oidc_provider_arn
}

output "cluster_autoscaler_role_arn" {
  description = "IRSA role ARN to annotate onto staging's cluster-autoscaler ServiceAccount."
  value       = module.iam.cluster_autoscaler_role_arn
}

output "repository_urls" {
  description = "ECR repository URLs, passed through from dev's state (see data.tf) — staging deploys the same images dev proved, never its own separately-built ones."
  value       = data.terraform_remote_state.dev.outputs.repository_urls
}
