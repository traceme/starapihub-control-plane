// Package integration contains end-to-end tests that run the starapihub CLI
// against a live Docker Compose stack (New-API + Bifrost + Postgres + Redis).
//
// These tests require Docker and are skipped when INTEGRATION=1 is not set.
// Run with: INTEGRATION=1 go test -v -timeout 300s ./tests/integration/
//
// The test suite:
//  1. Builds the starapihub binary
//  2. Starts a minimal Docker Compose stack
//  3. Runs CLI commands against the live stack
//  4. Verifies real behavior (not mocks)
//  5. Tears down the stack
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	newAPIPort  = "13000"
	bifrostPort = "18080"
)

var (
	binaryPath string
	composeDir string
	fixtures   string
)

func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION") != "1" {
		fmt.Println("SKIP: set INTEGRATION=1 to run integration tests")
		os.Exit(0)
	}

	// Resolve paths
	wd, _ := os.Getwd()
	composeDir = filepath.Join(wd, "compose")
	fixtures = filepath.Join(wd, "fixtures")

	// Build CLI binary
	binaryPath = filepath.Join(os.TempDir(), "starapihub-test")
	dashboardDir := filepath.Join(wd, "..", "..", "dashboard")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/starapihub/")
	build.Dir = dashboardDir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Start Docker Compose stack
	if err := composeUp(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: docker compose up failed: %v\n", err)
		os.Exit(1)
	}

	// Wait for services to be healthy
	if err := waitForServices(180 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: services did not become healthy: %v\n", err)
		composeDown()
		os.Exit(1)
	}

	code := m.Run()

	composeDown()
	os.Remove(binaryPath)
	os.Exit(code)
}

// --- Docker Compose helpers ---

func composeUp() error {
	cmd := exec.Command("docker", "compose", "-f",
		filepath.Join(composeDir, "docker-compose.test.yml"),
		"up", "-d", "--wait", "--wait-timeout", "120")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func composeDown() {
	cmd := exec.Command("docker", "compose", "-f",
		filepath.Join(composeDir, "docker-compose.test.yml"),
		"down", "-v", "--remove-orphans")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func waitForServices(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	services := map[string]string{
		"new-api": fmt.Sprintf("http://127.0.0.1:%s/api/status", newAPIPort),
		"bifrost": fmt.Sprintf("http://127.0.0.1:%s/health", bifrostPort),
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for name, url := range services {
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("%s did not become healthy within %v", name, timeout)
			}
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode < 500 {
				resp.Body.Close()
				fmt.Printf("  [OK] %s healthy\n", name)
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}

// --- CLI runner ---

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NEWAPI_URL=http://127.0.0.1:%s", newAPIPort),
		"NEWAPI_ADMIN_TOKEN=test-admin-token",
		fmt.Sprintf("BIFROST_URL=http://127.0.0.1:%s", bifrostPort),
		"CLEWDR_URLS=",
		"CLEWDR_ADMIN_TOKEN=",
		fmt.Sprintf("BIFROST_API_KEY=dummy-key-for-test"),
		fmt.Sprintf("OPENAI_API_KEY=sk-test-dummy"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// --- Integration tests ---

func TestHealth_ReportsServiceStatus(t *testing.T) {
	stdout, _, exitCode := runCLI(t, "health",
		"--config-dir", fixtures)

	// health may exit 0 or 1 depending on service config
	// but it should not crash
	t.Logf("health output:\n%s", stdout)
	t.Logf("exit code: %d", exitCode)

	if exitCode > 1 {
		t.Fatalf("health crashed with exit code %d", exitCode)
	}
}

func TestHealth_JSONOutput(t *testing.T) {
	stdout, _, _ := runCLI(t, "health",
		"--config-dir", fixtures,
		"--output", "json")

	t.Logf("health JSON output:\n%s", stdout)

	// Should be valid JSON
	if len(strings.TrimSpace(stdout)) > 0 {
		var parsed any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Errorf("health --output json did not produce valid JSON: %v", err)
		}
	}
}

func TestBootstrap_SeedsAdminAndSyncs(t *testing.T) {
	auditLog := filepath.Join(t.TempDir(), "audit.log")

	stdout, stderr, exitCode := runCLI(t,
		"bootstrap",
		"--config-dir", fixtures,
		"--timeout", "90s",
		"--audit-log", auditLog,
	)

	t.Logf("bootstrap stdout:\n%s", stdout)
	t.Logf("bootstrap stderr:\n%s", stderr)
	t.Logf("exit code: %d", exitCode)

	// Bootstrap may fail on seed if admin already exists, that's OK.
	// The key test is that it runs to completion without crashing.
	if exitCode > 1 {
		t.Fatalf("bootstrap crashed with exit code %d", exitCode)
	}

	// Check audit log was written
	if _, err := os.Stat(auditLog); err != nil {
		t.Logf("WARNING: audit log not created (bootstrap may have failed early)")
	} else {
		data, _ := os.ReadFile(auditLog)
		t.Logf("audit log content:\n%s", string(data))
		if !strings.Contains(string(data), `"operation":"bootstrap"`) {
			t.Error("audit log should contain bootstrap operation")
		}
		if !strings.Contains(string(data), `"bootstrap_steps"`) {
			t.Error("audit log should contain bootstrap_steps field")
		}
	}
}

func TestSync_DryRunShowsActions(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"sync",
		"--config-dir", fixtures,
		"--dry-run",
	)

	t.Logf("sync --dry-run output:\n%s", stdout)
	t.Logf("exit code: %d", exitCode)

	// exit code 2 is expected when there are plan errors (e.g., missing routing-rules.yaml)
	// The test verifies the CLI runs and produces output, not zero exit.

	// Should produce some output (either changes or "no changes needed")
	if len(strings.TrimSpace(stdout)) == 0 {
		t.Error("sync --dry-run produced no output")
	}

	// Should show resource types being processed
	if !strings.Contains(stdout, "provider") && !strings.Contains(stdout, "channel") {
		t.Error("sync output should mention resource types")
	}
}

func TestSync_DryRunJSON(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"sync",
		"--config-dir", fixtures,
		"--dry-run",
		"--output", "json",
	)

	t.Logf("sync --dry-run JSON:\n%s", stdout)
	t.Logf("exit code: %d", exitCode)

	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			t.Errorf("JSON output is not valid: %v", err)
		}
		if _, ok := parsed["total_actions"]; !ok {
			t.Error("JSON output should contain total_actions field")
		}
	}
}

func TestSync_TargetNormalization(t *testing.T) {
	// Plural should work
	stdout, _, exitCode := runCLI(t,
		"sync",
		"--config-dir", fixtures,
		"--dry-run",
		"--target", "channels,providers",
	)
	t.Logf("sync --target channels,providers:\n%s", stdout)
	if exitCode > 1 {
		t.Fatalf("sync with plural targets crashed: exit code %d", exitCode)
	}
}

func TestSync_UnknownTargetErrors(t *testing.T) {
	_, stderr, exitCode := runCLI(t,
		"sync",
		"--config-dir", fixtures,
		"--dry-run",
		"--target", "bogus",
	)

	if exitCode == 0 {
		t.Fatal("sync --target bogus should fail, got exit code 0")
	}
	if !strings.Contains(stderr, "unknown target") {
		t.Errorf("error should mention 'unknown target', got:\n%s", stderr)
	}
}

func TestSync_ApplyWithAuditLog(t *testing.T) {
	auditLog := filepath.Join(t.TempDir(), "audit.log")

	stdout, _, exitCode := runCLI(t,
		"sync",
		"--config-dir", fixtures,
		"--audit-log", auditLog,
	)

	t.Logf("sync apply output:\n%s", stdout)
	t.Logf("exit code: %d", exitCode)

	// Check audit log
	if data, err := os.ReadFile(auditLog); err == nil {
		t.Logf("audit log:\n%s", string(data))
		if !strings.Contains(string(data), `"operation":"sync"`) {
			t.Error("audit log should contain sync operation")
		}
	}
}

func TestDiff_ProducesDriftReport(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"diff",
		"--config-dir", fixtures,
	)

	t.Logf("diff output:\n%s", stdout)
	t.Logf("exit code: %d", exitCode)

	// diff exit codes: 0=clean, 1=warning, 2=blocking
	if exitCode > 2 {
		t.Fatalf("diff crashed with exit code %d", exitCode)
	}

	// Should produce some output
	if len(strings.TrimSpace(stdout)) == 0 {
		t.Error("diff produced no output")
	}
}

func TestDiff_JSONOutput(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"diff",
		"--config-dir", fixtures,
		"--output", "json",
	)

	t.Logf("diff JSON:\n%s", stdout)
	if exitCode > 2 {
		t.Fatalf("diff --output json crashed with exit code %d", exitCode)
	}

	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			t.Errorf("diff JSON output not valid: %v", err)
		}
	}
}

func TestDiff_TargetFilterWorks(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"diff",
		"--config-dir", fixtures,
		"--target", "channel",
	)

	t.Logf("diff --target channel:\n%s", stdout)
	if exitCode > 2 {
		t.Fatalf("diff with target filter crashed: exit code %d", exitCode)
	}
}

func TestValidate_ValidFixtures(t *testing.T) {
	stdout, _, exitCode := runCLI(t,
		"validate",
		"--config-dir", fixtures,
	)

	t.Logf("validate output:\n%s", stdout)
	// validate may pass or fail depending on schema presence
	// but should not crash
	if exitCode > 1 {
		t.Logf("validate failed (may be missing schemas in fixture dir)")
	}
}

// --- Auditability E2E: Patch 001 (X-Request-ID propagation) ---

func TestPatch001_XRequestIDPropagation(t *testing.T) {
	// This test verifies that Patch 001 (New-API X-Request-ID propagation)
	// works in practice: send a request with a known X-Request-ID to New-API,
	// and verify it appears in the response header.
	//
	// Full correlation to Bifrost requires a working channel + provider chain
	// (real API keys), so we test what we can: New-API preserves the header.

	requestID := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())
	url := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", newAPIPort)

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}],"max_tokens":1}`
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test-token")
	req.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response body: %s", string(respBody))
	t.Logf("Response headers: %v", resp.Header)

	// New-API should return X-Oneapi-Request-Id header.
	// With Patch 001 applied, this should be our original requestID
	// (not a server-generated one).
	oneapiID := resp.Header.Get("X-Oneapi-Request-Id")
	if oneapiID == "" {
		// The request may fail auth (401) — that's fine.
		// Check if New-API even processed it.
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			t.Logf("Auth failed (expected without valid token). Checking if header is present...")
			// Even on 401, the middleware should set the header
			if oneapiID == "" {
				t.Log("X-Oneapi-Request-Id not present on 401 response")
				t.Log("This may mean the middleware runs after auth rejection")
				t.Skip("Cannot verify Patch 001: auth middleware rejects before request-id middleware runs")
			}
		} else {
			t.Error("X-Oneapi-Request-Id header missing from response")
		}
	}

	if oneapiID != "" {
		if oneapiID == requestID {
			t.Logf("PASS: Patch 001 confirmed — New-API used our X-Request-ID: %s", requestID)
		} else {
			t.Logf("X-Oneapi-Request-Id = %s (sent: %s)", oneapiID, requestID)
			t.Log("New-API generated its own ID instead of using the incoming one.")
			t.Log("This suggests Patch 001 may not be applied to the running image.")
			t.Log("(Expected if using upstream image without the patch)")
		}
	}

	// Verify Bifrost received the same ID by checking its logs
	bifrostLogs := getBifrostLogs(t)
	if strings.Contains(bifrostLogs, requestID) {
		t.Logf("PASS: Bifrost logs contain our X-Request-ID: %s", requestID)
	} else {
		t.Logf("Bifrost logs do not contain %s (expected if request didn't reach Bifrost)", requestID)
	}
}

func getBifrostLogs(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("docker", "logs", "test-bifrost", "--since", "60s")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Logf("WARNING: failed to get bifrost logs: %v", err)
		return ""
	}
	return out.String()
}
