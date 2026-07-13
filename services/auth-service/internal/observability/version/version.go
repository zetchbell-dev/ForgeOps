// Package version carries Auth Service's build identity — the values
// exposed by metrics.New's auth_service_build_info gauge (M5 Phase 1:
// "Build/version metrics") and available to any other package that wants
// to stamp a log line or trace resource with what's actually running.
package version

import "runtime/debug"

// Version and Commit are populated at build time via
// `-ldflags "-X .../version.Version=... -X .../version.Commit=..."` (see
// deployment/Dockerfile's build stage). The defaults below are
// deliberately obvious placeholders — "dev"/"none" — so a binary built
// without those flags (a local `go build`/`go run`) never gets mistaken
// for a stamped release in a dashboard or log line.
var (
	Version = "dev"
	Commit  = "none"
)

// GoVersion is read from the running binary's own embedded build info at
// startup, not ldflags-injected — that's the toolchain that actually
// built this binary, so there's no separate value to keep in sync by
// hand.
var GoVersion = readGoVersion()

func readGoVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.GoVersion != "" {
		return info.GoVersion
	}
	return "unknown"
}
