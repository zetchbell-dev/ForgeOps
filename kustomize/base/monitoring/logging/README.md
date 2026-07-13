# kustomize/base/monitoring/logging — M5 Phase 4

Loki (single-binary, filesystem storage) + Fluent Bit (DaemonSet log
collector), wired as Grafana's `Loki` datasource (uncommented in
`../grafana/configmap-datasources.yaml` as part of this phase).

## Prerequisites
- Phase 2 applied (shares the `monitoring` namespace).
- Phase 3 (Grafana) applied if you want the Loki datasource usable
  immediately — not a hard dependency for Loki/Fluent Bit to run on
  their own.
- Nodes running a CRI-compliant container runtime (containerd, the EKS
  default) — see `fluentbit-configmap.yaml`'s ASSUMPTION FLAG if that's
  not the case; the `docker` parser is defined as a fallback but not
  wired in by default.

## Apply
```
kubectl apply -k kustomize/base/monitoring
```

## Components
| Component | Kind | Notes |
|---|---|---|
| Loki | Deployment (1 replica) | Filesystem/tsdb storage, 360h retention — matches Prometheus's 15d for metrics/logs correlation parity |
| Fluent Bit | DaemonSet | One pod per node, tails `/var/log/containers`, enriches with `kubernetes` filter, ships to Loki |

## Why Loki is single-binary, single-replica
Same reasoning as Grafana's sqlite constraint (Phase 3): the tsdb index
and filesystem chunk store on a shared ReadWriteOnce volume don't have a
safe multi-writer story. Scale via Loki's distributed mode (separate
read/write/backend + object storage), not by raising `replicas` here.

## Why Fluent Bit runs as root
Every other workload in this repository (Prometheus, Alertmanager,
Grafana, Loki) runs `runAsNonRoot: true`. Fluent Bit is the one
deliberate exception — reading `/var/log/containers` on arbitrary node
images reliably requires root, and that's a node-level file permission
this repository doesn't control. Documented, not accidental.

## Known gaps
- Fluent Bit's `Parser cri` assumes a containerd-style CRI log format.
  Verify against the actual node AMI before relying on this — see the
  ASSUMPTION FLAG comment in `fluentbit-configmap.yaml`.
- No log-based alerting is wired up (Alertmanager, per Phase 2, remains
  the single alerting source of truth — Loki's `ruler.enable_api` is
  explicitly disabled).
- No NetworkPolicy is added for Loki/Fluent Bit traffic — the
  `monitoring` namespace has no default-deny policy today, so intra-namespace
  traffic (Fluent Bit → Loki, Grafana → Loki) already works without one.

See `VALIDATION.md` for what was statically checked and what still
needs a real cluster.
