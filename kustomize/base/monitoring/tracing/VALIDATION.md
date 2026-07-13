# M5 Phase 5 — Static Validation Report

Performed without cluster/`kustomize`/`kubeval` access (none available in this
environment). All checks below are static: YAML parsing + cross-referencing
field values across files by script. **Runtime validation (listed at the
bottom) still has to happen against a real cluster before this is
considered done.**

## 1. YAML syntax
All 46 manifests across the full `kustomize/base/monitoring` tree (Phases
2–5 combined) parse cleanly with `yaml.safe_load_all`. No syntax errors.

## 2. Kustomization resolution
`kustomize/base/monitoring/kustomization.yaml` now resolves — recursively
through `grafana`, `logging`, and `tracing` — to 49 objects total, with
zero dangling resource references and zero duplicate `(kind, name)` pairs.
Phase 5 contributes 9 of those: `ServiceAccount` x2, `ClusterRole` x1,
`ClusterRoleBinding` x1, `ConfigMap` x2, `Deployment` x2, `Service` x2,
`PersistentVolumeClaim` x1 (13 objects across the 9 manifest files —
`serviceaccount.yaml` and `rbac.yaml` each define 2 objects).

## 3. Cross-resource reference checks (all PASS)
- `otel-collector-deployment.yaml` `serviceAccountName: otel-collector` → `serviceaccount.yaml` defines it.
- `tempo-deployment.yaml` `serviceAccountName: tempo` → `serviceaccount.yaml` defines it.
- `rbac.yaml` ClusterRoleBinding subject (`otel-collector`, ns `monitoring`) → matches the SA + the namespace this kustomization applies.
- `otel-collector-deployment.yaml`'s config volume → `otel-collector-configmap.yaml`'s `otel-collector-config` ConfigMap name matches exactly.
- `tempo-deployment.yaml`'s config volume → `tempo-configmap.yaml`'s `tempo-config` ConfigMap name matches exactly.
- `tempo-deployment.yaml`'s data volume → `tempo-pvc.yaml`'s `tempo-data` PVC name matches exactly.
- Every named container port (`otlp-grpc`, `otlp-http`, `http`) on both Deployments has a matching `targetPort` name on the corresponding Service — `otel-collector-service.yaml` and `tempo-service.yaml`.
- `otel-collector-configmap.yaml`'s `otlp/tempo` exporter endpoint (`tempo.monitoring.svc:4317`) → matches `tempo-service.yaml`'s `otlp-grpc` port exactly.
- `otel-collector-configmap.yaml`'s `health_check` extension (port 13133) → matches `otel-collector-deployment.yaml`'s readiness/liveness probe port exactly.
- `grafana/configmap-datasources.yaml`'s newly-enabled Tempo datasource URL (`http://tempo.monitoring.svc:3200`) → matches `tempo-service.yaml`'s `http` port exactly.
- `grafana/configmap-datasources.yaml`'s Tempo datasource `tracesToLogsV2.datasourceUid: loki` → matches the `loki` datasource's own `uid` field, defined earlier in the same ConfigMap (Phase 4).
- Parent `kustomize/base/monitoring/kustomization.yaml` now lists `tracing` alongside `grafana` and `logging`, in the same pattern.

## 4. RBAC
`tracing/rbac.yaml`'s ClusterRole grants only `get/list/watch` on
`pods`, `namespaces`, and `replicasets` — the minimum the `k8sattributes`
processor needs to resolve a source pod IP to its owning
namespace/deployment. No write verbs, no other resource types. Distinct
ClusterRole/Binding names (`forgeops-otel-collector`) from Prometheus's
own `rbac.yaml` (`prometheus`) one directory up — no name collision,
confirmed in the duplicate-check in §2.

## 5. Tempo configuration
Single-binary mode, `local` storage backend pointed at the
`tempo-data` PVC, OTLP receiver on 4317 (gRPC)/4318 (HTTP), HTTP query
API on 3200. `compactor.compaction.block_retention: 360h` intentionally
matches `prometheus.yaml`'s `retention: 15d` and
`logging/loki-configmap.yaml`'s `retention_period: 360h` — one retention
window across metrics/logs/traces.

## 6. OpenTelemetry Collector configuration
`memory_limiter` → `k8sattributes` → `batch` pipeline, single
`otlp/tempo` exporter. Confirmed the Collector does **not** stack a
`probabilistic_sampler`/`tail_sampling` processor on top of each
service's own SDK-side head sampling (Phase 1's `tracing.go`) — the
commented `tail_sampling` block is documented as a future upgrade, not
silently enabled. `health_check` extension backs the Deployment's
probes (added during this pass — the deployment referenced a health
port that had no matching Collector extension until now; see §3 fourth
bullet).

## 7. Grafana datasource configuration
Tempo datasource enabled exactly as the Phase 3/4 placeholder specified
(same URL, same `tracesToLogsV2` correlation to the `loki` datasource
UID) — no structural changes beyond uncommenting, per the task
constraint to avoid touching Phase 3/4 files beyond a genuine
integration update.

## 8. Known gaps — NOT fixed in this pass, flagged rather than silently ignored
- **Single Collector replica**: the commented `tail_sampling` processor in `otel-collector-configmap.yaml` requires a trace-ID-aware load-balancing exporter/receiver pair to shard correctly across replicas — not implemented. Do not scale past 1 replica until that's added, or tail-sampling decisions will be made on incomplete traces.
- **Single Tempo replica**: local-disk single-writer storage, same constraint as Loki/Grafana one and two directories up. Scale via Tempo's microservices mode + object storage, not by raising `replicas`.
- **Local-disk trace storage**: bounded by `tempo-pvc.yaml`'s 20Gi and the compactor's 360h retention — not a real object-store lifecycle policy. Swap for S3/GCS/Azure blob storage in an overlay before production trace volume.
- **`k8sattributes` processor's `auth_type: serviceAccount`** assumes the Collector pod's own mounted token is used for the K8s API lookup (standard for in-cluster processors) — not yet confirmed against the actual Operator/K8s API version installed (same category of "unconfirmed against install-time specifics" flag Phase 2's `servicemonitor-platform-self.yaml` already carries).

## 9. Runtime validation still required (cannot be done statically)
- `kustomize build kustomize/base/monitoring | kubectl apply --dry-run=server -f -` once a cluster is available.
- Confirm the `otel/opentelemetry-collector-contrib:0.114.0` and `grafana/tempo:2.6.1` image tags are still current/patched before deploying to production.
- Set `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.monitoring.svc:4318` on auth-service (or any other instrumented service) and confirm a real trace round-trips through the Collector into Tempo and is queryable from Grafana's Explore view.
- Confirm the `k8sattributes` processor actually resolves pod metadata correctly once real traffic flows — its enrichment behavior can only be verified against a live API server, not statically.
