# Terraform Module: `iam`

Provisions IRSA (IAM Roles for Service Accounts) roles for pods running inside
the EKS cluster — starting with the one role M1/M2 implementation actually
needs: **cluster-autoscaler**.

Implements the design in `docs/architecture/M1-terraform-foundation.md` §2, §6.

## What this module deliberately does NOT do

- **Does not create the EKS OIDC provider.** That's created by `modules/eks`
  (it's a property of the cluster). This module takes `oidc_provider_arn` and
  `oidc_provider_url` as inputs instead, so the two modules stay decoupled at
  authoring time — Terraform's dependency graph handles apply ordering once
  `environments/<env>/main.tf` wires `module.eks` outputs into this module.
- **Does not create an external-dns role, or any other "we'll need this
  eventually" role.** A role for a service account that doesn't exist yet is
  unused surface area and untestable — it gets added in the milestone that
  actually introduces that workload, with its own scoped trust policy at that
  time.
- **Does not grant broad `autoscaling:*` or wildcard-resource mutate
  permissions.** The mutate-permission statement is scoped via a resource tag
  condition to only ASGs tagged for this specific cluster.

## Usage

```hcl
module "iam" {
  source = "../../modules/iam"

  environment        = "dev"
  cluster_name       = module.eks.cluster_name
  oidc_provider_arn  = module.eks.oidc_provider_arn
  oidc_provider_url  = module.eks.oidc_provider_url
}
```

Note the dependency: this module's inputs come from `module.eks`'s outputs, so
in `environments/<env>/main.tf`, `module.eks` must be declared (Terraform infers
the correct apply order from these references automatically — no explicit
`depends_on` needed at the root level beyond the input references themselves).

## Inputs / Outputs

See `variables.tf` and `outputs.tf` — documented inline with design rationale.
`cluster_autoscaler_role_arn` is what gets annotated onto the Kubernetes
ServiceAccount (via the cluster-autoscaler Helm chart's
`serviceAccount.annotations`) — that annotation step happens at the Helm/K8s
layer, not in this Terraform module.

## Testing

Per M1 §8: `terraform fmt -check`, `terraform validate`, `tflint`, `checkov -d .`
— with particular attention for this module to policy-document scope (checkov
will flag overly broad `resources = ["*"]` statements; the describe-only
statement above is intentionally `"*"` since AWS's cluster-autoscaler describe
actions don't support resource-level scoping, and this is documented as an
accepted, reviewed exception rather than an oversight).

## Known limitation

Same as `modules/vpc`: written but **not yet run through `terraform validate`**
or `checkov` — no Terraform binary/network access in this environment. Treat as
draft pending CI validation, not verified working code.
