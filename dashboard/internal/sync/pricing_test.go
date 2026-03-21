package sync

import (
	"encoding/json"
	"testing"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// mockPricingClient satisfies NewAPIPricingClient for testing.
type mockPricingClient struct {
	options     []upstream.OptionEntry
	putCalls    []putOptionCall
	putErr      error
}

type putOptionCall struct {
	Key   string
	Value string
}

func (m *mockPricingClient) GetOptionsTyped(adminToken string) ([]upstream.OptionEntry, error) {
	return m.options, nil
}

func (m *mockPricingClient) PutOption(adminToken string, key string, value string) error {
	m.putCalls = append(m.putCalls, putOptionCall{Key: key, Value: value})
	if m.putErr != nil {
		return m.putErr
	}
	// Update internal state to reflect the put
	for i, o := range m.options {
		if o.Key == key {
			m.options[i].Value = value
			return nil
		}
	}
	m.options = append(m.options, upstream.OptionEntry{Key: key, Value: value})
	return nil
}

func float64Ptr(v float64) *float64 { return &v }

func TestPricingReconciler_Plan_CreatesActions(t *testing.T) {
	r := NewPricingReconciler(nil, "token")

	desired := map[string]registry.ModelPricing{
		"gpt-4": {
			ModelRatio:      float64Ptr(15),
			CompletionRatio: float64Ptr(2),
		},
		"claude-3-opus": {
			ModelRatio: float64Ptr(20),
			ModelPrice: float64Ptr(0.01),
		},
	}
	live := []upstream.OptionEntry{} // empty options

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}

	// Should have actions for ModelRatio, CompletionRatio, ModelPrice (3 ratio keys have values)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	// Count update actions
	updates := 0
	for _, a := range actions {
		if a.Type == ActionUpdate {
			updates++
		}
	}
	if updates < 3 {
		t.Errorf("expected at least 3 update actions (ModelRatio, CompletionRatio, ModelPrice), got %d", updates)
	}
}

func TestPricingReconciler_Plan_NoChanges(t *testing.T) {
	r := NewPricingReconciler(nil, "token")

	desired := map[string]registry.ModelPricing{
		"gpt-4": {ModelRatio: float64Ptr(15)},
	}
	live := []upstream.OptionEntry{
		{Key: "ModelRatio", Value: `{"gpt-4":15}`},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (values match), got %d", len(actions))
	}
}

func TestPricingReconciler_Plan_MergesExisting(t *testing.T) {
	r := NewPricingReconciler(nil, "token")

	// We manage gpt-4, but live also has claude-3 that we don't manage
	desired := map[string]registry.ModelPricing{
		"gpt-4": {ModelRatio: float64Ptr(20)}, // changed from 15
	}
	live := []upstream.OptionEntry{
		{Key: "ModelRatio", Value: `{"gpt-4":15,"claude-3":10}`},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	// Should detect gpt-4 changed from 15 to 20
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", actions[0].Type)
	}
}

func TestPricingReconciler_Apply_MergesValues(t *testing.T) {
	mock := &mockPricingClient{
		options: []upstream.OptionEntry{
			{Key: "ModelRatio", Value: `{"claude-3":10,"gpt-4":15}`},
		},
	}
	r := NewPricingReconciler(mock, "token")

	// Build desired that changes gpt-4 to 20 but should preserve claude-3
	desired := map[string]registry.ModelPricing{
		"gpt-4": {ModelRatio: float64Ptr(20)},
	}

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "pricing",
		ResourceID:   "ModelRatio",
		Desired:      desired,
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}

	// Verify the put call merged values
	if len(mock.putCalls) != 1 {
		t.Fatalf("expected 1 PutOption call, got %d", len(mock.putCalls))
	}
	call := mock.putCalls[0]
	if call.Key != "ModelRatio" {
		t.Errorf("PutOption key = %q, want ModelRatio", call.Key)
	}

	var merged map[string]float64
	if err := json.Unmarshal([]byte(call.Value), &merged); err != nil {
		t.Fatalf("PutOption value is not valid JSON: %v", err)
	}
	if merged["gpt-4"] != 20 {
		t.Errorf("merged[gpt-4] = %v, want 20", merged["gpt-4"])
	}
	if merged["claude-3"] != 10 {
		t.Errorf("merged[claude-3] = %v, want 10 (should be preserved)", merged["claude-3"])
	}
}

func TestPricingReconciler_Verify_Success(t *testing.T) {
	mock := &mockPricingClient{
		options: []upstream.OptionEntry{
			{Key: "ModelRatio", Value: `{"gpt-4":20,"claude-3":10}`},
		},
	}
	r := NewPricingReconciler(mock, "token")

	desired := map[string]registry.ModelPricing{
		"gpt-4": {ModelRatio: float64Ptr(20)},
	}

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "pricing",
		ResourceID:   "ModelRatio",
		Desired:      desired,
	}
	result := &Result{Action: action, Status: StatusOK}

	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
}

func TestPricingReconciler_Verify_DetectsDrift(t *testing.T) {
	mock := &mockPricingClient{
		options: []upstream.OptionEntry{
			{Key: "ModelRatio", Value: `{"gpt-4":15}`}, // still old value
		},
	}
	r := NewPricingReconciler(mock, "token")

	desired := map[string]registry.ModelPricing{
		"gpt-4": {ModelRatio: float64Ptr(20)},
	}

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "pricing",
		ResourceID:   "ModelRatio",
		Desired:      desired,
	}
	result := &Result{Action: action, Status: StatusOK}

	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusAppliedWithDrift {
		t.Errorf("expected StatusAppliedWithDrift, got %s", result.Status)
	}
}

func TestPricingReconciler_Name(t *testing.T) {
	r := NewPricingReconciler(nil, "token")
	if r.Name() != "pricing" {
		t.Errorf("expected name 'pricing', got %s", r.Name())
	}
}

func TestPricingReconciler_Plan_AllFourRatioKeys(t *testing.T) {
	r := NewPricingReconciler(nil, "token")

	desired := map[string]registry.ModelPricing{
		"model-a": {
			ModelRatio:      float64Ptr(10),
			ModelPrice:      float64Ptr(0.005),
			CompletionRatio: float64Ptr(1.5),
			CacheRatio:      float64Ptr(0.5),
		},
	}
	live := []upstream.OptionEntry{}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}

	// Should have exactly 4 actions, one for each ratio key
	ratioKeys := map[string]bool{}
	for _, a := range actions {
		ratioKeys[a.ResourceID] = true
	}
	for _, key := range []string{"ModelRatio", "ModelPrice", "CompletionRatio", "CacheRatio"} {
		if !ratioKeys[key] {
			t.Errorf("missing action for ratio key %s", key)
		}
	}
}
