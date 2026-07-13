package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/metrics"
)

// newTestMetrics gives each test its own registry, so tests can run in
// parallel (or just repeatedly) without colliding on collector names —
// exactly the isolation reason metrics.New takes a *prometheus.Registry
// parameter instead of using prometheus.DefaultRegisterer.
func newTestMetrics(t *testing.T) (*metrics.Metrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	return metrics.New(reg), reg
}

func TestObserveHTTPRequest_IncrementsCounterAndHistogram(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.ObserveHTTPRequest("GET", "/v1/auth/verify", 200, 15*time.Millisecond)
	m.ObserveHTTPRequest("GET", "/v1/auth/verify", 200, 20*time.Millisecond)
	m.ObserveHTTPRequest("POST", "/v1/auth/login", 401, 5*time.Millisecond)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	var sawGetOK, sawPostUnauthorized bool
	for _, mf := range families {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range metric.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			switch {
			case labels["method"] == "GET" && labels["path"] == "/v1/auth/verify" && labels["status"] == "200":
				if got := metric.GetCounter().GetValue(); got != 2 {
					t.Errorf("GET /v1/auth/verify 200 count = %v, want 2", got)
				}
				sawGetOK = true
			case labels["method"] == "POST" && labels["path"] == "/v1/auth/login" && labels["status"] == "401":
				if got := metric.GetCounter().GetValue(); got != 1 {
					t.Errorf("POST /v1/auth/login 401 count = %v, want 1", got)
				}
				sawPostUnauthorized = true
			}
		}
	}
	if !sawGetOK {
		t.Error("expected a http_requests_total series for GET /v1/auth/verify status=200")
	}
	if !sawPostUnauthorized {
		t.Error("expected a http_requests_total series for POST /v1/auth/login status=401")
	}

	var histogramSampleCount uint64
	for _, mf := range families {
		if mf.GetName() != "http_request_duration_seconds" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			histogramSampleCount += metric.GetHistogram().GetSampleCount()
		}
	}
	if histogramSampleCount != 3 {
		t.Errorf("http_request_duration_seconds total sample count = %d, want 3", histogramSampleCount)
	}
}

func TestObserveLoginAttempt_LabelsResultCorrectly(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.ObserveLoginAttempt(metrics.LoginResultSuccess)
	m.ObserveLoginAttempt(metrics.LoginResultRateLimited)
	m.ObserveLoginAttempt(metrics.LoginResultRateLimited)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	counts := map[string]float64{}
	for _, mf := range families {
		if mf.GetName() != "auth_login_attempts_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, l := range metric.GetLabel() {
				if l.GetName() == "result" {
					counts[l.GetValue()] = metric.GetCounter().GetValue()
				}
			}
		}
	}

	if counts[metrics.LoginResultSuccess] != 1 {
		t.Errorf("success count = %v, want 1", counts[metrics.LoginResultSuccess])
	}
	if counts[metrics.LoginResultRateLimited] != 2 {
		t.Errorf("rate_limited count = %v, want 2", counts[metrics.LoginResultRateLimited])
	}
}

func TestActiveRefreshTokens_IncAndDec(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.IncActiveRefreshTokens()
	m.IncActiveRefreshTokens()
	m.DecActiveRefreshTokens()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != "auth_active_refresh_tokens" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if got := metric.GetGauge().GetValue(); got != 1 {
				t.Errorf("auth_active_refresh_tokens = %v, want 1", got)
			}
		}
	}
}

func TestObserveTokenVerifyDuration_RecordsObservation(t *testing.T) {
	m, reg := newTestMetrics(t)

	m.ObserveTokenVerifyDuration(10 * time.Millisecond)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}
	var sampleCount uint64
	for _, mf := range families {
		if mf.GetName() != "auth_token_verify_duration_seconds" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			sampleCount = metric.GetHistogram().GetSampleCount()
		}
	}
	if sampleCount != 1 {
		t.Errorf("auth_token_verify_duration_seconds sample count = %d, want 1", sampleCount)
	}
}

func TestNew_RegistersBuildInfoAndRuntimeCollectors(t *testing.T) {
	_, reg := newTestMetrics(t)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	names := map[string]bool{}
	for _, mf := range families {
		names[mf.GetName()] = true
	}

	for _, want := range []string{"auth_service_build_info", "go_goroutines", "process_start_time_seconds"} {
		if !names[want] {
			t.Errorf("expected registered metric %q, not found among %d families", want, len(families))
		}
	}
}
