# Terraform Module: `ecr`

Provisions one ECR repository per service, KMS-encrypted, scan-on-push
enabled, with a lifecycle policy that expires untagged images quickly and
caps retained tagged images at a generous but bounded count.

Implements the design in `docs/architecture/M1-terraform-foundation.md` §2, §6.

## What this module deliberately does NOT do

- **Does not pre-create repositories for services that don't exist yet.**
  `repository_names` starts as `["auth-service"]`; User Service, Gateway,
  and the rest are added to the list in M6/M9 when those services actually
  land — not provisioned now as unused surface area (ADR-6).
- **Does not grant any IAM push/pull permissions.** That's the CI OIDC
  role (M3 §4) and the node group's `AmazonEC2ContainerRegistryReadOnly`
  policy (`modules/eks`) — this module only owns the repositories
  themselves and their outputs.
- **Does not allow a one-step destroy.** Every repository carries
  `lifecycle { prevent_destroy = true }` per M1 §5's blast-radius table.

## Usage

```hcl
module "ecr" {
  source = "../../modules/ecr"

  environment       = "dev"
  repository_names  = ["auth-service"]

  image_tag_mutability       = "IMMUTABLE"
  untagged_image_expiry_days = 14
  tagged_image_count_to_keep = 100
}
```

`repository_urls["auth-service"]` is what CI's `docker push` step (M3 §3)
and the manifest repo's Kustomize image transformer (M4 §2) both target.

## Design notes

- **`IMMUTABLE` tags by default.** CI tags images with the git SHA (M3
  §2); once a manifest-repo overlay points at `sha-abc123`, that tag must
  never resolve to different bytes later, or the audit trail M4 §4
  depends on ("promotion bumps to a SHA already proven in the prior
  environment") stops meaning anything.
- **`KMS` encryption by default**, not the ECR default `AES256` — treats
  container images as sensitive artifacts, consistent with the
  `modules/eks` choice to KMS-encrypt Kubernetes Secrets.
- **Lifecycle policy has two rules, evaluated in priority order:** untagged
  images (no manifest-repo reference points at them) expire after
  `untagged_image_expiry_days`; tagged images are capped at
  `tagged_image_count_to_keep` most-recent, sized generously (default 100)
  because a manual rollback (M4 §7) may need to reference a SHA older than
  the immediately preceding one.
- **`prevent_destroy = true`** is the Terraform-level enforcement of M1
  §5's mitigation for accidental repo deletion. Removing a service from
  `repository_names`, or destroying this module, requires the two-step
  manual process from the runbook (lift the guard deliberately, confirm,
  then remove) — not a side effect of an unrelated `terraform apply`.

## Inputs / Outputs

See `variables.tf` and `outputs.tf` — documented inline with design
rationale. `repository_urls` and `repository_arns` are both keyed by
service name (matching `repository_names` entries), not by repository ARN
or ID, so callers (CI workflow, IAM policy scoping) can reference
`module.ecr.repository_urls["auth-service"]` directly.

## Testing

Per M1 §8 test plan:

- `terraform fmt -check`
- `terraform validate`
- `tflint`
- `checkov -d .` — expect a finding on `prevent_destroy` being relied on as
  a safety control rather than a resource-policy-level restriction; this
  is a reviewed, accepted approach (Terraform-level guard plus documented
  runbook process, not a compliance gap).
- `terraform plan` reviewed as a PR comment before any apply.
- Post-apply smoke test (M1 §8): confirm the repository is reachable
  (`aws ecr describe-repositories`) and `imageScanningConfiguration.scanOnPush`
  is `true`.

These run in CI (M3-style pipeline for infrastructure changes) — not yet
implemented as a workflow file; tracked as the next infra-CI task now that
all four M1 modules (`vpc`, `iam`, `eks`, `ecr`) exist.

## Known limitation

Same as the other three M1 modules: written but **not yet run through
`terraform validate`** against a real Terraform binary/provider — no
network access in this environment. Treat as a draft pending CI validation,
not verified working code yet.
