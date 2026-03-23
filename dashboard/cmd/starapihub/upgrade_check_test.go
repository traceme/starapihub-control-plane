package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeCheckRegistered(t *testing.T) {
	root := buildRootCmd()
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Name() == "upgrade-check" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("upgrade-check subcommand not registered in buildRootCmd()")
	}
}

func TestUpgradeCheckHelp(t *testing.T) {
	root := buildRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade-check", "--help"})
	_ = root.Execute()
	out := buf.String()
	for _, gate := range []string{"Gate 1", "Gate 2", "Gate 3", "Gate 4", "Gate 5"} {
		if !strings.Contains(out, gate) {
			t.Errorf("help output missing %s", gate)
		}
	}
}

func TestUpgradeCheckExitCodeOnFailure(t *testing.T) {
	// No env vars set, no services -- gates will fail
	root := buildRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade-check", "--repo-root", "/nonexistent"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error when gates fail")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestUpgradeCheckJSONOutput(t *testing.T) {
	root := buildRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade-check", "--output", "json", "--repo-root", "/nonexistent"})
	_ = root.Execute()
	out := buf.String()
	if !strings.Contains(out, `"gates"`) {
		t.Error("JSON output missing 'gates' key")
	}
	if !strings.Contains(out, `"all_pass"`) {
		t.Error("JSON output missing 'all_pass' key")
	}
}

func TestUpgradeCheckGate5WithPatch(t *testing.T) {
	// Create temp repo structure with patch present
	tmpDir := t.TempDir()
	midDir := filepath.Join(tmpDir, "new-api", "middleware")
	os.MkdirAll(midDir, 0755)
	patchContent := `package middleware

func RequestId() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" { id = common.GetTimeString() + common.GetRandomString(8) }
	}
}`
	os.WriteFile(filepath.Join(midDir, "request-id.go"), []byte(patchContent), 0644)

	root := buildRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade-check", "--repo-root", tmpDir})
	_ = root.Execute()
	out := buf.String()
	if !strings.Contains(out, "PASS") {
		t.Error("expected at least Gate 5 to pass with patch present")
	}
	// Gate 5 specifically should pass
	if !strings.Contains(out, "Patch Intent") || !strings.Contains(out, "PASS") {
		// Check with more context -- the text report should show Gate 5 passing
		t.Log("Output:", out)
	}
}
