package drift

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ANSI color constants for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"  // blocking
	colorYellow = "\033[33m"  // warning
	colorDim    = "\033[2m"   // informational
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"  // clean verdict
)

// FormatTextDriftReport produces a human-readable, color-coded drift report.
// When verbose is false, informational entries are hidden.
func FormatTextDriftReport(report *DriftReport, verbose bool) string {
	if report.Summary.Verdict == VerdictClean {
		return "No drift detected -- desired state matches live state\n"
	}

	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "Drift Report (%s)\n", report.Timestamp.Format("2006-01-02 15:04:05 UTC"))

	// Summary line
	fmt.Fprintf(&b, "%sVerdict: %s%s%s | %d blocking, %d warning, %d informational | %d resources checked, %d drifted\n\n",
		colorBold,
		verdictColor(report.Summary.Verdict), strings.ToUpper(string(report.Summary.Verdict)), colorReset,
		report.Summary.BlockingCount,
		report.Summary.WarningCount,
		report.Summary.InformationalCount,
		report.Summary.TotalResources,
		report.Summary.DriftedResources,
	)

	// Filter entries
	var entries []DriftEntry
	if verbose {
		entries = report.Entries
	} else {
		entries = report.FilterBySeverity(SeverityWarning)
	}

	// Group by resource type
	grouped := make(map[string][]DriftEntry)
	var order []string
	for _, e := range entries {
		if _, seen := grouped[e.ResourceType]; !seen {
			order = append(order, e.ResourceType)
		}
		grouped[e.ResourceType] = append(grouped[e.ResourceType], e)
	}

	for _, rt := range order {
		fmt.Fprintf(&b, "--- %s ---\n", rt)
		for _, e := range grouped[rt] {
			sColor := severityColor(e.Severity)
			sevLabel := strings.ToUpper(string(e.Severity))

			switch e.ActionType {
			case "create":
				fmt.Fprintf(&b, "%s[%s]%s %s :: (missing) -- needs to be created\n",
					sColor, sevLabel, colorReset, e.ResourceID)
			case "delete":
				fmt.Fprintf(&b, "%s[%s]%s %s :: (extra) -- not in desired state\n",
					sColor, sevLabel, colorReset, e.ResourceID)
			default:
				fmt.Fprintf(&b, "%s[%s]%s %s :: %s: %s -> %s\n",
					sColor, sevLabel, colorReset, e.ResourceID, e.Field, e.LiveValue, e.DesiredValue)
			}
		}
	}

	// Hidden informational hint
	if !verbose && report.Summary.InformationalCount > 0 {
		fmt.Fprintf(&b, "\n(%d informational differences hidden -- use --verbose to show)\n",
			report.Summary.InformationalCount)
	}

	return b.String()
}

// severityColor returns the ANSI color code for a severity level.
func severityColor(s DriftSeverity) string {
	switch s {
	case SeverityBlocking:
		return colorRed
	case SeverityWarning:
		return colorYellow
	case SeverityInformational:
		return colorDim
	default:
		return colorReset
	}
}

// verdictColor returns the ANSI color code for a verdict.
func verdictColor(v Verdict) string {
	switch v {
	case VerdictClean:
		return colorGreen
	case VerdictWarning:
		return colorYellow
	case VerdictBlocking:
		return colorRed
	default:
		return colorReset
	}
}

// JSON serialization types (unexported).

type jsonDriftEntry struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	ActionType   string `json:"action_type"`
	Field        string `json:"field"`
	DesiredValue string `json:"desired_value"`
	LiveValue    string `json:"live_value"`
	Severity     string `json:"severity"`
}

type jsonDriftSummary struct {
	TotalResources     int    `json:"total_resources"`
	DriftedResources   int    `json:"drifted_resources"`
	InformationalCount int    `json:"informational_count"`
	WarningCount       int    `json:"warning_count"`
	BlockingCount      int    `json:"blocking_count"`
	Verdict            string `json:"verdict"`
}

type jsonDriftReport struct {
	Timestamp string           `json:"timestamp"`
	Summary   jsonDriftSummary `json:"summary"`
	Entries   []jsonDriftEntry `json:"entries"`
}

// FormatJSONDriftReport produces a structured JSON drift report.
// When verbose is false, informational entries are excluded from the entries array.
// The summary always reflects the full report regardless of verbose flag.
func FormatJSONDriftReport(report *DriftReport, verbose bool) ([]byte, error) {
	// Filter entries
	var entries []DriftEntry
	if verbose {
		entries = report.Entries
	} else {
		entries = report.FilterBySeverity(SeverityWarning)
	}

	jr := jsonDriftReport{
		Timestamp: report.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		Summary: jsonDriftSummary{
			TotalResources:     report.Summary.TotalResources,
			DriftedResources:   report.Summary.DriftedResources,
			InformationalCount: report.Summary.InformationalCount,
			WarningCount:       report.Summary.WarningCount,
			BlockingCount:      report.Summary.BlockingCount,
			Verdict:            string(report.Summary.Verdict),
		},
		Entries: make([]jsonDriftEntry, 0, len(entries)),
	}

	for _, e := range entries {
		jr.Entries = append(jr.Entries, jsonDriftEntry{
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			ActionType:   e.ActionType,
			Field:        e.Field,
			DesiredValue: e.DesiredValue,
			LiveValue:    e.LiveValue,
			Severity:     string(e.Severity),
		})
	}

	return json.MarshalIndent(jr, "", "  ")
}
