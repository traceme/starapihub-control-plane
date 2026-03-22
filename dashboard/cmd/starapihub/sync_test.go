package main

import (
	"bytes"
	"errors"
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
	if !strings.Contains(output, "drift") {
		t.Errorf("diff --help should mention drift:\n%s", output)
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

func TestDiffCmd_HelpShowsDriftFlags(t *testing.T) {
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
	for _, flag := range []string{"--severity", "--exit-warn", "--report-file", "--target", "--verbose", "--output"} {
		if !strings.Contains(output, flag) {
			t.Errorf("diff --help missing flag %s in output:\n%s", flag, output)
		}
	}
}

func TestDiffCmd_SeverityFlagAcceptsValues(t *testing.T) {
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
	// Verify the severity flag is documented with its default
	if !strings.Contains(output, "severity") {
		t.Errorf("diff --help should document severity flag:\n%s", output)
	}
	if !strings.Contains(output, "warning") {
		t.Errorf("diff --help should show default severity value 'warning':\n%s", output)
	}
}

func TestDiffCmd_WithoutEnvVars_ReturnsError(t *testing.T) {
	t.Setenv("NEWAPI_URL", "")
	t.Setenv("NEWAPI_ADMIN_TOKEN", "")
	t.Setenv("BIFROST_URL", "")

	cmd := buildRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	tmpDir := t.TempDir()
	cmd.SetArgs([]string{"diff", "--config-dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when NEWAPI_URL is not set")
	}
	if !strings.Contains(err.Error(), "NEWAPI_URL") {
		t.Errorf("error should mention NEWAPI_URL, got: %v", err)
	}
}

func TestDiffCmd_IsNotSyncAlias(t *testing.T) {
	cmd := buildRootCmd()
	diffCmd, _, _ := cmd.Find([]string{"diff"})
	if diffCmd == nil {
		t.Fatal("diff command not found")
	}
	if diffCmd.Flags().Lookup("dry-run") != nil {
		t.Error("diff should not have --dry-run flag")
	}
	if diffCmd.Flags().Lookup("prune") != nil {
		t.Error("diff should not have --prune flag")
	}
	if diffCmd.Flags().Lookup("fail-fast") != nil {
		t.Error("diff should not have --fail-fast flag")
	}
	// But it should have drift-specific flags
	if diffCmd.Flags().Lookup("severity") == nil {
		t.Error("diff should have --severity flag")
	}
	if diffCmd.Flags().Lookup("exit-warn") == nil {
		t.Error("diff should have --exit-warn flag")
	}
	if diffCmd.Flags().Lookup("report-file") == nil {
		t.Error("diff should have --report-file flag")
	}
}

func TestExitError_ImplementsError(t *testing.T) {
	var err error = &ExitError{Code: 2}
	if err.Error() != "exit code 2" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal("ExitError should satisfy errors.As")
	}
	if exitErr.Code != 2 {
		t.Errorf("expected code 2, got %d", exitErr.Code)
	}

	// Test different codes
	err1 := &ExitError{Code: 1}
	if err1.Error() != "exit code 1" {
		t.Errorf("unexpected error message: %s", err1.Error())
	}
}
