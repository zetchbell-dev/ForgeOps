# Terraform Module: `vpc`

Provisions the network foundation for one environment: VPC, public/private
subnets across N AZs, Internet Gateway, NAT Gateway(s), and route tables.

Implements the design in `docs/architecture/M1-terraform-foundation.md`.

## What this module deliberately does NOT do

- No EKS cluster (see `modules/eks`, depends on this module's outputs)
- No IAM roles (see `modules/iam`)
- No route53 or NAT-per-AZ enforcement beyond the `single_nat_gateway` flag —
  prod's `environments/prod/main.tf` is responsible for setting
  `single_nat_gateway = false`; this module does not hard-code environment-name
  branching per M1 §4 ("module logic itself does not branch on environment name").

## Usage

```hcl
module "vpc" {
  source = "../../modules/vpc"

  environment        = "dev"
  cidr_block         = "10.0.0.0/16"
  az_count           = 2
  single_nat_gateway = true # accepted SPOF for dev per M1 cost tradeoff

  tags = {
    Team = "platform"
  }
}
```

Production usage differs only in tfvars, not module code:

```hcl
module "vpc" {
  source = "../../modules/vpc"

  environment        = "prod"
  cidr_block         = "10.1.0.0/16"
  az_count           = 3
  single_nat_gateway = false # HA: one NAT per AZ
}
```

## Inputs

See `variables.tf` — all inputs are documented inline with their design rationale.

## Outputs

See `outputs.tf`. Notable: `private_subnet_ids` is what `modules/eks` consumes
for node group placement — nodes never go in `public_subnet_ids` (M1 §6).

## Testing

Per M1 §8 test plan, this module is validated via:
- `terraform fmt -check`
- `terraform validate`
- `tflint`
- `checkov -d .` (security policy scan)
- `terraform plan` reviewed as a PR comment before any apply

These run in CI (M3-style pipeline for infrastructure changes) — not yet
implemented as a workflow file; tracked as the next infra-CI task after all
four M1 modules (`vpc`, `iam`, `eks`, `ecr`) exist.

## Known limitation

This module has been written but **not yet run through `terraform validate`**
against a real Terraform binary/provider — that step requires network access
this environment doesn't have. It must be validated in CI (or locally) before
merge; treat it as a draft pending that check per the M1 test plan, not as
verified working code yet.
