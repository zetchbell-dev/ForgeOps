variable "aws_region" {
  description = "AWS region the state bucket and lock table are created in. Must match the `region` value every environments/<env>/backend.hcl points at, since a backend can only read state from the region its bucket actually lives in."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used in resource naming and tagging."
  type        = string
  default     = "enterprise-cicd-platform"
}

variable "state_bucket_name" {
  description = "Globally-unique S3 bucket name for Terraform remote state. No default — S3 bucket names collide across all AWS accounts globally, so this must be chosen deliberately per deployment, not derived automatically from project_name alone."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$", var.state_bucket_name))
    error_message = "state_bucket_name must be a valid, lowercase S3 bucket name (3-63 chars, lowercase letters/digits/dots/hyphens)."
  }
}

variable "lock_table_name" {
  description = "DynamoDB table name for Terraform state locking."
  type        = string
  default     = "enterprise-cicd-platform-terraform-locks"
}

variable "ci_role_arn" {
  description = "ARN of the IAM role/OIDC-federated identity CI uses to run terraform apply. Granted write access to the state bucket and lock table; every other principal is restricted to read-only, per M1 §3 (\"no local terraform apply from a laptop against shared environments\"). Optional — if unset (empty string), the bucket policy's write restriction is skipped and access relies entirely on IAM identity policies attached elsewhere, since a placeholder ARN would be worse than no restriction (a policy that can never match is silently useless, not silently safe)."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags to merge into every resource, on top of the mandatory ManagedBy/Project tags applied by this module."
  type        = map(string)
  default     = {}
}
