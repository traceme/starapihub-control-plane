package drift_test

import (
	"testing"

	"github.com/starapihub/dashboard/internal/drift"
	"github.com/starapihub/dashboard/internal/sync"
)

func TestDetect_EmptyReport(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results:      []sync.Result{},
		TotalActions: 0,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(dr.Entries))
	}
	if dr.Summary.Verdict != drift.VerdictClean {
		t.Errorf("expected clean verdict, got %s", dr.Summary.Verdict)
	}
	if dr.Summary.TotalResources != 0 {
		t.Errorf("expected 0 total resources, got %d", dr.Summary.TotalResources)
	}
}

func TestDetect_NoChangeOnly(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionNoChange,
					ResourceType: "channel",
					ResourceID:   "my-channel",
				},
				Status: sync.StatusOK,
			},
			{
				Action: sync.Action{
					Type:         sync.ActionNoChange,
					ResourceType: "provider",
					ResourceID:   "my-provider",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 2,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 0 {
		t.Errorf("expected 0 entries for no-change, got %d", len(dr.Entries))
	}
	if dr.Summary.Verdict != drift.VerdictClean {
		t.Errorf("expected clean verdict, got %s", dr.Summary.Verdict)
	}
	if dr.Summary.TotalResources != 2 {
		t.Errorf("expected 2 total resources, got %d", dr.Summary.TotalResources)
	}
	if dr.Summary.DriftedResources != 0 {
		t.Errorf("expected 0 drifted resources, got %d", dr.Summary.DriftedResources)
	}
}

func TestDetect_ChannelCreate_Blocking(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionCreate,
					ResourceType: "channel",
					ResourceID:   "new-channel",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 1,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dr.Entries))
	}
	entry := dr.Entries[0]
	if entry.Severity != drift.SeverityBlocking {
		t.Errorf("expected blocking severity for channel create, got %s", entry.Severity)
	}
	if entry.Field != "(missing)" {
		t.Errorf("expected field '(missing)', got %q", entry.Field)
	}
	if entry.ActionType != "create" {
		t.Errorf("expected action type 'create', got %q", entry.ActionType)
	}
	if dr.Summary.Verdict != drift.VerdictBlocking {
		t.Errorf("expected blocking verdict, got %s", dr.Summary.Verdict)
	}
	if dr.Summary.BlockingCount != 1 {
		t.Errorf("expected 1 blocking count, got %d", dr.Summary.BlockingCount)
	}
}

func TestDetect_ProviderUpdateKeys_Blocking(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "provider",
					ResourceID:   "openai-provider",
					Diff:         "update Keys.0.Models: [gpt-4] -> [gpt-4,gpt-4o]",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 1,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dr.Entries))
	}
	entry := dr.Entries[0]
	if entry.Severity != drift.SeverityBlocking {
		t.Errorf("expected blocking severity for keys field, got %s", entry.Severity)
	}
	if entry.Field != "Keys.0.Models" {
		t.Errorf("expected field 'Keys.0.Models', got %q", entry.Field)
	}
	if entry.LiveValue != "[gpt-4]" {
		t.Errorf("expected live value '[gpt-4]', got %q", entry.LiveValue)
	}
	if entry.DesiredValue != "[gpt-4,gpt-4o]" {
		t.Errorf("expected desired value '[gpt-4,gpt-4o]', got %q", entry.DesiredValue)
	}
}

func TestDetect_ChannelUpdateWeight_Warning(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "my-channel",
					Diff:         "update Weight: 1 -> 2",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 1,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dr.Entries))
	}
	entry := dr.Entries[0]
	if entry.Severity != drift.SeverityWarning {
		t.Errorf("expected warning severity for Weight field, got %s", entry.Severity)
	}
	if entry.Field != "Weight" {
		t.Errorf("expected field 'Weight', got %q", entry.Field)
	}
	if dr.Summary.Verdict != drift.VerdictWarning {
		t.Errorf("expected warning verdict, got %s", dr.Summary.Verdict)
	}
}

func TestDetect_MixedSeverities_VerdictEscalation(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			// Informational: channel id change
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "ch-1",
					Diff:         "update Id: 1 -> 2",
				},
				Status: sync.StatusOK,
			},
			// Warning: weight change
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "ch-2",
					Diff:         "update Weight: 1 -> 5",
				},
				Status: sync.StatusOK,
			},
			// Blocking: provider create
			{
				Action: sync.Action{
					Type:         sync.ActionCreate,
					ResourceType: "provider",
					ResourceID:   "new-provider",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 3,
	}

	dr := detector.Detect(report)

	if dr.Summary.Verdict != drift.VerdictBlocking {
		t.Errorf("expected blocking verdict (highest severity), got %s", dr.Summary.Verdict)
	}
	if dr.Summary.InformationalCount != 1 {
		t.Errorf("expected 1 informational, got %d", dr.Summary.InformationalCount)
	}
	if dr.Summary.WarningCount != 1 {
		t.Errorf("expected 1 warning, got %d", dr.Summary.WarningCount)
	}
	if dr.Summary.BlockingCount != 1 {
		t.Errorf("expected 1 blocking, got %d", dr.Summary.BlockingCount)
	}
	if dr.Summary.DriftedResources != 3 {
		t.Errorf("expected 3 drifted resources, got %d", dr.Summary.DriftedResources)
	}
}

func TestFilterBySeverity(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "ch-1",
					Diff:         "update Id: 1 -> 2",
				},
			},
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "ch-2",
					Diff:         "update Weight: 1 -> 5",
				},
			},
			{
				Action: sync.Action{
					Type:         sync.ActionCreate,
					ResourceType: "provider",
					ResourceID:   "new-prov",
				},
			},
		},
		TotalActions: 3,
	}

	dr := detector.Detect(report)

	// Filter warning and above -- should exclude informational
	filtered := dr.FilterBySeverity(drift.SeverityWarning)
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries at warning+, got %d", len(filtered))
	}

	// Filter blocking only
	blocking := dr.FilterBySeverity(drift.SeverityBlocking)
	if len(blocking) != 1 {
		t.Errorf("expected 1 blocking entry, got %d", len(blocking))
	}

	// Filter informational (all entries)
	all := dr.FilterBySeverity(drift.SeverityInformational)
	if len(all) != 3 {
		t.Errorf("expected 3 entries at informational+, got %d", len(all))
	}
}

func TestDetect_CookieNoValidCookies_Blocking(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "cookie",
					ResourceID:   "clewdr-instance-1",
					Diff:         "update no_valid_cookies: false -> true",
				},
				Status: sync.StatusOK,
			},
		},
		TotalActions: 1,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dr.Entries))
	}
	entry := dr.Entries[0]
	if entry.Severity != drift.SeverityBlocking {
		t.Errorf("expected blocking severity for no_valid_cookies, got %s", entry.Severity)
	}
	if entry.Field != "no_valid_cookies" {
		t.Errorf("expected field 'no_valid_cookies', got %q", entry.Field)
	}
}

func TestDetect_HasBlockingAndHasWarning(t *testing.T) {
	detector := drift.NewDriftDetector()

	// Report with only warning
	warningReport := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "ch-1",
					Diff:         "update Weight: 1 -> 2",
				},
			},
		},
		TotalActions: 1,
	}
	dr := detector.Detect(warningReport)
	if dr.HasBlocking() {
		t.Error("expected HasBlocking() false for warning-only report")
	}
	if !dr.HasWarning() {
		t.Error("expected HasWarning() true for warning report")
	}

	// Empty report
	emptyReport := &sync.SyncReport{Results: []sync.Result{}, TotalActions: 0}
	dr2 := detector.Detect(emptyReport)
	if dr2.HasBlocking() {
		t.Error("expected HasBlocking() false for empty report")
	}
	if dr2.HasWarning() {
		t.Error("expected HasWarning() false for empty report")
	}
}

func TestDetect_UnparseableChannelDiff(t *testing.T) {
	detector := drift.NewDriftDetector()
	report := &sync.SyncReport{
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "channel",
					ResourceID:   "my-ch",
					Diff:         "channel my-ch differs from desired state",
				},
			},
		},
		TotalActions: 1,
	}

	dr := detector.Detect(report)

	if len(dr.Entries) != 1 {
		t.Fatalf("expected 1 entry for unparseable diff, got %d", len(dr.Entries))
	}
	entry := dr.Entries[0]
	if entry.Field != "(unknown)" {
		t.Errorf("expected field '(unknown)' for unparseable diff, got %q", entry.Field)
	}
	// Default severity for channel update is warning
	if entry.Severity != drift.SeverityWarning {
		t.Errorf("expected warning severity for unparseable channel diff, got %s", entry.Severity)
	}
}
