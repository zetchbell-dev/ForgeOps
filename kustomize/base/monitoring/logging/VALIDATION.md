# M5 Phase 4 — Static Validation Report

Same method as Phases 2/3: no cluster/`kustomize`/`kubeval` available, so
everything below is static (YAML parsing + cross-file reference checks
via a hand-written script, re-run after every edit in this phase).

## 1. YAML syntax
All 10 files in this directory parse cleanly.

## 2. Required kinds present
2x ServiceAccount (loki, fluent-bit), 2x ConfigMap (loki-config,
fluent-bit-config), 1x PersistentVolumeClaim, 1x Deployment (loki), 1x
Service (loki), 1x ClusterRole + 1x ClusterRoleBinding (fluent-bit), 1x
DaemonSet (fluent-bit) — all present.

## 3. Cross-resource reference checks (all PASS)
- Loki Deployment's `serviceAccountName` → ServiceAccount exists.
- Loki's `volumes[].configMap.name` / `.persistentVolumeClaim.claimName`
  → both resolve to resources defined in this directory.
- Loki's container `args: [-config.file=/etc/loki/loki.yaml]` → matches
  the ConfigMap key name (`loki.yaml`) mounted at `/etc/loki`, so the
  path Loki is told to read is the path that actually exists.
- Loki `Service.spec.selector` → proper subset of the Deployment's pod
  labels.
- Both Loki Service ports (`http`, `grpc`) → `targetPort` names resolve
  to actual named `containerPort`s.
- Fluent Bit DaemonSet's `serviceAccountName` → ServiceAccount exists.
- Fluent Bit's `volumes[].configMap.name` → resolves.
- `ClusterRoleBinding` subject (`fluent-bit`, ns `monitoring`) → matches
  the actual ServiceAccount name/namespace.
- Fluent Bit's `fluent-bit.conf` OUTPUT stanza's `Host`/`Port`
  (`loki.monitoring.svc` / `3100`) → matches Loki's actual Service name
  and port exactly — not just assumed, cross-checked against
  `loki-service.yaml` directly.
- The INPUT's `Parser cri` directive → resolves to a parser actually
  defined in the same ConfigMap's `parsers.conf` (`docker` is also
  defined, as the documented fallback).

## 4. Cross-phase integration check (Grafana ↔ Loki)
`../grafana/configmap-datasources.yaml`'s Loki datasource stanza
(previously commented out in Phase 3) was uncommented as part of this
phase and its `url: http://loki.monitoring.svc:3100` re-verified against
this phase's actual `loki-service.yaml` — name and port match exactly,
not carried over as an unchecked assumption.

## 5. Known gaps — flagged, not fixed here
- `Parser cri` assumes a containerd/CRI-format node log — flagged inline
  in `fluentbit-configmap.yaml` and in `README.md`. The `docker` parser
  exists as a documented fallback but isn't switched on automatically —
  doing so blindly could be wrong in the other direction.
- No log volume/retention monitoring (e.g., an alert on Loki approaching
  `loki-data`'s 50Gi PVC capacity) is added in this phase — out of the
  requested scope, but worth a Phase 6 ("Platform Integration &
  Alerting") follow-up.
- Fluent Bit intentionally runs as root — a documented departure from
  this repository's usual `runAsNonRoot` convention, not an oversight
  (see `README.md`).

## 6. Runtime validation still required (cannot be done statically)
- `kustomize build kustomize/base/monitoring | kubectl apply --dry-run=server -f -`
  against a cluster with Phase 2 (and ideally Phase 3) already applied.
- Confirm the node AMI's actual container log format matches the `cri`
  parser assumption (`kubectl logs` won't reveal this — needs a raw file
  read on a node, e.g. `cat /var/log/containers/<pod>.log`).
- Confirm Fluent Bit pods reach `Ready` on every node (DaemonSet rollout
  status) and that log lines are actually arriving in Loki
  (`curl loki:3100/loki/api/v1/labels` from inside the cluster).
- Confirm Grafana's Loki datasource shows green in Settings → Data
  Sources once both are live.
