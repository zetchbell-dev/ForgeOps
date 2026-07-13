# kustomize/base/monitoring — M5 Phase 2 (Prometheus Platform)

## Prerequisites (install once, outside this kustomization)
1. Prometheus Operator CRDs + controller (Prometheus, Alertmanager,
   ServiceMonitor, PodMonitor, PrometheusRule, ThanosRuler, Probe) —
   typically the upstream `prometheus-operator` Helm chart or the
   `kube-prometheus` jsonnet bundle. `rbac.yaml` in this directory is
   **not** that install-time RBAC; it's the narrower runtime RBAC the
   *running Prometheus server* needs for target discovery.
2. `slack_api_url` in `alertmanager-config-secret.yaml` provisioned
   out-of-band (External Secrets Operator / Sealed Secrets / SOPS) —
   never commit a real webhook URL.

## Apply
```
kubectl apply -k kustomize/base/monitoring
```
This creates the `monitoring` namespace and everything scoped to it.

## Not included here
`networkpolicy.yaml` ships in this directory but is **excluded from the
kustomization** — it's an auth-service NetworkPolicy (podSelector
matches `auth-service` workloads), and belongs in each
`auth-service-{dev,staging,prod}` namespace once the auth-service
kustomize base (M4) exists. Apply it manually per-namespace until then:
```
kubectl apply -f networkpolicy.yaml -n auth-service-dev
kubectl apply -f networkpolicy.yaml -n auth-service-staging
kubectl apply -f networkpolicy.yaml -n auth-service-prod
```

## Scaling / overlay guidance
Base defaults (`prometheus.yaml`: 2 replicas / 50Gi / 15d retention,
`alertmanager.yaml`: 3 replicas / 5Gi / 120h retention) are intentionally
conservative. Environments needing different storage size or retention
should layer a strategic-merge patch in an overlay rather than editing
this base directly.

See `VALIDATION.md` for the static validation performed and the runtime
checks still required before this goes live.
