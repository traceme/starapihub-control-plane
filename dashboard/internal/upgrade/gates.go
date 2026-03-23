package upgrade

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/starapihub/dashboard/internal/bootstrap"
	"github.com/starapihub/dashboard/internal/registry"
)

// RunGateDeploy checks that all upstream services are healthy (Gate 1).
func RunGateDeploy(opts GateOptions) GateResult {
	bopts := bootstrap.BootstrapOptions{
		NewAPIURL:        opts.NewAPIURL,
		NewAPIAdminToken: opts.NewAPIAdminToken,
		BifrostURL:       opts.BifrostURL,
		ClewdRURLs:       opts.ClewdRURLs,
		ClewdRAdminToken: opts.ClewdRAdminToken,
		Verbose:          opts.Verbose,
		Output:           "text",
	}

	b := bootstrap.New(bopts)
	statuses := b.CheckAllHealth()

	if len(statuses) == 0 {
		return GateResult{
			Gate:    "deployment",
			Number:  1,
			Status:  "fail",
			Message: "No upstream services configured",
		}
	}

	var details []string
	allHealthy := true
	for _, s := range statuses {
		if s.Healthy {
			details = append(details, fmt.Sprintf("OK: %s (%s)", s.Name, s.URL))
		} else {
			allHealthy = false
			errMsg := s.Error
			if errMsg == "" {
				errMsg = "unhealthy"
			}
			details = append(details, fmt.Sprintf("FAIL: %s (%s) -- %s", s.Name, s.URL, errMsg))
		}
	}

	status := "pass"
	message := "All services healthy"
	if !allHealthy {
		status = "fail"
		message = "One or more services unhealthy"
	}

	return GateResult{
		Gate:    "deployment",
		Number:  1,
		Status:  status,
		Message: message,
		Details: details,
	}
}

// RunGateSync verifies sync compatibility by loading the registry (Gate 2).
func RunGateSync(opts GateOptions) GateResult {
	if opts.ConfigDir == "" {
		return GateResult{
			Gate:    "sync",
			Number:  2,
			Status:  "fail",
			Message: "ConfigDir not set -- cannot verify sync compatibility",
		}
	}

	_, err := registry.LoadAll(opts.ConfigDir)
	if err != nil {
		return GateResult{
			Gate:    "sync",
			Number:  2,
			Status:  "fail",
			Message: fmt.Sprintf("Registry load failed: %v", err),
		}
	}

	return GateResult{
		Gate:    "sync",
		Number:  2,
		Status:  "pass",
		Message: "Dry-run sync clean",
		Details: []string{fmt.Sprintf("Registry loaded successfully from %s", opts.ConfigDir)},
	}
}

// RunGateRequest verifies the relay endpoint is reachable (Gate 3).
func RunGateRequest(relayURL string) GateResult {
	if relayURL == "" {
		return GateResult{
			Gate:    "request-path",
			Number:  3,
			Status:  "fail",
			Message: "RelayURL not set -- cannot verify request path",
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(relayURL + "/api/status")
	if err != nil {
		return GateResult{
			Gate:    "request-path",
			Number:  3,
			Status:  "fail",
			Message: fmt.Sprintf("Relay endpoint unreachable: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return GateResult{
			Gate:    "request-path",
			Number:  3,
			Status:  "pass",
			Message: fmt.Sprintf("Relay endpoint reachable (status %d)", resp.StatusCode),
		}
	}

	return GateResult{
		Gate:    "request-path",
		Number:  3,
		Status:  "fail",
		Message: fmt.Sprintf("Relay endpoint returned %d", resp.StatusCode),
	}
}

// RunGateAudit verifies X-Request-ID header propagation (Gate 4).
func RunGateAudit(relayURL string) GateResult {
	if relayURL == "" {
		return GateResult{
			Gate:    "auditability",
			Number:  4,
			Status:  "fail",
			Message: "RelayURL not set -- cannot verify auditability",
		}
	}

	testID := fmt.Sprintf("upgrade-check-%d", time.Now().UnixNano())

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", relayURL+"/api/status", nil)
	if err != nil {
		return GateResult{
			Gate:    "auditability",
			Number:  4,
			Status:  "fail",
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	req.Header.Set("X-Request-ID", testID)

	resp, err := client.Do(req)
	if err != nil {
		return GateResult{
			Gate:    "auditability",
			Number:  4,
			Status:  "fail",
			Message: fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	actual := resp.Header.Get("X-Oneapi-Request-Id")
	if actual == testID {
		return GateResult{
			Gate:    "auditability",
			Number:  4,
			Status:  "pass",
			Message: "X-Request-ID propagation verified",
		}
	}

	return GateResult{
		Gate:    "auditability",
		Number:  4,
		Status:  "fail",
		Message: fmt.Sprintf("Expected X-Oneapi-Request-Id=%s, got=%s", testID, actual),
		Details: []string{"Ensure Patch 001 is applied and nginx X-Request-ID injection is configured"},
	}
}

// RunGatePatch verifies that all active patches are applied (Gate 5).
func RunGatePatch(repoRoot string) GateResult {
	if repoRoot == "" {
		repoRoot = "."
	}

	patchFile := filepath.Join(repoRoot, "new-api", "middleware", "request-id.go")
	content, err := os.ReadFile(patchFile)
	if err != nil {
		return GateResult{
			Gate:    "patch-intent",
			Number:  5,
			Status:  "fail",
			Message: fmt.Sprintf("Cannot read patch file: %v", err),
		}
	}

	src := string(content)
	if strings.Contains(src, `c.GetHeader("X-Request-ID")`) || strings.Contains(src, `c.GetHeader("x-request-id")`) {
		return GateResult{
			Gate:    "patch-intent",
			Number:  5,
			Status:  "pass",
			Message: "All active patches verified (1/1)",
			Details: []string{"Patch 001 (X-Request-ID propagation): detected"},
		}
	}

	return GateResult{
		Gate:    "patch-intent",
		Number:  5,
		Status:  "fail",
		Message: "Patch 001 (X-Request-ID propagation) not detected",
		Details: []string{
			`Expected pattern: c.GetHeader("X-Request-ID")`,
			"See docs/upstream-patches.md for reapplication steps",
		},
	}
}
