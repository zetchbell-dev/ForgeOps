package tracing_test

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/tracing"
)

// These tests exercise tracing.Setup without a real OTLP collector.
// Per tracing.go's own doc comment, an empty cfg.Endpoint is the no-op
// path (otel's built-in no-op tracer stays installed) — that's the
// primary case exercised here. The "configured endpoint" cases only
// verify that construction succeeds and returns a non-nil, callable
// shutdown func; they deliberately use a short-timeout context around
// shutdown rather than asserting shutdown succeeds, since flushing to an
// endpoint that isn't actually reachable in a test environment is
// expected to fail or time out, not something this suite should treat as
// a Setup defect.

func TestSetup_EmptyEndpointIsNoop(t *testing.T) {
	shutdown, err := tracing.Setup(context.Background(), tracing.Config{
		Endpoint:       "",
		ServiceName:    "auth-service-test",
		ServiceVersion: "test",
	})
	if err != nil {
		t.Fatalf("Setup with empty endpoint returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Setup with empty endpoint returned a nil shutdown func, want a valid (noop) one")
	}
}

func TestSetup_EmptyEndpointShutdownIsSafeNoop(t *testing.T) {
	shutdown, err := tracing.Setup(context.Background(), tracing.Config{Endpoint: ""})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown() returned error: %v, want nil", err)
	}
	// Calling it twice must also be safe — nothing in noopShutdown
	// (tracing.go) is stateful.
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown() called a second time returned error: %v, want nil", err)
	}
}

func TestSetup_PropagatorInstalledEvenWhenEndpointEmpty(t *testing.T) {
	// Setup unconditionally installs the composite TraceContext+Baggage
	// propagator (tracing.go), even on the no-op path — this is the
	// behavior an incoming traceparent header from an upstream service
	// depends on.
	_, err := tracing.Setup(context.Background(), tracing.Config{Endpoint: ""})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		t.Fatal("expected a non-nil global text map propagator after Setup")
	}
	fields := propagator.Fields()
	wantFields := map[string]bool{"traceparent": false, "baggage": false}
	for _, f := range fields {
		if _, ok := wantFields[f]; ok {
			wantFields[f] = true
		}
	}
	for field, found := range wantFields {
		if !found {
			t.Errorf("expected global propagator's Fields() to include %q, got %v", field, fields)
		}
	}
}

func TestSetup_ConfiguredEndpointConstructsSuccessfully(t *testing.T) {
	// otlptracehttp's client is lazy about dialing (tracing.go's own doc
	// comment: "doesn't actively connect ... on New()"), so pointing it
	// at an endpoint with nothing listening should still construct
	// without error — no network call happens during Setup itself.
	shutdown, err := tracing.Setup(context.Background(), tracing.Config{
		Endpoint:       "127.0.0.1:0",
		Insecure:       true,
		SampleRatio:    1.0,
		ServiceName:    "auth-service-test",
		ServiceVersion: "test",
	})
	if err != nil {
		t.Fatalf("Setup with a configured (unreachable) endpoint returned error: %v, want construction to succeed lazily", err)
	}
	if shutdown == nil {
		t.Fatal("Setup with a configured endpoint returned a nil shutdown func")
	}

	// Bound the flush attempt so an unreachable collector can't hang
	// this test suite; a timeout/connection error here is expected and
	// not asserted against, only that shutdown returns instead of
	// blocking indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = shutdown(ctx)
}

func TestSetup_ZeroSampleRatioConstructsSuccessfully(t *testing.T) {
	// SampleRatio is a plain float passed straight into
	// sdktrace.TraceIDRatioBased (tracing.go) — 0.0 is a valid ratio
	// (sample nothing) and must not error at construction time.
	shutdown, err := tracing.Setup(context.Background(), tracing.Config{
		Endpoint:    "127.0.0.1:0",
		Insecure:    true,
		SampleRatio: 0.0,
	})
	if err != nil {
		t.Fatalf("Setup with SampleRatio=0 returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = shutdown(ctx)
}
