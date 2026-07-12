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

variable "repository_names" {
  description = "List of service names to create an ECR repository for, one per service (M1 §2: modules/ecr responsibility is \"ECR repo per service\"). Start with just [\"auth-service\"] per the reference-service-first ordering (ADR-6) — repositories for User Service, Gateway, etc. are added to this list in M6/M9, not pre-created now as unused surface area."
  type        = list(string)

  validation {
    condition     = length(var.repository_names) > 0
    error_message = "repository_names must contain at least one repository name."
  }

  validation {
    condition     = length(var.repository_names) == length(distinct(var.repository_names))
    error_message = "repository_names must not contain duplicates."
  }
}

variable "image_tag_mutability" {
  description = "Whether image tags can be overwritten after push. IMMUTABLE by default — once CI pushes a tag (M3 §2: tag = git SHA), that tag must never resolve to a different image, or a manifest-repo entry pointing at a SHA could silently start deploying different bytes than what passed CI's scans."
  type        = string
  default     = "IMMUTABLE"

  validation {
    condition     = contains(["MUTABLE", "IMMUTABLE"], var.image_tag_mutability)
    error_message = "image_tag_mutability must be either MUTABLE or IMMUTABLE."
  }
}

variable "scan_on_push" {
  description = "Enable ECR basic scan-on-push. Baseline scanning at the registry level, complementary to (not redundant with) the Trivy/Snyk pipeline scanning in M3 — this catches known CVEs in the image at rest; pipeline scanning gates the PR before merge (M1 §6)."
  type        = bool
  default     = true
}

variable "encryption_type" {
  description = "ECR repository encryption at rest. KMS (customer-managed) by default rather than the AES256 default, consistent with the KMS-for-Secrets choice in modules/eks — container images (including Auth Service's) are treated as sensitive artifacts, not just build output."
  type        = string
  default     = "KMS"

  validation {
    condition     = contains(["AES256", "KMS"], var.encryption_type)
    error_message = "encryption_type must be either AES256 or KMS."
  }
}

variable "untagged_image_expiry_days" {
  description = "Days after which untagged images are expired by the lifecycle policy. Untagged images accumulate from every intermediate build layer and superseded mutable-tag pushes; they have no manifest-repo reference pointing at them, so nothing depends on keeping them past a short window."
  type        = number
  default     = 14

  validation {
    condition     = var.untagged_image_expiry_days >= 1
    error_message = "untagged_image_expiry_days must be at least 1."
  }
}

variable "tagged_image_count_to_keep" {
  description = "Number of most-recent tagged images to retain per repository; older tagged images are expired. Tagged images are git-SHA-tagged builds (M3 §2) — a generous count is kept because a rollback (M4 §7) may need to reference an older SHA than the immediately preceding one, and because ArgoCD's own history/rollback UI is only useful as far back as an image the registry still has."
  type        = number
  default     = 100

  validation {
    condition     = var.tagged_image_count_to_keep >= 1
    error_message = "tagged_image_count_to_keep must be at least 1."
  }
}

variable "tags" {
  description = "Additional tags to merge into every resource, on top of the mandatory Environment/ManagedBy/Project tags applied by this module."
  type        = map(string)
  default     = {}
}
