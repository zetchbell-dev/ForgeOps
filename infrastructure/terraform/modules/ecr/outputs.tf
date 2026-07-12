output "repository_urls" {
  description = "Map of service name -> full ECR repository URL (registry/repo), what CI's `docker push` and the manifest repo's `image.repository` value target. Matches the module boundary output named in M1 §2."
  value       = { for name, repo in aws_ecr_repository.this : name => repo.repository_url }
}

output "repository_arns" {
  description = "Map of service name -> repository ARN, needed when scoping the CI OIDC role's ECR push permission to specific repositories (M3 §4: \"scoped to push-only on the Auth Service ECR repo\")."
  value       = { for name, repo in aws_ecr_repository.this : name => repo.arn }
}

output "repository_registry_id" {
  description = "AWS account ID that owns the registry (same for every repository in this module — ECR registries are one per account per region)."
  value       = length(aws_ecr_repository.this) > 0 ? values(aws_ecr_repository.this)[0].registry_id : null
}

output "kms_key_arn" {
  description = "ARN of the KMS key used for repository encryption, or null if encryption_type is AES256."
  value       = var.encryption_type == "KMS" ? aws_kms_key.ecr[0].arn : null
}
