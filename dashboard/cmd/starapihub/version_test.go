package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/starapihub/dashboard/internal/buildinfo"
)

func TestVersionCmd_TextOutput(t *testing.T) {
	buildinfo.Version = "0.2.0-test"
	buildinfo.BuildDate = "2026-03-22T00:00:00Z"
	t.Setenv("STARAPIHUB_MODE", "")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version", "--output", "text"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.2.0-test") {
		t.Errorf("expected version in output, got: %s", out)
	}
	if !strings.Contains(out, "2026-03-22") {
		t.Errorf("expected build date in output, got: %s", out)
	}
	if !strings.Contains(out, "unknown") {
		t.Errorf("expected mode=unknown when env not set, got: %s", out)
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	buildinfo.Version = "0.2.0-test"
	buildinfo.BuildDate = "2026-03-22T00:00:00Z"
	t.Setenv("STARAPIHUB_MODE", "")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"version":"0.2.0-test"`) {
		t.Errorf("expected version in JSON output, got: %s", out)
	}
	if !strings.Contains(out, `"mode":"unknown"`) {
		t.Errorf("expected mode=unknown in JSON output, got: %s", out)
	}
}

func TestVersionCmd_ModeFromEnv(t *testing.T) {
	buildinfo.Version = "0.2.0-test"
	t.Setenv("STARAPIHUB_MODE", "appliance")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"mode":"appliance"`) {
		t.Errorf("expected mode=appliance, got: %s", out)
	}
}

func init() {
	// Ensure tests don't inherit STARAPIHUB_MODE from the calling environment
	os.Unsetenv("STARAPIHUB_MODE")
}
