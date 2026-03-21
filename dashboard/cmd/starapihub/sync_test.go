package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncCmd_HelpShowsAllFlags(t *testing.T) {
	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"sync", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sync --help error: %v", err)
	}

	output := buf.String()
	for _, flag := range []string{"--dry-run", "--prune", "--fail-fast", "--target", "--verbose", "--output"} {
		if !strings.Contains(output, flag) {
			t.Errorf("sync --help missing flag %s in output:\n%s", flag, output)
		}
	}
}

func TestDiffCmd_HelpShowsDiff(t *testing.T) {
	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diff", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("diff --help error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "diff") {
		t.Errorf("diff --help should mention diff:\n%s", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Errorf("diff --help should mention dry-run:\n%s", output)
	}
}

func TestSyncCmd_WithoutEnvVars_ReturnsError(t *testing.T) {
	// Ensure env vars are unset
	t.Setenv("NEWAPI_URL", "")
	t.Setenv("NEWAPI_ADMIN_TOKEN", "")
	t.Setenv("BIFROST_URL", "")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Point to a temp dir for config to avoid load errors
	tmpDir := t.TempDir()
	cmd.SetArgs([]string{"sync", "--config-dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when NEWAPI_URL is not set")
	}
	if !strings.Contains(err.Error(), "NEWAPI_URL") {
		t.Errorf("error should mention NEWAPI_URL, got: %v", err)
	}
}

func TestSyncCmd_DryRunFlagParses(t *testing.T) {
	// Verify the flag is accepted without error (will fail on missing env vars after)
	t.Setenv("NEWAPI_URL", "")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	tmpDir := t.TempDir()
	cmd.SetArgs([]string{"sync", "--dry-run", "--config-dir", tmpDir})

	err := cmd.Execute()
	// Should fail on missing env var, not on flag parsing
	if err == nil {
		t.Fatal("expected error from missing env vars")
	}
	if !strings.Contains(err.Error(), "NEWAPI_URL") {
		t.Errorf("should fail on env var not flag parsing, got: %v", err)
	}
}
