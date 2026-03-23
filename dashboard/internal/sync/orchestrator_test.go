package sync

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- Mock Reconciler for orchestrator tests ---

type orchMockReconciler struct {
	name        string
	planResult  []Action
	planErr     error
	applyFn     func(Action) (*Result, error)
	verifyFn    func(Action, *Result) error
	planCalls   int
	applyCalls  int
	verifyCalls int
}

func (m *orchMockReconciler) Name() string { return m.name }

func (m *orchMockReconciler) Plan(desired, live any) ([]Action, error) {
	m.planCalls++
	if m.planErr != nil {
		return nil, m.planErr
	}
	return m.planResult, nil
}

func (m *orchMockReconciler) Apply(action Action) (*Result, error) {
	m.applyCalls++
	if m.applyFn != nil {
		return m.applyFn(action)
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

func (m *orchMockReconciler) Verify(action Action, result *Result) error {
	m.verifyCalls++
	if m.verifyFn != nil {
		return m.verifyFn(action, result)
	}
	return nil
}

// --- Orchestrator Tests ---

func TestOrchestrator_RunsReconcilersInDependencyOrder(t *testing.T) {
	var callOrder []string

	makeReconciler := func(name string) *orchMockReconciler {
		return &orchMockReconciler{
			name: name,
			planResult: []Action{{
				Type:         ActionCreate,
				ResourceType: name,
				ResourceID:   name + "-1",
			}},
			applyFn: func(action Action) (*Result, error) {
				callOrder = append(callOrder, action.ResourceType)
				return &Result{Action: action, Status: StatusOK}, nil
			},
		}
	}

	reconcilers := []Reconciler{
		makeReconciler("cookie"),
		makeReconciler("provider"),
		makeReconciler("config"),
		makeReconciler("routing-rule"),
		makeReconciler("channel"),
		makeReconciler("pricing"),
	}

	orch := &SyncOrchestrator{
		reconcilers:  reconcilers,
		options:      SyncOptions{},
		liveState:    make(map[string]any),
		desiredState: make(map[string]any),
	}

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	expected := []string{"cookie", "provider", "config", "routing-rule", "channel", "pricing"}
	if len(callOrder) != len(expected) {
		t.Fatalf("expected %d reconciler calls, got %d: %v", len(expected), len(callOrder), callOrder)
	}
	for i, name := range expected {
		if callOrder[i] != name {
			t.Errorf("position %d: expected %s, got %s", i, name, callOrder[i])
		}
	}

	if report.TotalActions != 6 {
		t.Errorf("expected 6 total actions, got %d", report.TotalActions)
	}
	if report.Succeeded != 6 {
		t.Errorf("expected 6 succeeded, got %d", report.Succeeded)
	}
}

func TestOrchestrator_DryRunSkipsApplyAndVerify(t *testing.T) {
	rec := &orchMockReconciler{
		name: "provider",
		planResult: []Action{
			{Type: ActionCreate, ResourceType: "provider", ResourceID: "p1"},
			{Type: ActionUpdate, ResourceType: "provider", ResourceID: "p2"},
		},
	}

	orch := &SyncOrchestrator{
		reconcilers:  []Reconciler{rec},
		options:      SyncOptions{DryRun: true},
		liveState:    make(map[string]any),
		desiredState: make(map[string]any),
	}

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if rec.planCalls != 1 {
		t.Errorf("expected 1 Plan call, got %d", rec.planCalls)
	}
	if rec.applyCalls != 0 {
		t.Errorf("expected 0 Apply calls, got %d", rec.applyCalls)
	}
	if rec.verifyCalls != 0 {
		t.Errorf("expected 0 Verify calls, got %d", rec.verifyCalls)
	}

	if report.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", report.Skipped)
	}
	if report.TotalActions != 2 {
		t.Errorf("expected 2 total actions, got %d", report.TotalActions)
	}
}

func TestOrchestrator_FailFastStopsOnFirstFailure(t *testing.T) {
	callCount := 0
	rec := &orchMockReconciler{
		name: "channel",
		planResult: []Action{
			{Type: ActionCreate, ResourceType: "channel", ResourceID: "c1"},
			{Type: ActionCreate, ResourceType: "channel", ResourceID: "c2"},
		},
		applyFn: func(action Action) (*Result, error) {
			callCount++
			if action.ResourceID == "c1" {
				return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("create failed")}, nil
			}
			return &Result{Action: action, Status: StatusOK}, nil
		},
	}

	orch := &SyncOrchestrator{
		reconcilers:  []Reconciler{rec},
		options:      SyncOptions{FailFast: true},
		liveState:    make(map[string]any),
		desiredState: make(map[string]any),
	}

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 apply call (fail-fast), got %d", callCount)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
}

func TestOrchestrator_WithoutFailFast_ContinuesThroughFailures(t *testing.T) {
	callCount := 0
	rec := &orchMockReconciler{
		name: "channel",
		planResult: []Action{
			{Type: ActionCreate, ResourceType: "channel", ResourceID: "c1"},
			{Type: ActionCreate, ResourceType: "channel", ResourceID: "c2"},
		},
		applyFn: func(action Action) (*Result, error) {
			callCount++
			if action.ResourceID == "c1" {
				return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("create failed")}, nil
			}
			return &Result{Action: action, Status: StatusOK}, nil
		},
	}

	orch := &SyncOrchestrator{
		reconcilers:  []Reconciler{rec},
		options:      SyncOptions{FailFast: false},
		liveState:    make(map[string]any),
		desiredState: make(map[string]any),
	}

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 apply calls (continue through failure), got %d", callCount)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", report.Succeeded)
	}
}

func TestOrchestrator_TargetFiltering(t *testing.T) {
	cookie := &orchMockReconciler{name: "cookie", planResult: []Action{{Type: ActionCreate, ResourceType: "cookie", ResourceID: "c1"}}}
	channel := &orchMockReconciler{name: "channel", planResult: []Action{{Type: ActionCreate, ResourceType: "channel", ResourceID: "ch1"}}}
	pricing := &orchMockReconciler{name: "pricing", planResult: []Action{{Type: ActionUpdate, ResourceType: "pricing", ResourceID: "ModelRatio"}}}

	opts := SyncOptions{Targets: []string{"channel", "pricing"}}
	orch := NewSyncOrchestrator(
		[]Reconciler{cookie, channel, pricing},
		opts,
		make(map[string]any),
		make(map[string]any),
	)

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if cookie.planCalls != 0 {
		t.Errorf("cookie should not have been called, got %d plan calls", cookie.planCalls)
	}
	if channel.planCalls != 1 {
		t.Errorf("channel should have been called once, got %d", channel.planCalls)
	}
	if pricing.planCalls != 1 {
		t.Errorf("pricing should have been called once, got %d", pricing.planCalls)
	}
	if report.TotalActions != 2 {
		t.Errorf("expected 2 total actions, got %d", report.TotalActions)
	}
}

func TestNormalizeTargets_PluralsNormalized(t *testing.T) {
	targets, err := NormalizeTargets([]string{"channels", "providers"})
	if err != nil {
		t.Fatalf("NormalizeTargets error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "channel" {
		t.Errorf("expected channel, got %s", targets[0])
	}
	if targets[1] != "provider" {
		t.Errorf("expected provider, got %s", targets[1])
	}
}

func TestNormalizeTargets_SingularsPassThrough(t *testing.T) {
	targets, err := NormalizeTargets([]string{"channel", "config", "routing-rule"})
	if err != nil {
		t.Fatalf("NormalizeTargets error: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
}

func TestNormalizeTargets_UnknownTargetErrors(t *testing.T) {
	_, err := NormalizeTargets([]string{"channel", "bogus", "fakething"})
	if err == nil {
		t.Fatal("expected error for unknown targets")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus': %v", err)
	}
	if !strings.Contains(err.Error(), "fakething") {
		t.Errorf("error should mention 'fakething': %v", err)
	}
	if !strings.Contains(err.Error(), "valid:") {
		t.Errorf("error should list valid targets: %v", err)
	}
}

func TestNormalizeTargets_EmptyReturnsNil(t *testing.T) {
	targets, err := NormalizeTargets(nil)
	if err != nil {
		t.Fatalf("NormalizeTargets error: %v", err)
	}
	if targets != nil {
		t.Errorf("expected nil, got %v", targets)
	}
}

func TestNormalizeTargets_DeduplicatesPluralsAndSingulars(t *testing.T) {
	targets, err := NormalizeTargets([]string{"channel", "channels"})
	if err != nil {
		t.Fatalf("NormalizeTargets error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (deduped), got %d: %v", len(targets), targets)
	}
	if targets[0] != "channel" {
		t.Errorf("expected channel, got %s", targets[0])
	}
}

func TestOrchestrator_ReportCountsCorrect(t *testing.T) {
	rec := &orchMockReconciler{
		name: "provider",
		planResult: []Action{
			{Type: ActionCreate, ResourceType: "provider", ResourceID: "p1"},
			{Type: ActionUpdate, ResourceType: "provider", ResourceID: "p2"},
			{Type: ActionDelete, ResourceType: "provider", ResourceID: "p3"},
		},
		applyFn: func(action Action) (*Result, error) {
			if action.ResourceID == "p2" {
				return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("update failed")}, nil
			}
			return &Result{Action: action, Status: StatusOK}, nil
		},
		verifyFn: func(action Action, result *Result) error {
			if action.ResourceID == "p3" {
				result.Status = StatusAppliedWithDrift
				result.DriftMsg = "still present"
			}
			return nil
		},
	}

	orch := &SyncOrchestrator{
		reconcilers:  []Reconciler{rec},
		options:      SyncOptions{},
		liveState:    make(map[string]any),
		desiredState: make(map[string]any),
	}

	report, err := orch.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if report.TotalActions != 3 {
		t.Errorf("expected 3 total, got %d", report.TotalActions)
	}
	if report.Succeeded != 1 {
		t.Errorf("expected 1 succeeded, got %d", report.Succeeded)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.DriftWarnings != 1 {
		t.Errorf("expected 1 drift warning, got %d", report.DriftWarnings)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		report   *SyncReport
		expected int
	}{
		{"all succeeded", &SyncReport{TotalActions: 3, Succeeded: 3, Failed: 0}, 0},
		{"some failed", &SyncReport{TotalActions: 3, Succeeded: 2, Failed: 1}, 1},
		{"no actions", &SyncReport{TotalActions: 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := ExitCode(tt.report)
			if code != tt.expected {
				t.Errorf("ExitCode() = %d, want %d", code, tt.expected)
			}
		})
	}
}

// --- Report Tests ---

func TestFormatTextReport_ShowsSummaryAndTable(t *testing.T) {
	report := &SyncReport{
		Results: []Result{
			{Action: Action{Type: ActionCreate, ResourceType: "channel", ResourceID: "ch1"}, Status: StatusOK},
			{Action: Action{Type: ActionUpdate, ResourceType: "provider", ResourceID: "p1", Diff: "weight changed"}, Status: StatusAppliedWithDrift, DriftMsg: "drift detected"},
		},
		TotalActions:  2,
		Succeeded:     1,
		Failed:        0,
		DriftWarnings: 1,
		Skipped:       0,
	}

	text := FormatTextReport(report, false)

	if !strings.Contains(text, "1 succeeded") {
		t.Errorf("should contain succeeded count: %s", text)
	}
	if !strings.Contains(text, "1 drift") {
		t.Errorf("should contain drift count: %s", text)
	}
	if !strings.Contains(text, "channel") {
		t.Errorf("should contain resource type: %s", text)
	}
	if !strings.Contains(text, "ch1") {
		t.Errorf("should contain resource ID: %s", text)
	}
}

func TestFormatTextReport_VerboseShowsDiff(t *testing.T) {
	report := &SyncReport{
		Results: []Result{
			{Action: Action{Type: ActionUpdate, ResourceType: "provider", ResourceID: "p1", Diff: "weight: 1.0 -> 2.0"}, Status: StatusOK},
		},
		TotalActions: 1,
		Succeeded:    1,
	}

	text := FormatTextReport(report, true)
	if !strings.Contains(text, "weight: 1.0 -> 2.0") {
		t.Errorf("verbose report should contain diff: %s", text)
	}
}

func TestFormatTextReport_NoChanges(t *testing.T) {
	report := &SyncReport{TotalActions: 0}
	text := FormatTextReport(report, false)
	if !strings.Contains(text, "No changes needed") {
		t.Errorf("should say no changes needed: %s", text)
	}
}

func TestFormatJSONReport_ValidJSON(t *testing.T) {
	report := &SyncReport{
		Results: []Result{
			{Action: Action{Type: ActionCreate, ResourceType: "channel", ResourceID: "ch1"}, Status: StatusOK},
		},
		TotalActions: 1,
		Succeeded:    1,
	}

	data, err := FormatJSONReport(report)
	if err != nil {
		t.Fatalf("FormatJSONReport error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["total_actions"].(float64) != 1 {
		t.Errorf("expected total_actions=1, got %v", parsed["total_actions"])
	}
	if parsed["succeeded"].(float64) != 1 {
		t.Errorf("expected succeeded=1, got %v", parsed["succeeded"])
	}

	results, ok := parsed["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected 1 result in array")
	}
	r := results[0].(map[string]any)
	if r["resource_type"] != "channel" {
		t.Errorf("expected resource_type=channel, got %v", r["resource_type"])
	}
}
