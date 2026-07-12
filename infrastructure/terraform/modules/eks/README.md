# Terraform Module: `eks`

Provisions the EKS control plane, its IAM OIDC provider (the foundation for
IRSA), and one managed node group, plus the three baseline add-ons
(`vpc-cni`, `coredns`, `kube-proxy`) every cluster needs regardless of workload.

Implements the design in `docs/architecture/M1-terraform-foundation.md` §2, §6.

## What this module deliberately does NOT do

- **Does not create IRSA roles for workloads.** That's `modules/iam`, which
  takes this module's `oidc_provider_arn`/`oidc_provider_url` outputs as
  inputs. The two modules stay decoupled at authoring time; Terraform's
  dependency graph handles apply ordering once `environments/<env>/main.tf`
  wires them together.
- **Does not create the VPC or subnets.** Takes `vpc_id` and
  `private_subnet_ids` from `module.vpc`.
- **Does not install workload-specific add-ons** (EBS CSI driver, ALB
  ingress controller, external-dns, etc.). Only the three add-ons every
  cluster needs are included; anything workload-specific is added in the
  milestone that first needs it — same "don't provision what nothing needs
  yet" principle M1 §1 applies to RDS/Vault.
- **Does not put nodes, or the control plane's own ENIs, in a public
  subnet.** `subnet_ids` for both the cluster and the node group is
  `var.private_subnet_ids` exclusively (M1 §6). This is independent of
  `endpoint_public_access`, which only governs whether the API server's
  endpoint is reachable from outside the VPC.
- **Does not default the public endpoint open to the internet.**
  `endpoint_public_access_cidrs` has a validation rule rejecting
  `0.0.0.0/0` — callers must supply the actual CI runner range and
  break-glass admin IPs (M1 §6).

## Usage

```hcl
module "eks" {
  source = "../../modules/eks"

  environment = "dev"

  cluster_name    = "enterprise-cicd-platform-dev"
  cluster_version = "1.29"

  vpc_id              = module.vpc.vpc_id
  private_subnet_ids  = module.vpc.private_subnet_ids

  endpoint_public_access        = true
  endpoint_public_access_cidrs  = ["203.0.113.0/24"] # CI runner range + break-glass admin IPs — never 0.0.0.0/0

  node_instance_types = ["m6i.large"]
  node_desired_size   = 2
  node_min_size       = 1
  node_max_size       = 4
}

module "iam" {
  source = "../../modules/iam"

  environment       = "dev"
  cluster_name      = module.eks.cluster_name
  oidc_provider_arn = module.eks.oidc_provider_arn
  oidc_provider_url = module.eks.oidc_provider_url
}
```

Production usage differs only in tfvars (larger `node_instance_types`,
`node_max_size`, and typically 3 AZs from `module.vpc`), not module code —
consistent with M1 §4's "sizing is the only thing that varies between
environments."

## Design notes

- **Managed node group, not a hand-rolled ASG + launch template.** AWS
  handles AMI selection and patch rollout mechanics; Terraform manages the
  scaling band. `scaling_config[0].desired_size` is excluded from plan
  diffs (`lifecycle.ignore_changes`) because the cluster-autoscaler
  (`modules/iam`) adjusts it at runtime — Terraform still owns `min_size`
  and `max_size`, the band the autoscaler can't exceed.
- **ASG autoscaler tags applied via the node group's `tags` argument**,
  which EKS propagates to the backing ASG — this is what makes
  `modules/iam`'s `autoscaling:ResourceTag/k8s.io/cluster-autoscaler/<cluster_name>
  = owned` condition actually match. If this module's tagging changes, the
  IAM module's policy condition and this module's node group tags must be
  kept in sync (both reference `var.cluster_name`, so a caller only ever
  sets this once and both modules pick it up).
- **KMS envelope encryption for Secrets is on by default**
  (`enable_secrets_encryption`), layered on top of EKS's default etcd
  encryption — an explicit AWS Well-Architected / OWASP-aligned choice
  given Auth Service (M2) will run credential-adjacent secrets on this
  cluster.
- **All five control-plane log types shipped to CloudWatch by default** —
  `audit` and `authenticator` in particular are what an incident
  investigation needs to answer "who did what to the API server," which
  Prometheus/Grafana (M5) doesn't cover.

## Inputs / Outputs

See `variables.tf` and `outputs.tf` — documented inline with design
rationale. Notable: `oidc_provider_url` is not in M1's original
module-boundary table but is added here because `modules/iam` requires it
(IAM trust-policy condition keys are string-matched against the issuer URL
without its `https://` prefix) — a documented, deliberate extension of the
table, not a deviation from it.

## Testing

Per M1 §8 test plan:

- `terraform fmt -check`
- `terraform validate`
- `tflint`
- `checkov -d .` — expect this to flag the `cluster_security_group_id`
  output and the AWS-managed policy attachments; both are reviewed,
  accepted exceptions (control-plane service permissions are the standard
  AWS-managed policy, not a candidate for hand-rolled least-privilege).
- `terraform plan` reviewed as a PR comment before any apply.
- Post-apply smoke test (M1 §8): `aws eks describe-cluster` returns
  `ACTIVE`; `kubectl get nodes` shows the expected node count, which
  depends on `node_group_status` (see `outputs.tf`) reaching `ACTIVE`
  first — the node group typically takes several minutes longer than the
  control plane to become ready, so a smoke test polling immediately after
  `terraform apply` returns should retry rather than fail on the first
  check.

These run in CI (M3-style pipeline for infrastructure changes) — not yet
implemented as a workflow file; tracked as the next infra-CI task after all
four M1 modules (`vpc`, `iam`, `eks`, `ecr`) exist.

## Known limitation

Same as `modules/vpc` and `modules/iam`: written but **not yet run through
`terraform validate`** against a real Terraform binary/provider — that step
requires network access this environment doesn't have. It must be validated
in CI (or locally) before merge; treat it as a draft pending that check per
the M1 test plan, not as verified working code yet.
