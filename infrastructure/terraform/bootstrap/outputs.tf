output "state_bucket_name" {
  description = "Name of the S3 bucket holding Terraform state for every environment. Used as the `bucket` value in each environments/<env>/backend.hcl."
  value       = aws_s3_bucket.state.id
}

output "state_bucket_arn" {
  description = "ARN of the state bucket."
  value       = aws_s3_bucket.state.arn
}

output "lock_table_name" {
  description = "Name of the DynamoDB lock table. Used as the `dynamodb_table` value in each environments/<env>/backend.hcl."
  value       = aws_dynamodb_table.lock.name
}

output "kms_key_arn" {
  description = "ARN of the KMS key encrypting the state bucket."
  value       = aws_kms_key.state.arn
}
