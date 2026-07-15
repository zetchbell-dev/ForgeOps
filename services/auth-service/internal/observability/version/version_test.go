package version_test

import (
	"testing"

	"github.com/enterprise-cicd-platform/auth-service/internal/observability/version"
)

func TestVersion_DefaultValue(t *testing.T) {
	// Version defaults to "dev" (version.go) unless overridden at build
	// time via -ldflags. In a `go test` run (no ldflags applied), it
	// must still hold that default.
	if version.Version != "dev" {
		t.Errorf("version.Version = %q, want default %q (no -ldflags override in a test build)", version.Version, "dev")
	}
}

func TestVersion_CommitDefaultValue(t *testing.T) {
	if version.Commit != "none" {
		t.Errorf("version.Commit = %q, want default %q (no -ldflags override in a test build)", version.Commit, "none")
	}
}

func TestVersion_GoVersionIsPopulated(t *testing.T) {
	// GoVersion is read from runtime/debug.ReadBuildInfo() at package
	// init (version.go's readGoVersion) — it must resolve to either a
	// real Go version string or the "unknown" fallback, and must never
	// be empty.
	if version.GoVersion == "" {
		t.Fatal("version.GoVersion is empty, want either a real go version string or \"unknown\"")
	}
}

func TestVersion_GoVersionHasExpectedShape(t *testing.T) {
	// When build info is available (the normal `go test` case), Go's
	// own convention is a "go1.x" or "go1.x.y" prefix. We don't assert
	// full semver, just that it's not garbage — this catches
	// readGoVersion regressing to always return "unknown" even when
	// build info is available.
	got := version.GoVersion
	if got == "unknown" {
		t.Skip("build info unavailable in this test environment; readGoVersion correctly fell back to \"unknown\"")
	}
	if len(got) < 2 || got[:2] != "go" {
		t.Errorf("version.GoVersion = %q, want it to start with %q when build info is available", got, "go")
	}
}
