# kustomize/base/monitoring/tracing — M5 Phase 5 (Distributed Tracing)

## What this deploys
- **OTel Collector** (`otel-collector-*`): receives OTLP traces over both
  gRPC (4317) and HTTP (4318), enriches spans with pod/namespace metadata
  via the `k8sattributes` processor, batches, and forwards to Tempo.
- **Tempo** (`tempo-*`): single-binary trace storage, local-disk backend,
  exposes its own OTLP receiver (4317/4318, used directly by anything
  that wants to skip the Collector) plus the HTTP query API (3200) that
  Grafana's Tempo datasource queries.

## Wiring auth-service (or any other service) to this Collector
Set, per M5 Phase 1's `tracing.Config`:
```
OTEL_EXPORTER_OTLP_ENDPOINT = otel-collector.monitoring.svc:4318
```
No code change required — Phase 1's `tracing.go` treats an empty
endpoint as a no-op and a populated one as "go live" (see that file's
package doc comment).

## Trace propagation
W3C `traceparent`/`baggage` propagation is installed unconditionally by
Phase 1's `tracing.Setup` regardless of whether tracing export is
enabled, so an incoming header from an already-instrumented upstream
still passes through. Nothing in this directory implements propagation
itself — that's SDK-side, not Collector-side.

## Trace sampling
Head-based sampling (`ParentBased(TraceIDRatioBased(ratio))`) lives in
each service's own SDK config (Phase 1's `tracing.Config.SampleRatio`),
not here. `otel-collector-configmap.yaml` deliberately does not run a
second probabilistic sampler on top of that — see that file's header
comment for why stacking two independent samplers is worse, not safer.
A commented-out `tail_sampling` processor there documents the upgrade
path once real prod traffic volume justifies keep-if-error/keep-if-slow
policies a head sampler can't express.

## Prerequisites
None beyond what Phase 2–4 already require (the `monitoring` namespace,
already created by `../namespace.yaml`). No external Operator/CRDs
needed for this phase — Tempo and the Collector are plain Deployments.

## Apply
```
kubectl apply -k kustomize/base/monitoring
```
(this directory is included as a resource from the parent monitoring
kustomization, not applied standalone).

## Known limitations
- Single Collector replica + single Tempo replica: see
  `otel-collector-deployment.yaml` and `tempo-deployment.yaml` header
  comments for why (tail-sampling trace-ID sharding, single-writer local
  disk, respectively) — both documented, not accidental.
- Local-disk trace storage only, retention bounded by `tempo-pvc.yaml`'s
  20Gi and `tempo-configmap.yaml`'s 360h compactor retention — swap for
  an object-store backend in an overlay before running at production
  trace volume.

See `VALIDATION.md` for the static validation performed and the runtime
checks still required before this goes live.
