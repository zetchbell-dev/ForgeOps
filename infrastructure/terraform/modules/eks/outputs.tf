output "cluster_name" {
  description = "Name of the EKS cluster. Consumed by module.iam (ASG tag condition) and by CI (M3) for kubectl/ArgoCD context."
  value       = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  description = "EKS API server endpoint URL."
  value       = aws_eks_cluster.this.endpoint
}

output "cluster_certificate_authority_data" {
  description = "Base64-encoded cluster CA certificate, needed to build a kubeconfig without calling `aws eks update-kubeconfig` interactively (e.g. from CI)."
  value       = aws_eks_cluster.this.certificate_authority[0].data
}

output "cluster_security_group_id" {
  description = "ID of the cluster's primary security group (created automatically by EKS). Needed later when wiring RDS/Redis security group ingress rules to allow traffic from cluster pods (M2 infra appendix)."
  value       = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id
}

output "oidc_provider_arn" {
  description = "ARN of the cluster's IAM OIDC provider. Feeds directly into module.iam's oidc_provider_arn input for IRSA trust policies."
  value       = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_provider_url" {
  description = "OIDC issuer URL without the https:// prefix (matches the format module.iam's oidc_provider_url input expects, since IAM trust-policy condition keys are string-matched against this exact value)."
  value       = replace(aws_eks_cluster.this.identity[0].oidc[0].issuer, "https://", "")
}

output "cluster_iam_role_arn" {
  description = "ARN of the EKS cluster's own (control-plane) service role."
  value       = aws_iam_role.cluster.arn
}

output "node_group_id" {
  description = "ID of the default managed node group."
  value       = aws_eks_node_group.this.id
}

output "node_group_role_arn" {
  description = "ARN of the worker node IAM role. Referenced when granting the node role additional access (e.g. an aws-auth ConfigMap entry, or an ECR cross-account pull grant), though no such grant exists yet — kept as a named output rather than requiring a caller to re-derive it."
  value       = aws_iam_role.node.arn
}

output "node_group_status" {
  description = "Current status of the managed node group (e.g. ACTIVE), useful for the post-apply smoke test (M1 §8: confirm kubectl get nodes shows expected node count follows this being ACTIVE)."
  value       = aws_eks_node_group.this.status
}

output "kms_key_arn" {
  description = "ARN of the KMS key used for Kubernetes Secrets envelope encryption, or null if enable_secrets_encryption is false."
  value       = var.enable_secrets_encryption ? aws_kms_key.eks_secrets[0].arn : null
}
