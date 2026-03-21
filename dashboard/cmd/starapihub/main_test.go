package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryBuilds(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "starapihub")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = getPackageDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary not found after build: %v", err)
	}
}

func TestCLIHelp(t *testing.T) {
	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "starapihub") {
		t.Errorf("help output should contain 'starapihub', got: %s", output)
	}
	if !strings.Contains(output, "validate") {
		t.Errorf("help output should list 'validate' subcommand, got: %s", output)
	}
	for _, sub := range []string{"sync", "diff", "bootstrap", "health"} {
		if !strings.Contains(output, sub) {
			t.Errorf("help output should list '%s' subcommand, got: %s", sub, output)
		}
	}
}

func TestValidateCommandValid(t *testing.T) {
	repoRoot := findRepoRoot(t)
	policiesDir := filepath.Join(repoRoot, "control-plane", "policies")
	schemasDir := filepath.Join(repoRoot, "control-plane", "schemas")

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"validate", "--config-dir", policiesDir, "--schemas-dir", schemasDir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("validate with valid policies should succeed, got: %v\noutput: %s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "passed") && !strings.Contains(strings.ToLower(output), "valid") {
		t.Errorf("expected success message containing 'passed' or 'valid', got: %s", output)
	}
}

func TestValidateCommandInvalid(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := findRepoRoot(t)

	policiesDir := filepath.Join(tmp, "policies")
	schemasDir := filepath.Join(repoRoot, "control-plane", "schemas")
	os.MkdirAll(policiesDir, 0755)

	// Write invalid models.yaml (missing required display_name)
	os.WriteFile(filepath.Join(policiesDir, "models.yaml"), []byte(`models:
  bad:
    billing_name: "bad"
    upstream_model: "test"
    risk_level: low
    allowed_groups: ["all"]
    channel: "test"
    route_policy: "test"
`), 0644)

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"validate", "--config-dir", policiesDir, "--schemas-dir", schemasDir})

	err := rootCmd.Execute()
	// The validate command calls os.Exit(1) on validation failure,
	// but when testing via cobra Execute(), the RunE returns nil (it prints and exits).
	// We need to check error output instead.
	combined := buf.String() + errBuf.String()
	if err == nil && !strings.Contains(strings.ToLower(combined), "fail") && !strings.Contains(strings.ToLower(combined), "error") {
		t.Errorf("expected validation failure output, got: %s", combined)
	}
}

func TestStubCommands(t *testing.T) {
	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"sync"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected stub command to return error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

// findRepoRoot walks up to find the starapihub repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "control-plane", "schemas")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

// getPackageDir returns the directory of this test package.
func getPackageDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return dir
}
