package drift

import "time"

// DriftSeverity classifies how critical a detected drift is.
type DriftSeverity string

const (
	SeverityInformational DriftSeverity = "informational"
	SeverityWarning       DriftSeverity = "warning"
	SeverityBlocking      DriftSeverity = "blocking"
)

// severityOrder maps severity to numeric rank for comparison.
var severityOrder = map[DriftSeverity]int{
	SeverityInformational: 0,
	SeverityWarning:       1,
	SeverityBlocking:      2,
}

// MaxSeverity returns the higher of two severities.
func MaxSeverity(a, b DriftSeverity) DriftSeverity {
	if severityOrder[a] >= severityOrder[b] {
		return a
	}
	return b
}

// Verdict summarizes the overall drift status of a report.
type Verdict string

const (
	VerdictClean    Verdict = "clean"
	VerdictWarning  Verdict = "warning"
	VerdictBlocking Verdict = "blocking"
)

// DriftEntry represents a single field-level drift observation.
type DriftEntry struct {
	ResourceType string        // "channel", "provider", "routing-rule", "config", "pricing", "cookie"
	ResourceID   string        // identifier for the specific resource
	ActionType   string        // "create", "update", "delete"
	Field        string        // field path that drifted (e.g. "base_url", "Keys.0.Models")
	DesiredValue string        // stringified desired value
	LiveValue    string        // stringified live value
	Severity     DriftSeverity // classified severity
}

// DriftReport is the result of running drift detection against a SyncReport.
type DriftReport struct {
	Entries        []DriftEntry // individual drift entries
	Summary        DriftSummary // aggregated summary
	Timestamp      time.Time    // when detection ran
	DesiredStateDir string      // path to policy dir for context
}

// DriftSummary aggregates drift counts and determines overall verdict.
type DriftSummary struct {
	TotalResources      int     // total unique resources examined
	DriftedResources    int     // unique resources with at least one drift entry
	InformationalCount  int     // entries at informational severity
	WarningCount        int     // entries at warning severity
	BlockingCount       int     // entries at blocking severity
	Verdict             Verdict // overall verdict (highest severity or clean)
}
