# Environment: `staging`

Root module wiring `modules/vpc`, `modules/eks`, `modules/iam` together with
staging-sized inputs that deliberately mirror prod's topology (3 AZs,
per-AZ NAT) rather than dev's (M4 §3: "staging mirrors prod path for
realistic pre-prod validation"). Implements
`docs/architecture/M1-terraform-foundation.md` for the `staging` environment.

## What's different from `environments/dev`

| Aspect | dev | staging |
|---|---|---|
| AZ count | 2 | 3 (matches prod) |
| NAT Gateway | single, shared (accepted SPOF) | one per AZ (HA, matches prod) |
| `module.ecr` | instantiated | **not instantiated** — read via remote state (see below) |

Sizing differences only — no module logic branches on environment name
(M1 §4), and this file's structure mirrors `environments/dev`'s except for
the ECR piece.

## Why there's no `module.ecr` here

ECR repositories are account/region-scoped, shared by every environment —
staging must deploy the exact image SHA dev already validated (M4 §4), not
build or reference a separate set of repositories. `data.tf` reads
`environments/dev`'s already-applied state (specifically
`repository_urls`/`repository_arns`) via `terraform_remote_state`, rather
than re-declaring `module.ecr` here. See
`environments/dev/README.md` → "Why ECR lives here and only here" for the
full reasoning.

## Prerequisites

Same three as `environments/dev` (bootstrap state backend, filled-in
`backend.hcl`, real `endpoint_public_access_cidrs`), plus:

4. **`environments/dev` must already be applied.** `data.terraform_remote_state.dev`
   reads dev's state file — if dev hasn't been applied yet, staging's `plan`
   will fail resolving `repository_urls`. This is the concrete expression
   of the dependency order implied by M4's promotion flow (dev proves an
   image before staging/prod ever see it).

## Usage

```bash
cd infrastructure/terraform/environments/staging
terraform init -backend-config=backend.hcl
terraform plan
terraform apply  # via CI only
```

## Testing

Same as `environments/dev` — `terraform fmt -check`, `terraform validate`,
`tflint`, `checkov -d .`, PR-commented `plan`, post-apply smoke test.

## Known limitation

Same disclosed limitation as every other file in this repository so far:
written but not yet run through `terraform validate`/`plan` against real
AWS credentials or a real remote state file to read from.
