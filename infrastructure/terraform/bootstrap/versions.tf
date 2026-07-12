terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Deliberately no backend block: this configuration provisions the S3
  # bucket + DynamoDB table that every other environment's backend depends
  # on, so it cannot depend on them itself. State for this module stays
  # local, per the design note in main.tf.
}
