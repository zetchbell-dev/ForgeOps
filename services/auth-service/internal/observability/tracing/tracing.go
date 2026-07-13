// Package tracing bootstraps the OpenTelemetry SDK's TracerProvider for
// Auth Service (M5 §7: "OpenTelemetry SDK instruments: HTTP handler
// entry, ... each as a child span under the request's root span" and
// "Trace ID ... propagated via traceparent header").
//
// M5's own implementation order (§11) and this milestone's Phase 1-3
// scope (Instrumentation → Prometheus → Grafana) do not include standing
// up the Jaeger backend §2's diagram shows — that's still a design-doc
// commitment, not a deployed target in this repository yet. Setup is
// therefore config-gated and defaults to a no-op: with no OTLP endpoint
// configured, otel's own built-in no-op tracer stays installed (every
// span created anywhere in the codebase is a valid, harmless no-op, and
// traceparent propagation still works so a header arriving from an
// upstream Gateway that DOES have tracing enabled passes through
// correctly) rather than pointing a real exporter at a collector that
// doesn't exist yet. Flipping this on for real is just setting
// OTEL_EXPORTER_OTLP_ENDPOINT once a collector exists — no code change
// required.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config controls whether/how the tracer provider is wired. Populated
// from environment variables at the composition root (cmd/server/main.go),
// the same convention config.Config already follows.
type Config struct {
	// Endpoint is the OTLP/HTTP collector endpoint (host:port, no
	// scheme, matching otlptracehttp.WithEndpoint's expected format).
	// Empty means tracing stays a no-op — see package doc comment.
	Endpoint string
	// Insecure disables TLS to the collector. true is the expected value
	// for an in-cluster collector, which is the only deployment shape
	// this milestone's design doc describes (M5 §2's diagram has no
	// external Jaeger endpoint).
	Insecure bool
	// SampleRatio is the head-based sampling probability in [0,1]. M5
	// §7: 100% (1.0) in dev/staging, a lower tuned ratio in prod once
	// real traffic volume is known — that tuning is a config change
	// (this field), not a code change.
	SampleRatio float64
	// ServiceName and ServiceVersion populate the OTel resource
	// attributes every span this process emits carries.
	ServiceName    string
	ServiceVersion string
}

// noopShutdown is returned by Setup when tracing is disabled, so callers
// always get a valid shutdown func to defer without a nil check.
func noopShutdown(context.Context) error { return nil }

// Setup installs the global TracerProvider and the W3C
// tracecontext/baggage propagator (M5 §7's traceparent propagation). The
// returned shutdown func flushes and closes the exporter; callers must
// defer it regardless of whether tracing is enabled (see noopShutdown).
//
// The propagator is installed unconditionally, even when cfg.Endpoint is
// empty — an incoming traceparent header from a service upstream that
// DOES export spans should still be read and passed along, not dropped,
// even while this process's own spans are no-ops.
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.Endpoint == "" {
		return noopShutdown, nil
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("constructing otlp trace exporter: %w", err)
	}

	// service.name/service.version are set as plain attribute.KeyValue
	// pairs rather than through the semconv package: semconv ships as a
	// spec-version-pinned subpackage (e.g. semconv/v1.24.0) whose exact
	// version has to track whatever otel/sdk version go.mod resolves to,
	// which this change can't verify against a live module proxy. The
	// two keys used here ("service.name", "service.version") are stable,
	// well-known OTel resource semantic conventions regardless of which
	// semconv subpackage version they're also declared in.
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("building otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
