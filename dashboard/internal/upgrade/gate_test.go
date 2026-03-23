package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatePatchPresent(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "new-api", "middleware")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package middleware

import "github.com/gin-gonic/gin"

func RequestId() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = "generated-id"
		}
		c.Next()
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "request-id.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := RunGatePatch(tmpDir)
	if result.Status != "pass" {
		t.Errorf("expected pass, got %s: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "1/1") {
		t.Errorf("expected message to contain '1/1', got: %s", result.Message)
	}
}

func TestGatePatchMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "new-api", "middleware")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package middleware

func RequestId() func() {
	return func() {
		id := common.GetTimeString() + common.GetRandomString(8)
		_ = id
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "request-id.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := RunGatePatch(tmpDir)
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "not detected") {
		t.Errorf("expected message to contain 'not detected', got: %s", result.Message)
	}
}

func TestGatePatchFileNotFound(t *testing.T) {
	result := RunGatePatch("/nonexistent/path")
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "Cannot read") {
		t.Errorf("expected message to contain 'Cannot read', got: %s", result.Message)
	}
}

func TestGateRequestNoURL(t *testing.T) {
	result := RunGateRequest("")
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s: %s", result.Status, result.Message)
	}
}

func TestGateAuditNoURL(t *testing.T) {
	result := RunGateAudit("")
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s: %s", result.Status, result.Message)
	}
}

func TestGateSyncNoConfigDir(t *testing.T) {
	result := RunGateSync(GateOptions{})
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "ConfigDir not set") {
		t.Errorf("expected message to contain 'ConfigDir not set', got: %s", result.Message)
	}
}

func TestFormatTextAllPass(t *testing.T) {
	report := GateReport{
		Gates: []GateResult{
			{Gate: "deployment", Number: 1, Status: "pass", Message: "All services healthy"},
			{Gate: "sync", Number: 2, Status: "pass", Message: "Dry-run sync clean"},
			{Gate: "request-path", Number: 3, Status: "pass", Message: "Relay endpoint reachable (status 200)"},
			{Gate: "auditability", Number: 4, Status: "pass", Message: "X-Request-ID propagation verified"},
			{Gate: "patch-intent", Number: 5, Status: "pass", Message: "All active patches verified (1/1)"},
		},
		AllPass: true,
		Summary: "| `current` | current | `v1` | `v2` | `v3` | 1 | upgrade-check passed | 2026-03-22 |",
	}

	output := FormatText(report, false)
	passCount := strings.Count(output, "PASS")
	// 5 gate lines + 1 result line = 6 PASS occurrences
	if passCount < 5 {
		t.Errorf("expected at least 5 PASS occurrences, got %d in:\n%s", passCount, output)
	}
	if !strings.Contains(output, "Result: PASS") {
		t.Errorf("expected 'Result: PASS' in output:\n%s", output)
	}
}

func TestFormatTextSomeFail(t *testing.T) {
	report := GateReport{
		Gates: []GateResult{
			{Gate: "deployment", Number: 1, Status: "pass", Message: "All services healthy"},
			{Gate: "sync", Number: 2, Status: "pass", Message: "Dry-run sync clean"},
			{Gate: "request-path", Number: 3, Status: "fail", Message: "Relay endpoint unreachable"},
			{Gate: "auditability", Number: 4, Status: "pass", Message: "X-Request-ID propagation verified"},
			{Gate: "patch-intent", Number: 5, Status: "pass", Message: "All active patches verified (1/1)"},
		},
		AllPass: false,
		Summary: "| `current` | current | `v1` | `v2` | `v3` | 1 | upgrade-check FAILED | 2026-03-22 |",
	}

	output := FormatText(report, false)
	if !strings.Contains(output, "FAIL") {
		t.Errorf("expected FAIL in output:\n%s", output)
	}
	if !strings.Contains(output, "4/5") {
		t.Errorf("expected '4/5' in output:\n%s", output)
	}
}

func TestFormatJSON(t *testing.T) {
	report := GateReport{
		Gates: []GateResult{
			{Gate: "deployment", Number: 1, Status: "pass", Message: "All services healthy"},
			{Gate: "patch-intent", Number: 5, Status: "fail", Message: "Patch not detected"},
		},
		AllPass: false,
		Summary: "test summary",
	}

	jsonStr, err := FormatJSON(report)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var parsed GateReport
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed.AllPass != report.AllPass {
		t.Errorf("AllPass mismatch: expected %v, got %v", report.AllPass, parsed.AllPass)
	}
	if len(parsed.Gates) != len(report.Gates) {
		t.Errorf("Gates count mismatch: expected %d, got %d", len(report.Gates), len(parsed.Gates))
	}
}

func TestRunAllGatesIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "new-api", "middleware")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package middleware

func RequestId() func() {
	return func() {
		id := c.GetHeader("X-Request-ID")
		_ = id
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "request-id.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	report := RunAllGates(GateOptions{RepoRoot: tmpDir})

	if len(report.Gates) != 5 {
		t.Errorf("expected 5 gates, got %d", len(report.Gates))
	}

	// Gate 5 should pass (patch file present with correct pattern)
	if len(report.Gates) >= 5 {
		gate5 := report.Gates[4]
		if gate5.Number != 5 {
			t.Errorf("expected gate 5 at index 4, got gate %d", gate5.Number)
		}
		if gate5.Status != "pass" {
			t.Errorf("expected gate 5 to pass, got %s: %s", gate5.Status, gate5.Message)
		}
	}

	// Summary should be present
	if report.Summary == "" {
		t.Error("expected non-empty summary")
	}
}
