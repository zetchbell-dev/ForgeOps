# Terraform Bootstrap: Remote State Backend

Provisions the S3 bucket (versioned, SSE-KMS encrypted, public access
blocked, write-restricted to the CI role) and DynamoDB lock table that
every `environments/<env>` configuration uses as its Terraform backend.

Implements `docs/architecture/M1-terraform-foundation.md` §3.

## Why this isn't just another module under `modules/`

Every other module in this repository is applied *through* the S3+DynamoDB
backend, by CI. This one provisions that backend, so it can't depend on
it — a chicken-and-egg problem inherent to any "state lives in the cloud"
setup. It is applied **once, manually, with local state**, by whoever is
bootstrapping a new AWS account/region for this project. It is not part of
the `vpc → iam → eks → ecr` dependency chain and is not wired into
`environments/<env>/main.tf`.

## Usage

```bash
cd infrastructure/terraform/bootstrap
terraform init
terraform apply \
  -var="state_bucket_name=enterprise-cicd-platform-tfstate-<unique-suffix>" \
  -var="ci_role_arn=arn:aws:iam::<account-id>:role/github-actions-terraform"
```

Record the outputs (`state_bucket_name`, `lock_table_name`) — they become
the literal values in every `environments/<env>/backend.hcl`.

`ci_role_arn` is optional: if you haven't created the CI OIDC role yet
(that's a separate, later piece of IAM work — not part of the M1 module
set), leave it unset and re-apply once it exists. Passing an empty string
is deliberately treated as "skip the write-restriction statement" rather
than silently generating a policy condition that can never match — a
policy statement referencing a placeholder ARN would look like a real
control while doing nothing.

## Where this module's own state lives

Local (`terraform.tfstate` in this directory, `.gitignore`d). Copy it
somewhere durable outside of Git after applying — it contains resource IDs,
not secrets, but losing it means losing track of the state-backend
resources' Terraform-managed identity (the AWS resources themselves would
still exist; only Terraform's bookkeeping would be lost, recoverable via
`terraform import` if needed).

## Testing

`terraform fmt -check`, `terraform validate`, `tflint`, `checkov -d .` —
same as every other module. No CI pipeline runs this one (it predates CI's
own ability to authenticate to AWS), so these checks are run manually
before the one-time apply.

## Known limitation

Written but not yet run through `terraform validate` — no Terraform binary
or network access in this environment. Treat as draft pending manual
validation before the actual bootstrap apply.
