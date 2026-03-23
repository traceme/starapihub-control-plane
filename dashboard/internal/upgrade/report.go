package upgrade

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatText produces a human-readable text report of the gate results.
func FormatText(report GateReport, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("Upgrade Verification Report\n")
	sb.WriteString("===========================\n\n")

	gateLabels := []string{
		"Deployment",
		"Sync",
		"Request Path",
		"Auditability",
		"Patch Intent",
	}

	passCount := 0
	for i, g := range report.Gates {
		label := fmt.Sprintf("Gate %d", g.Number)
		if i < len(gateLabels) {
			label = fmt.Sprintf("Gate %d (%s)", g.Number, gateLabels[i])
		}

		status := "PASS"
		if g.Status != "pass" {
			status = "FAIL"
		} else {
			passCount++
		}

		sb.WriteString(fmt.Sprintf("  %-28s %s  %s\n", label+":", status, g.Message))

		if verbose && len(g.Details) > 0 {
			for _, d := range g.Details {
				sb.WriteString(fmt.Sprintf("    - %s\n", d))
			}
		}
	}

	sb.WriteString("\n")

	total := len(report.Gates)
	if report.AllPass {
		sb.WriteString(fmt.Sprintf("Result: PASS (%d/%d gates passed)\n", passCount, total))
	} else {
		sb.WriteString(fmt.Sprintf("Result: FAIL (%d/%d gates passed)\n", passCount, total))
	}

	sb.WriteString("\nVersion Matrix Row:\n")
	sb.WriteString(report.Summary + "\n")

	return sb.String()
}

// FormatJSON returns the gate report as indented JSON.
func FormatJSON(report GateReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal gate report: %w", err)
	}
	return string(data), nil
}
