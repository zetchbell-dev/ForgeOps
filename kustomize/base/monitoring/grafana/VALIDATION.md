# M5 Phase 3 — Static Validation Report

Same method as Phase 2's `VALIDATION.md`: no cluster/`kustomize`/`kubeval`
available here, so everything below is static (YAML + embedded-JSON
parsing, cross-file reference checks via a hand-written script).

## 1. YAML syntax
All 13 files in this directory (including the 4 dashboard ConfigMaps,
each embedding a JSON dashboard model as a literal block scalar) parse
cleanly. Each embedded JSON payload was separately re-parsed with
`json.loads` to confirm the dashboard models themselves are valid, not
just the YAML wrapper.

## 2. Required kinds present
1x ServiceAccount, 1x Secret, 1x PersistentVolumeClaim, 7x ConfigMap
(config, datasources, dashboard-provisioning, 4x dashboards), 1x
Deployment, 1x Service — all present.

## 3. Cross-resource reference checks (all PASS)
- Deployment's `serviceAccountName: grafana` → ServiceAccount exists.
- Every `volumes[].configMap.name` / `.persistentVolumeClaim.claimName`
  in the Deployment → resolves to a ConfigMap/PVC actually defined in
  this directory. No dangling volume reference.
- Both `GF_SECURITY_ADMIN_*` env vars' `secretKeyRef.name` → resolves to
  the `grafana-admin` Secret.
- Every `volumeMounts[].name` in the container → resolves to a declared
  `volumes[].name`. No orphaned mount.
- Service `selector` → proper subset of the Deployment's pod template
  labels (will actually route traffic to the pods it creates).
- Service `targetPort: http` → resolves to a named `containerPort` on
  the container (not a raw number that could drift from the actual port).
- Each dashboard-provisioning provider's `options.path` →
  matches, exactly, the `mountPath` of the corresponding dashboard
  ConfigMap's volumeMount (Auth Service / Kubernetes / Platform / Alerts,
  all 4 confirmed).
- `grafana.ini`'s `default_home_dashboard_path` → points inside a path
  that's actually mounted (Platform folder).
- `grafana-datasources` ConfigMap's Prometheus `url` → uses the same
  `prometheus-operated.monitoring.svc` governing-Service convention
  Phase 2's `prometheus.yaml` and `servicemonitor-platform-self.yaml`
  already rely on — not a new, unverified assumption.

## 4. Dashboard query sanity (cross-checked against Phase 2 rules)
Every PromQL expression in the Auth Service and Platform dashboards
references a metric or recording rule that actually exists:
- `auth_service:http_requests:rate5m`, `auth_service:http_error_ratio:rate5m`,
  `auth_service:http_request_duration_seconds:{p50,p90,p99}`,
  `auth_service:token_verify_duration_seconds:p99`,
  `auth_service:login_attempts:rate5m`,
  `auth_service:login_failure_ratio:rate5m`,
  `auth_service:active_refresh_tokens:sum` — all defined in
  `prometheusrule-recording-auth-service.yaml` (Phase 2).
- `prometheus_rule_evaluation_failures_total`, `prometheus_tsdb_head_series`,
  `alertmanager_config_hash` — standard Prometheus/Alertmanager
  self-metrics, same ones `prometheusrule-alerts-platform.yaml` already
  alerts on.
- `ALERTS{alertstate="firing"}` — Prometheus's built-in synthetic series,
  always present once any rule group is loaded.

## 5. Known gaps — flagged, not fixed here
- **Kubernetes dashboard** (`node_cpu_seconds_total`,
  `node_memory_MemAvailable_bytes`, `kube_pod_container_status_restarts_total`,
  `kube_pod_status_ready`) depends on node-exporter and kube-state-metrics.
  Neither is deployed by this repository (same pre-existing gap as
  `AuthServicePodCrashLooping` in Phase 2) — panels are valid but will
  read "No data" until those exporters exist. Each panel's `description`
  field documents this inline.
- **Loki/Tempo datasources** intentionally left commented out (see
  `README.md`) — not a bug, a deliberate Phase 4/5 seam.
- **Single-replica Grafana / sqlite backend** is a scaling ceiling, not
  a bug — documented in `deployment.yaml` and `README.md`. No ingress/
  external access method is defined here; left to an overlay, same
  pattern as Phase 2's storage-size layering.

## 6. Runtime validation still required (cannot be done statically)
- `kustomize build kustomize/base/monitoring | kubectl apply --dry-run=server -f -`
  against a cluster with Phase 2 already applied.
- Confirm Grafana actually reaches `prometheus-operated.monitoring.svc:9090`
  (NetworkPolicy in the parent `monitoring` namespace doesn't currently
  restrict egress, so this should work, but hasn't been confirmed against
  a live cluster).
- Confirm the `grafana-admin` Secret has real values before first login.
- Visually confirm all four dashboards render without panel errors once
  Phase 2 has live data flowing.
