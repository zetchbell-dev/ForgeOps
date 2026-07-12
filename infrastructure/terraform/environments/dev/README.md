# Environment: `dev`

Root module wiring `modules/vpc`, `modules/eks`, `modules/iam`, and
`modules/ecr` together with dev-sized inputs. Implements
`docs/architecture/M1-terraform-foundation.md` end to end for the `dev`
environment.

## Prerequisites

1. **State backend must already exist.** Run
   `infrastructure/terraform/bootstrap` once (manually, local state) before
   this root module can `init`. See `bootstrap/README.md`.
2. **`backend.hcl` filled in** with the real bucket/table names from that
   bootstrap's outputs (this file's checked-in copy has placeholder values).
3. **`endpoint_public_access_cidrs` in `terraform.tfvars` replaced** with
   your actual CI runner range and break-glass admin IP(s) — the checked-in
   values are examples and `modules/eks` will reject `0.0.0.0/0`, but it
   cannot detect "looks real but isn't yours," so this is a manual review
   step before the first apply, not something Terraform enforces for you.

## Usage

```bash
cd infrastructure/terraform/environments/dev
terraform init -backend-config=backend.hcl
terraform plan   # reviewed as a PR comment in CI (M1 §8) — never applied blind
terraform apply  # via CI only; S3 bucket policy denies write from any other principal (bootstrap module)
```

## Why ECR lives here and only here

`modules/ecr` is instantiated in this root module, not repeated in
`environments/staging` or `environments/prod`. An ECR repository is an
account/region-scoped registry resource — M4 §4's promotion flow depends on
dev, staging, and prod all pulling the *same* image SHA from the *same*
repository. Instantiating `module.ecr` again in staging/prod would either
collide on the repository name (ECR names are unique per account+region,
not per Terraform state) or produce two states both claiming to own the
same AWS resource. `environments/staging` and `environments/prod` instead
read `repository_urls`/`repository_arns` via a `terraform_remote_state`
data source pointed at this environment's state file — see the comment
above `module "ecr"` in `main.tf`.

This is a real, deliberate asymmetry between "dev" and the other two
environments' root modules, not an oversight — flagged here so it isn't
mistaken for one when `environments/staging` is written next and turns out
*not* to mirror this file 1:1.

## What varies vs. staging/prod

Per M1 §4: sizing and scoping only (`node_instance_types`, `az_count`,
`single_nat_gateway`, `endpoint_public_access_cidrs`). Module logic itself
does not branch on environment name — if a future change requires
environment-specific *behavior* rather than *sizing*, that belongs in the
module (as a new input), not as a conditional in a root module.

## Testing

Per M1 §8: `terraform fmt -check`, `terraform validate`, `tflint`,
`checkov -d .`, `terraform plan` posted as a PR comment, and the post-apply
smoke test (`aws eks describe-cluster` → `ACTIVE`, `kubectl get nodes` →
expected count, ECR repo reachable with scan-on-push enabled).

## Known limitation

Same as every module it wires together: written but not yet run through
`terraform validate`/`plan` against real AWS credentials — no network
access in this environment. The four modules' individual `Known
limitation` sections apply transitively here; this root module additionally
has never been through `terraform init` against a real backend, since that
requires the bootstrap step (also unvalidated, same reason) to have run
first.
