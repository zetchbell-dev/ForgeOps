output "cluster_autoscaler_role_arn" {
  description = "ARN of the IRSA role for cluster-autoscaler. Annotate the cluster-autoscaler ServiceAccount with eks.amazonaws.com/role-arn set to this value (done in the cluster-autoscaler Helm chart values, not in this module — this module only produces the AWS-side role)."
  value       = aws_iam_role.cluster_autoscaler.arn
}

output "cluster_autoscaler_role_name" {
  description = "Name of the IRSA role for cluster-autoscaler."
  value       = aws_iam_role.cluster_autoscaler.name
}
