package sync

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatTextReport produces a human-readable text report from a SyncReport.
// If verbose is true, includes Diff and DriftMsg details.
func FormatTextReport(report *SyncReport, verbose bool) string {
	if report.TotalActions == 0 {
		return "No changes needed -- desired state matches live state"
	}

	var b strings.Builder

	// Summary line
	fmt.Fprintf(&b, "Sync complete: %d succeeded, %d failed, %d drift warnings, %d skipped\n\n",
		report.Succeeded, report.Failed, report.DriftWarnings, report.Skipped)

	// Table header
	fmt.Fprintf(&b, "%-16s %-24s %-10s %-12s %s\n",
		"RESOURCE TYPE", "RESOURCE ID", "ACTION", "STATUS", "DETAILS")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 80))

	// Table rows
	for _, r := range report.Results {
		details := ""
		if verbose {
			if r.Action.Diff != "" {
				details = r.Action.Diff
			}
			if r.DriftMsg != "" {
				if details != "" {
					details += " | "
				}
				details += r.DriftMsg
			}
		} else {
			// Non-verbose: show error message for failed, drift msg for drift
			if r.Error != nil {
				details = r.Error.Error()
			} else if r.DriftMsg != "" {
				details = r.DriftMsg
			}
		}

		fmt.Fprintf(&b, "%-16s %-24s %-10s %-12s %s\n",
			r.Action.ResourceType,
			truncate(r.Action.ResourceID, 24),
			string(r.Action.Type),
			string(r.Status),
			details,
		)
	}

	return b.String()
}

// truncate shortens a string to maxLen if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// jsonReportEntry is a single result entry in the JSON report.
type jsonReportEntry struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	Diff         string `json:"diff,omitempty"`
	DriftMsg     string `json:"drift_msg,omitempty"`
	Error        string `json:"error,omitempty"`
}

// jsonReport is the top-level JSON report structure.
type jsonReport struct {
	TotalActions  int               `json:"total_actions"`
	Succeeded     int               `json:"succeeded"`
	Failed        int               `json:"failed"`
	DriftWarnings int               `json:"drift_warnings"`
	Skipped       int               `json:"skipped"`
	Results       []jsonReportEntry `json:"results"`
}

// FormatJSONReport produces a JSON report from a SyncReport.
func FormatJSONReport(report *SyncReport) ([]byte, error) {
	jr := jsonReport{
		TotalActions:  report.TotalActions,
		Succeeded:     report.Succeeded,
		Failed:        report.Failed,
		DriftWarnings: report.DriftWarnings,
		Skipped:       report.Skipped,
		Results:       make([]jsonReportEntry, 0, len(report.Results)),
	}

	for _, r := range report.Results {
		entry := jsonReportEntry{
			ResourceType: r.Action.ResourceType,
			ResourceID:   r.Action.ResourceID,
			Action:       string(r.Action.Type),
			Status:       string(r.Status),
			Diff:         r.Action.Diff,
			DriftMsg:     r.DriftMsg,
		}
		if r.Error != nil {
			entry.Error = r.Error.Error()
		}
		jr.Results = append(jr.Results, entry)
	}

	return json.MarshalIndent(jr, "", "  ")
}
