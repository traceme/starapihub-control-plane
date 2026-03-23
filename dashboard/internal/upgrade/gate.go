package upgrade

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GateResult represents the outcome of a single verification gate.
type GateResult struct {
	Gate    string   `json:"gate"`              // "deployment", "sync", "request-path", "auditability", "patch-intent"
	Number  int      `json:"number"`            // 1-5
	Status  string   `json:"status"`            // "pass" or "fail"
	Message string   `json:"message"`           // Human-readable summary
	Details []string `json:"details,omitempty"` // Per-check details
}

// GateReport is the full upgrade-check output.
type GateReport struct {
	Gates   []GateResult `json:"gates"`
	AllPass bool         `json:"all_pass"`
	Summary string       `json:"summary"` // Copy-paste line for version matrix
}

// GateOptions holds configuration for running verification gates.
type GateOptions struct {
	NewAPIURL        string
	NewAPIAdminToken string
	BifrostURL       string
	ClewdRURLs       []string
	ClewdRAdminToken string
	ConfigDir        string
	RepoRoot         string // Path to repository root (for patch verification and version extraction)
	RelayURL         string // URL to test relay endpoint (Gate 3)
	Verbose          bool
}

// RunAllGates executes all 5 verification gates in sequence and returns a GateReport.
func RunAllGates(opts GateOptions) GateReport {
	var gates []GateResult

	gates = append(gates, RunGateDeploy(opts))
	gates = append(gates, RunGateSync(opts))
	gates = append(gates, RunGateRequest(opts.RelayURL))
	gates = append(gates, RunGateAudit(opts.RepoRoot))
	gates = append(gates, RunGatePatch(opts.RepoRoot))

	allPass := true
	for _, g := range gates {
		if g.Status != "pass" {
			allPass = false
			break
		}
	}

	// Build summary line for version matrix
	newAPIVersion := getComponentVersion(opts.RepoRoot, "new-api")
	bifrostVersion := getComponentVersion(opts.RepoRoot, "bifrost")
	clewdrVersion := getComponentVersion(opts.RepoRoot, "clewdr")

	statusText := "upgrade-check passed"
	if !allPass {
		statusText = "upgrade-check FAILED"
	}

	summary := fmt.Sprintf("| `current` | current | `%s` | `%s` | `%s` | 1 | %s | %s |",
		newAPIVersion, bifrostVersion, clewdrVersion, statusText, time.Now().Format("2006-01-02"))

	return GateReport{
		Gates:   gates,
		AllPass: allPass,
		Summary: summary,
	}
}

// getComponentVersion runs git describe on a submodule directory to get its version.
func getComponentVersion(repoRoot, submodule string) string {
	if repoRoot == "" {
		repoRoot = "."
	}
	dir := repoRoot + "/" + submodule

	cmd := exec.Command("git", "-C", dir, "describe", "--tags", "--always")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
