# Environment: `prod`

Root module wiring `modules/vpc`, `modules/eks`, `modules/iam` together with
prod-sized inputs (3 AZs, one NAT Gateway per AZ, larger node group). This is
the HA baseline `environments/staging` is deliberately set relative to (M4
§3: "staging mirrors prod path for realistic pre-prod validation").
Implements `docs/architecture/M1-terraform-foundation.md` for the `prod`
environment.

## Prerequisites

1. **State backend must already exist.** Run
   `infrastructure/terraform/bootstrap` once (manually, local state) before
   this root module can `init`. See `bootstrap/README.md`.
2. **`backend.hcl` filled in** with the real bucket/table names from that
   bootstrap's outputs (this file's checked-in copy has placeholder values).
3. **`endpoint_public_access_cidrs` in `terraform.tfvars` replaced** with
   your actual CI runner range and break-glass admin IP(s) — the checked-in
   values are examples. Keep this list as tight as possible here: M1 §6
   calls for the EKS API endpoint to be private + restricted public access,
   never fully public, and that matters most in prod.
4. **`environments/dev` must already be applied.** `data.tf` reads dev's
   state for ECR repository info (see below) — a prod `plan` will fail if
   dev's state doesn't exist yet.

## What's different from `environments/dev` and `environments/staging`

| Aspect | dev | staging | prod |
|---|---|---|---|
| AZ count | 2 | 3 | 3 |
| NAT Gateway | single, shared (accepted SPOF) | one per AZ (HA) | one per AZ (HA) |
| Node instance types | `t3.large` | `m6i.large` | `m6i.xlarge` |
| Node min size | 1 | 2 | 3 (one per AZ, so a draining node never drops below quorum capacity) |
| `module.ecr` | instantiated | not instantiated — read via remote state | not instantiated — read via remote state |

Sizing differences only — no module logic branches on environment name
(M1 §4), and this file's structure mirrors `environments/staging`'s except
for the values above.

## Why there's no `module.ecr` here

Same reasoning as `environments/staging`: ECR repositories are
account/region-scoped, shared by every environment — prod must deploy the
exact image SHA already validated in dev and staging (M4 §4's promotion
flow), not build or reference a separate set of repositories. `data.tf`
reads `environments/dev`'s already-applied state
(`repository_urls`/`repository_arns`) via `terraform_remote_state`, rather
than re-declaring `module.ecr` here. See `environments/dev/README.md` →
"Why ECR lives here and only here" for the full reasoning.

## What Terraform does and does NOT gate here

This root module provisions the VPC/EKS/IAM infrastructure prod runs on. It
does not gate deployments to that infrastructure — that's ArgoCD's manual
sync trigger on the prod Application (M4 §3), a separate, later concern from
`docs/architecture/M4-gitops-deployment.md`. A `terraform apply` here
changes the cluster prod runs on; it is not the same approval gate as
promoting a new image to prod.

## Usage

```bash
cd infrastructure/terraform/environments/prod
terraform init -backend-config=backend.hcl
terraform plan   # reviewed as a PR comment in CI (M1 §8) — never applied blind
terraform apply  # via CI only, plus manual approval gate (M1 §5) — this
                  # environment's blast radius is the highest of the three
```

## Test plan

Per M1 §8: `terraform fmt -check`, `terraform validate`, `tflint`,
`checkov -d .`, `terraform plan` posted as a PR comment, and the post-apply
smoke test (`aws eks describe-cluster` → `ACTIVE`, `kubectl get nodes` →
expected count, ECR repo reachable with scan-on-push enabled). Per M1 §8,
no destructive testing (destroy/recreate cycles) is ever exercised against
this environment — `dev` is the only environment where that's done.

## Known limitation

Same as every module it wires together and as `environments/staging`:
written but not yet run through `terraform validate`/`plan` against real AWS
credentials — no network access in this environment. This root module has
additionally never been through `terraform init` against a real backend,
since that requires the bootstrap step (also unvalidated, same reason) and
`environments/dev`'s own apply to have run first.
