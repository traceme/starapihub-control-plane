package sync

import (
	"testing"
)

// Compile-time check: a mock can satisfy Reconciler interface
type mockReconciler struct{}

func (m *mockReconciler) Name() string                          { return "mock" }
func (m *mockReconciler) Plan(desired, live any) ([]Action, error) { return nil, nil }
func (m *mockReconciler) Apply(action Action) (*Result, error)  { return nil, nil }
func (m *mockReconciler) Verify(action Action, result *Result) error { return nil }

var _ Reconciler = (*mockReconciler)(nil)

func TestActionTypeConstants(t *testing.T) {
	tests := []struct {
		got  ActionType
		want string
	}{
		{ActionCreate, "create"},
		{ActionUpdate, "update"},
		{ActionDelete, "delete"},
		{ActionNoChange, "no-change"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ActionType %q != %q", tt.got, tt.want)
		}
	}
}

func TestResultStatusConstants(t *testing.T) {
	tests := []struct {
		got  ResultStatus
		want string
	}{
		{StatusOK, "ok"},
		{StatusAppliedWithDrift, "applied-with-drift"},
		{StatusUnverified, "unverified"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ResultStatus %q != %q", tt.got, tt.want)
		}
	}
}

func TestActionStructFields(t *testing.T) {
	a := Action{
		Type:         ActionCreate,
		ResourceType: "channel",
		ResourceID:   "ch-1",
		Desired:      "desired-val",
		Live:         nil,
		Diff:         "added channel ch-1",
	}
	if a.Type != ActionCreate {
		t.Error("wrong Type")
	}
	if a.ResourceType != "channel" {
		t.Error("wrong ResourceType")
	}
	if a.ResourceID != "ch-1" {
		t.Error("wrong ResourceID")
	}
	if a.Desired != "desired-val" {
		t.Error("wrong Desired")
	}
	if a.Live != nil {
		t.Error("wrong Live")
	}
	if a.Diff != "added channel ch-1" {
		t.Error("wrong Diff")
	}
}

func TestResultStructFields(t *testing.T) {
	r := Result{
		Action: Action{
			Type:         ActionUpdate,
			ResourceType: "provider",
			ResourceID:   "openai",
		},
		Status:   StatusAppliedWithDrift,
		Error:    nil,
		ReadBack: "readback-val",
		DriftMsg: "weight changed",
	}
	if r.Status != StatusAppliedWithDrift {
		t.Error("wrong Status")
	}
	if r.ReadBack != "readback-val" {
		t.Error("wrong ReadBack")
	}
	if r.DriftMsg != "weight changed" {
		t.Error("wrong DriftMsg")
	}
}

func TestSyncReportStructFields(t *testing.T) {
	sr := SyncReport{
		Results:       []Result{},
		TotalActions:  10,
		Succeeded:     7,
		Failed:        1,
		DriftWarnings: 2,
		Skipped:       0,
	}
	if sr.TotalActions != 10 {
		t.Error("wrong TotalActions")
	}
	if sr.Succeeded != 7 {
		t.Error("wrong Succeeded")
	}
	if sr.Failed != 1 {
		t.Error("wrong Failed")
	}
	if sr.DriftWarnings != 2 {
		t.Error("wrong DriftWarnings")
	}
}
