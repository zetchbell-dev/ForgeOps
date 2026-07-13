# M5 Phase 2 — Static Validation Report

Performed without cluster/`kustomize`/`kubeval` access (none available in this
environment). All checks below are static: YAML parsing + cross-referencing
field values across files by hand-written script. **Runtime validation
(listed at the bottom) still has to happen against a real cluster before
this is considered done.**

## 1. YAML syntax
All 12 pre-existing manifests + `pdb.yaml` + `kustomization.yaml` parse
cleanly with `yaml.safe_load_all` (14 files, 18 documents). No syntax errors.

## 2. Required kinds present (one of each expected)
Namespace, 2x ServiceAccount, ClusterRole, ClusterRoleBinding, Prometheus,
Alertmanager, Secret, 3x ServiceMonitor, 3x PrometheusRule, 2x
PodDisruptionBudget — all confirmed present with the names other files
reference them by.

## 3. Cross-resource reference checks (all PASS)
- `prometheus.yaml` `spec.serviceAccountName: prometheus` → `serviceaccount.yaml` defines it.
- `alertmanager.yaml` `spec.serviceAccountName: alertmanager` → `serviceaccount.yaml` defines it.
- `rbac.yaml` ClusterRoleBinding subject (`prometheus`, ns `monitoring`) → matches the SA + the namespace this kustomization applies.
- `alertmanager.yaml` `spec.configSecret: alertmanager-alertmanager` → `alertmanager-config-secret.yaml` Secret name matches exactly.
- `prometheus.yaml` `spec.alerting.alertmanagers[0]` (`alertmanager-operated` / `monitoring`) → matches the Operator's fixed governing-Service naming for an Alertmanager CR named `alertmanager`.
- `pdb.yaml` selectors (both PDBs) → proper subset of the corresponding CR's `podMetadata.labels`, so the PDB will actually match the pods the Operator creates.
- `networkpolicy.yaml`'s prometheus `podSelector` → proper subset of `prometheus.yaml`'s `podMetadata.labels`.
- Every `auth_service:*` recording-rule name used inside `prometheusrule-alerts-auth-service.yaml`'s alert `expr` fields resolves to a rule actually defined in `prometheusrule-recording-auth-service.yaml`. No dangling references.
- `alertmanager-config-secret.yaml`'s inhibit rule (`source: AuthServiceDown`, `target: AuthServiceHigh.*`) → both match real alert names in `prometheusrule-alerts-auth-service.yaml`.

## 4. Prometheus Operator compatibility
All CR manifests use the current stable `monitoring.coreos.com/v1` API
(no `v1alpha1`/beta CRD versions). `pdb.yaml` uses `policy/v1` (not the
removed `policy/v1beta1`), safe for any currently-supported Kubernetes
version.

## 5. Known gaps — NOT fixed in this pass, flagged rather than silently ignored
- **`AuthServicePodCrashLooping`** (in `prometheusrule-alerts-auth-service.yaml`) depends on `kube_pod_container_status_restarts_total`, which requires kube-state-metrics. Not deployed by any file in this directory — the alert is valid but will never fire until that's added (pre-existing, not introduced by Phase 2).
- **`servicemonitor-platform-self.yaml`**'s `operated-prometheus`/`operated-alertmanager` label assumption is unverified against the actual Operator version that will be installed in-cluster (pre-existing "ASSUMPTION FLAG" in that file, still open).
- **`networkpolicy.yaml`** is *not* part of this kustomization (see `kustomization.yaml` header comment) — its podSelector targets auth-service workloads in `auth-service-{dev,staging,prod}`, namespaces this repo's kustomize tree doesn't contain yet (M4 gap). It has to be applied per-namespace manually, or folded into the auth-service overlays once that base exists.

## 6. Runtime validation still required (cannot be done statically)
- `kustomize build kustomize/base/monitoring | kubectl apply --dry-run=server -f -` once a cluster with the Prometheus Operator CRDs installed is available.
- Confirm the Operator's actual governing-Service labels (`operated-prometheus`/`operated-alertmanager`) match `servicemonitor-platform-self.yaml`'s selector for the Operator version actually deployed.
- Confirm `alertmanager-config-secret.yaml`'s `slack_api_url` has been populated out-of-band before relying on the Slack receiver.
- Confirm kube-state-metrics is deployed if `AuthServicePodCrashLooping` is expected to ever fire.
