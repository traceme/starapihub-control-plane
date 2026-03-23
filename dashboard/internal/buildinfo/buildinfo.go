// Package buildinfo provides a single source of truth for appliance version,
// build date, and operating mode. Both the CLI binary and dashboard server
// binary set these values via -ldflags at build time.
//
// Mode detection is based exclusively on the STARAPIHUB_MODE environment
// variable. If unset, the mode is "unknown". The deploy contract
// (docker-compose.yml, env files) is responsible for setting this variable.
// We do not sniff workspace files or guess from image names.
package buildinfo

import "runtime"

// Set via -ldflags. Example:
//
//	go build -ldflags "-X github.com/starapihub/dashboard/internal/buildinfo.Version=0.2.0"
var (
	Version   = "dev"
	BuildDate = "unknown"
)

// Mode returns the appliance operating mode.
//
// The mode is read from the STARAPIHUB_MODE environment variable.
// Valid values: "upstream", "appliance". Any other value (including empty)
// returns "unknown".
//
// This is intentionally strict: if the deploy configuration does not
// explicitly declare the mode, we report "unknown" rather than guessing.
func Mode(getenv func(string) string) string {
	m := getenv("STARAPIHUB_MODE")
	switch m {
	case "upstream":
		return "upstream"
	case "appliance":
		return "appliance"
	default:
		return "unknown"
	}
}

// Info returns all build metadata as a map suitable for JSON serialization.
func Info(getenv func(string) string) map[string]string {
	return map[string]string{
		"version":    Version,
		"build_date": BuildDate,
		"go_version": runtime.Version(),
		"mode":       Mode(getenv),
	}
}
