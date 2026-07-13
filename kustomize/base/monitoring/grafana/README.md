# kustomize/base/monitoring/grafana — M5 Phase 3

Grafana, wired to Phase 2's Prometheus as its primary datasource, with
Loki/Tempo left as commented-out placeholders for Phase 4/5.

## Prerequisites
- Phase 2 (`kustomize/base/monitoring`'s Prometheus/Alertmanager) applied
  first — the Prometheus datasource points at
  `prometheus-operated.monitoring.svc:9090`, which only exists once that
  phase's `Prometheus` CR has been reconciled by the Operator.
- `grafana-admin` Secret's `admin-user`/`admin-password` provisioned
  out-of-band before applying (same pattern as Phase 2's Slack webhook).

## Apply
Applied automatically as part of the parent kustomization:
```
kubectl apply -k kustomize/base/monitoring
```
Or standalone, for local iteration on just this layer:
```
kubectl apply -k kustomize/base/monitoring/grafana -n monitoring
```

## Folder structure
Four provisioned dashboard folders, each backed by its own ConfigMap:

| Folder | ConfigMap | Depends on |
|---|---|---|
| Auth Service | `grafana-dashboards-auth-service` | Phase 2 recording rules — works today |
| Kubernetes | `grafana-dashboards-kubernetes` | node-exporter + kube-state-metrics — **not yet deployed**, panels show "No data" until they are |
| Platform | `grafana-dashboards-platform` | Phase 2 self-monitoring metrics — works today |
| Alerts | `grafana-dashboards-alerts` | Prometheus's `ALERTS` series — works today |

## Why replicas: 1
`configmap-grafana-ini.yaml` configures the default sqlite3 backend.
Sqlite has no safe multi-writer story on a shared ReadWriteOnce volume —
scaling this Deployment beyond 1 replica risks db-lock corruption, not
just stale caches. Migrate `[database]` to Postgres/MySQL in an overlay
before ever raising `replicas`.

## Datasource placeholders (Phase 4 / Phase 5)
`configmap-datasources.yaml` ships Loki and Tempo/Jaeger stanzas
commented out rather than active — an active-but-unreachable datasource
would show a persistent red health-check in Grafana's UI for services
that don't exist yet. When Phase 4/5 land:
1. Confirm the actual Service name/port they expose.
2. Uncomment the matching block in `configmap-datasources.yaml`.
3. No other Grafana file needs to change — dashboards, provisioning,
   and the Deployment are already structured to not require rework.

See `VALIDATION.md` for what was statically checked and what still
needs a real cluster.
