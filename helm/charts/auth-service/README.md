# auth-service

A production-ready Helm chart for ForgeOps Auth Service.

## Scope note

This chart is the Helm authoring layer referenced by the M4 GitOps design
doc's Section 2 as living in `helm/charts/auth-service/` in the main repo.
Per the current project roadmap it is built now, in M4 Phase 1; the
Kustomize base/overlays consumed by ArgoCD (M4 Phase 2) are produced from
this chart's rendered output at build time, not by ArgoCD invoking Helm
directly at sync time.

## Prerequisites

- Kubernetes >= 1.27
- Helm >= 3.12
- A reachable Postgres instance and Redis instance (this chart does not
  deploy either — see `config.redisAddr` and `secrets.postgresDSN` /
  `secrets.existingSecret`)

## Installing

```bash
# Local/dev (chart creates its own Secret from example values)
helm install auth-service ./charts/auth-service \
  -f examples/values-development.yaml \
  --namespace auth-dev --create-namespace

# Production (requires a pre-existing Secret; see below)
helm install auth-service ./charts/auth-service \
  -f examples/values-production.yaml \
  --namespace auth-prod --create-namespace
```

## Secrets

The chart never bakes real credentials into a values file meant for
staging/production. Two modes:

- **`secrets.existingSecret` unset** (default, dev-only): the chart
  creates a Secret from `secrets.postgresDSN` / `secrets.jwtSigningKey` /
  `secrets.redisPassword` in values. Fine for a throwaway dev cluster;
  never commit real values this way.
- **`secrets.existingSecret: <name>`** (required for staging/prod): the
  chart consumes a Secret you manage out-of-band (External Secrets
  Operator, Sealed Secrets, SOPS, or manually created), mapping its keys
  via `secrets.existingSecretKeys`.

## Migrations

`migration.enabled: true` (default) runs `/app/migrate -direction=up` as a
`pre-install,pre-upgrade` Helm hook Job, using the same image, config, and
secret env as the main Deployment, before new pods roll out. Disable with
`migration.enabled: false` if migrations are run through a separate
pipeline step instead.

## Key values

| Key | Description | Default |
|---|---|---|
| `image.repository` | Container image repository | `ghcr.io/enterprise-cicd-platform/auth-service` |
| `image.tag` | Image tag; defaults to `Chart.AppVersion` if unset | `""` |
| `replicaCount` | Replica count when `autoscaling.enabled: false` | `2` |
| `autoscaling.enabled` | Enable HPA (CPU + memory) | `true` |
| `podDisruptionBudget.enabled` | Enable PDB | `true` |
| `networkPolicy.enabled` | Enable default-deny-except NetworkPolicy | `true` |
| `ingress.enabled` | Create an Ingress | `false` |
| `secrets.existingSecret` | Name of a pre-existing Secret to consume | `""` |
| `config.redisAddr` | `host:port` for Redis | `auth-service-redis-master:6379` |
| `migration.enabled` | Run the migrate Job as a Helm hook | `true` |

See `values.yaml` (and `values.schema.json` for the enforced shape) for
the full set, and `examples/values-development.yaml` /
`examples/values-production.yaml` for worked configurations.

## Validating

```bash
helm lint ./charts/auth-service
helm template auth-service ./charts/auth-service -f examples/values-development.yaml | kubectl apply --dry-run=client -f -
```

## What this chart does not do

- Does not deploy Postgres or Redis — point `config.redisAddr` /
  `secrets.postgresDSN` at existing instances.
- Does not configure DNS/TLS certificate issuance for `ingress` — bring
  your own `IngressClass`/cert-manager `ClusterIssuer` and reference them
  via `ingress.className` / `ingress.annotations`.
- Does not implement canary/blue-green rollout — that's Argo Rollouts,
  layered on top of this chart's rendered output in the manifest repo
  (M4 Phase 2/3), not something this chart does itself.
