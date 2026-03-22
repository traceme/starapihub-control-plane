package drift_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/starapihub/dashboard/internal/drift"
)

// helper to build a DriftReport with preset values.
func makeReport(entries []drift.DriftEntry, total, drifted int) *drift.DriftReport {
	summary := drift.DriftSummary{
		TotalResources:   total,
		DriftedResources: drifted,
		Verdict:          drift.VerdictClean,
	}
	for _, e := range entries {
		switch e.Severity {
		case drift.SeverityInformational:
			summary.InformationalCount++
		case drift.SeverityWarning:
			summary.WarningCount++
		case drift.SeverityBlocking:
			summary.BlockingCount++
		}
	}
	if summary.BlockingCount > 0 {
		summary.Verdict = drift.VerdictBlocking
	} else if summary.WarningCount > 0 {
		summary.Verdict = drift.VerdictWarning
	}

	return &drift.DriftReport{
		Entries:         entries,
		Summary:         summary,
		Timestamp:       time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
		DesiredStateDir: "/tmp/test-policies",
	}
}

func TestFormatTextDriftReport_Clean(t *testing.T) {
	report := makeReport(nil, 5, 0)
	result := drift.FormatTextDriftReport(report, false)
	if !strings.Contains(result, "No drift detected") {
		t.Errorf("expected 'No drift detected' in output, got: %s", result)
	}
}

func TestFormatTextDriftReport_BlockingNonVerbose(t *testing.T) {
	entries := []drift.DriftEntry{
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "base_url",
			DesiredValue: "http://new",
			LiveValue:    "http://old",
			Severity:     drift.SeverityBlocking,
		},
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "created_time",
			DesiredValue: "2026-01-01",
			LiveValue:    "2025-12-01",
			Severity:     drift.SeverityInformational,
		},
	}
	report := makeReport(entries, 5, 1)
	result := drift.FormatTextDriftReport(report, false)

	if !strings.Contains(result, "BLOCKING") {
		t.Errorf("expected 'BLOCKING' in output, got: %s", result)
	}
	// Informational entries should be hidden when verbose=false
	if strings.Contains(result, "created_time") {
		t.Errorf("informational entry 'created_time' should be hidden when verbose=false, got: %s", result)
	}
}

func TestFormatTextDriftReport_MixedVerbose(t *testing.T) {
	entries := []drift.DriftEntry{
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "base_url",
			DesiredValue: "http://new",
			LiveValue:    "http://old",
			Severity:     drift.SeverityBlocking,
		},
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "created_time",
			DesiredValue: "2026-01-01",
			LiveValue:    "2025-12-01",
			Severity:     drift.SeverityInformational,
		},
		{
			ResourceType: "provider",
			ResourceID:   "prov-1",
			ActionType:   "update",
			Field:        "network_config",
			DesiredValue: "timeout=30",
			LiveValue:    "timeout=10",
			Severity:     drift.SeverityWarning,
		},
	}
	report := makeReport(entries, 5, 2)
	result := drift.FormatTextDriftReport(report, true)

	if !strings.Contains(result, "INFORMATIONAL") {
		t.Errorf("expected 'INFORMATIONAL' in verbose output, got: %s", result)
	}
	if !strings.Contains(result, "BLOCKING") {
		t.Errorf("expected 'BLOCKING' in verbose output, got: %s", result)
	}
	if !strings.Contains(result, "WARNING") {
		t.Errorf("expected 'WARNING' in verbose output, got: %s", result)
	}
}

func TestFormatJSONDriftReport_BlockingVerdict(t *testing.T) {
	entries := []drift.DriftEntry{
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "base_url",
			DesiredValue: "http://new",
			LiveValue:    "http://old",
			Severity:     drift.SeverityBlocking,
		},
	}
	report := makeReport(entries, 5, 1)
	data, err := drift.FormatJSONDriftReport(report, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !json.Valid(data) {
		t.Errorf("output is not valid JSON: %s", string(data))
	}
	if !strings.Contains(string(data), `"verdict":"blocking"`) && !strings.Contains(string(data), `"verdict": "blocking"`) {
		t.Errorf("expected verdict=blocking in JSON, got: %s", string(data))
	}
}

func TestFormatJSONDriftReport_NonVerboseExcludesInformational(t *testing.T) {
	entries := []drift.DriftEntry{
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "base_url",
			DesiredValue: "http://new",
			LiveValue:    "http://old",
			Severity:     drift.SeverityBlocking,
		},
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "created_time",
			DesiredValue: "2026-01-01",
			LiveValue:    "2025-12-01",
			Severity:     drift.SeverityInformational,
		},
	}
	report := makeReport(entries, 5, 1)

	data, err := drift.FormatJSONDriftReport(report, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed struct {
		Entries []struct {
			Severity string `json:"severity"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	for _, e := range parsed.Entries {
		if e.Severity == "informational" {
			t.Errorf("informational entry should be excluded when verbose=false")
		}
	}
	if len(parsed.Entries) != 1 {
		t.Errorf("expected 1 entry (blocking only), got %d", len(parsed.Entries))
	}
}

func TestFormatJSONDriftReport_VerboseIncludesInformational(t *testing.T) {
	entries := []drift.DriftEntry{
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "base_url",
			DesiredValue: "http://new",
			LiveValue:    "http://old",
			Severity:     drift.SeverityBlocking,
		},
		{
			ResourceType: "channel",
			ResourceID:   "chan-1",
			ActionType:   "update",
			Field:        "created_time",
			DesiredValue: "2026-01-01",
			LiveValue:    "2025-12-01",
			Severity:     drift.SeverityInformational,
		},
	}
	report := makeReport(entries, 5, 1)

	data, err := drift.FormatJSONDriftReport(report, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed struct {
		Entries []struct {
			Severity string `json:"severity"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed.Entries) != 2 {
		t.Errorf("expected 2 entries (all included in verbose), got %d", len(parsed.Entries))
	}
}

func TestFormatTextDriftReport_SummaryCountsCorrect(t *testing.T) {
	entries := []drift.DriftEntry{
		{ResourceType: "channel", ResourceID: "c1", ActionType: "update", Field: "base_url", Severity: drift.SeverityBlocking},
		{ResourceType: "channel", ResourceID: "c1", ActionType: "update", Field: "key", Severity: drift.SeverityBlocking},
		{ResourceType: "provider", ResourceID: "p1", ActionType: "update", Field: "network_config", Severity: drift.SeverityWarning},
		{ResourceType: "provider", ResourceID: "p1", ActionType: "update", Field: "description", Severity: drift.SeverityInformational},
	}
	report := makeReport(entries, 5, 2)
	result := drift.FormatTextDriftReport(report, true)

	if !strings.Contains(result, "2 blocking") {
		t.Errorf("expected '2 blocking' in summary, got: %s", result)
	}
	if !strings.Contains(result, "1 warning") {
		t.Errorf("expected '1 warning' in summary, got: %s", result)
	}
	if !strings.Contains(result, "1 informational") {
		t.Errorf("expected '1 informational' in summary, got: %s", result)
	}
}
