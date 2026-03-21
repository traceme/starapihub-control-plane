package sync

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// --- Mock client ---

type mockBifrostRoutingClient struct {
	rules          []upstream.BifrostRoutingRuleResponse
	createdPayload json.RawMessage
	updatedID      string
	updatedPayload json.RawMessage
	deletedID      string
	createErr      error
	updateErr      error
	deleteErr      error
	listErr        error
	// postActionRules is returned by ListRoutingRulesTyped after a create/update/delete
	postActionRules []upstream.BifrostRoutingRuleResponse
	acted           bool
}

func (m *mockBifrostRoutingClient) ListRoutingRulesTyped() ([]upstream.BifrostRoutingRuleResponse, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.acted && m.postActionRules != nil {
		return m.postActionRules, nil
	}
	return m.rules, nil
}

func (m *mockBifrostRoutingClient) CreateRoutingRuleTyped(rule json.RawMessage) (*upstream.BifrostRoutingRuleResponse, error) {
	m.createdPayload = rule
	m.acted = true
	if m.createErr != nil {
		return nil, m.createErr
	}
	var resp upstream.BifrostRoutingRuleResponse
	if err := json.Unmarshal(rule, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *mockBifrostRoutingClient) UpdateRoutingRuleTyped(id string, rule json.RawMessage) error {
	m.updatedID = id
	m.updatedPayload = rule
	m.acted = true
	return m.updateErr
}

func (m *mockBifrostRoutingClient) DeleteRoutingRuleTyped(id string) error {
	m.deletedID = id
	m.acted = true
	return m.deleteErr
}

// --- Compile-time check ---

var _ Reconciler = (*RoutingRuleReconciler)(nil)

// --- Plan tests ---

func TestRoutingPlan_2Desired0Live_Returns2Creates(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false)

	desired := map[string]registry.RoutingRuleDesired{
		"rule-a": {Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
		"rule-b": {Name: "Rule B", Enabled: true, CelExpression: "model == 'claude-3'", Scope: "global", Priority: 20},
	}
	live := []upstream.BifrostRoutingRuleResponse{}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	creates := 0
	for _, a := range actions {
		if a.Type == ActionCreate {
			creates++
		}
	}
	if creates != 2 {
		t.Errorf("expected 2 creates, got %d", creates)
	}
}

func TestRoutingPlan_1DesiredMatchingLive_Returns0Actions(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false)

	desired := map[string]registry.RoutingRuleDesired{
		"rule-a": {Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
	}
	live := []upstream.BifrostRoutingRuleResponse{
		{ID: "rule-a", Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, a := range actions {
		if a.Type != ActionNoChange {
			t.Errorf("expected no change, got %s for %s", a.Type, a.ResourceID)
		}
	}
}

func TestRoutingPlan_1DesiredDiffCelExpression_Returns1Update(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false)

	desired := map[string]registry.RoutingRuleDesired{
		"rule-a": {Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4o'", Scope: "global", Priority: 10},
	}
	live := []upstream.BifrostRoutingRuleResponse{
		{ID: "rule-a", Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	updates := 0
	for _, a := range actions {
		if a.Type == ActionUpdate {
			updates++
			if a.ResourceID != "rule-a" {
				t.Errorf("expected ResourceID 'rule-a', got %s", a.ResourceID)
			}
			if a.Diff == "" {
				t.Error("expected non-empty Diff for update")
			}
		}
	}
	if updates != 1 {
		t.Errorf("expected 1 update, got %d", updates)
	}
}

func TestRoutingPlan_PruneExtraLiveRule(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, true) // prune=true

	desired := map[string]registry.RoutingRuleDesired{}
	live := []upstream.BifrostRoutingRuleResponse{
		{ID: "stale-rule", Name: "Stale", Enabled: true, CelExpression: "true", Scope: "global", Priority: 1},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	deletes := 0
	for _, a := range actions {
		if a.Type == ActionDelete {
			deletes++
			if a.ResourceID != "stale-rule" {
				t.Errorf("expected delete ResourceID 'stale-rule', got %s", a.ResourceID)
			}
		}
	}
	if deletes != 1 {
		t.Errorf("expected 1 delete with prune=true, got %d", deletes)
	}
}

func TestRoutingPlan_NoPruneExtraLive(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false) // prune=false

	desired := map[string]registry.RoutingRuleDesired{}
	live := []upstream.BifrostRoutingRuleResponse{
		{ID: "stale-rule", Name: "Stale", Enabled: true, CelExpression: "true", Scope: "global", Priority: 1},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, a := range actions {
		if a.Type == ActionDelete {
			t.Error("should NOT delete with prune=false")
		}
	}
}

// --- Apply tests ---

func TestRoutingApply_Create(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		postActionRules: []upstream.BifrostRoutingRuleResponse{
			{ID: "rule-new", Name: "New Rule", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
		},
	}
	r := NewRoutingRuleReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "routing-rule",
		ResourceID:   "rule-new",
		Desired: registry.RoutingRuleDesired{
			Name: "New Rule", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10,
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if mock.createdPayload == nil {
		t.Fatal("expected CreateRoutingRuleTyped to be called")
	}

	// Verify the payload includes the ID
	var payload map[string]any
	if err := json.Unmarshal(mock.createdPayload, &payload); err != nil {
		t.Fatalf("payload parse error: %v", err)
	}
	if payload["id"] != "rule-new" {
		t.Errorf("expected id 'rule-new' in payload, got %v", payload["id"])
	}
}

func TestRoutingApply_Update(t *testing.T) {
	mock := &mockBifrostRoutingClient{}
	r := NewRoutingRuleReconciler(mock, false)

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "routing-rule",
		ResourceID:   "rule-a",
		Desired: registry.RoutingRuleDesired{
			Name: "Rule A Updated", Enabled: true, CelExpression: "model == 'gpt-4o'", Scope: "global", Priority: 10,
		},
		Live: upstream.BifrostRoutingRuleResponse{
			ID: "rule-a", Name: "Rule A", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10,
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if mock.updatedID != "rule-a" {
		t.Errorf("expected updatedID 'rule-a', got %s", mock.updatedID)
	}
}

func TestRoutingApply_Delete(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		postActionRules: []upstream.BifrostRoutingRuleResponse{},
	}
	r := NewRoutingRuleReconciler(mock, true)

	action := Action{
		Type:         ActionDelete,
		ResourceType: "routing-rule",
		ResourceID:   "rule-del",
		Live: upstream.BifrostRoutingRuleResponse{
			ID: "rule-del", Name: "Delete Me",
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if mock.deletedID != "rule-del" {
		t.Errorf("expected deletedID 'rule-del', got %s", mock.deletedID)
	}
}

// --- Verify tests ---

func TestRoutingVerify_CreateSuccess(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		rules: []upstream.BifrostRoutingRuleResponse{
			{ID: "rule-new", Name: "New Rule", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10},
		},
	}
	r := NewRoutingRuleReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "routing-rule",
		ResourceID:   "rule-new",
		Desired: registry.RoutingRuleDesired{
			Name: "New Rule", Enabled: true, CelExpression: "model == 'gpt-4'", Scope: "global", Priority: 10,
		},
	}
	result := &Result{Action: action, Status: StatusOK}

	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s (drift: %s)", result.Status, result.DriftMsg)
	}
}

func TestRoutingVerify_DeleteConfirmsAbsence(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		rules: []upstream.BifrostRoutingRuleResponse{}, // rule absent
	}
	r := NewRoutingRuleReconciler(mock, true)

	action := Action{
		Type:         ActionDelete,
		ResourceType: "routing-rule",
		ResourceID:   "rule-del",
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

func TestRoutingVerify_DeleteStillPresent(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		rules: []upstream.BifrostRoutingRuleResponse{
			{ID: "rule-del", Name: "Should Be Gone"},
		},
	}
	r := NewRoutingRuleReconciler(mock, true)

	action := Action{
		Type:         ActionDelete,
		ResourceType: "routing-rule",
		ResourceID:   "rule-del",
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

func TestRoutingName(t *testing.T) {
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false)
	if r.Name() != "routing-rule" {
		t.Errorf("expected name 'routing-rule', got %s", r.Name())
	}
}

func TestRoutingApply_CreateError(t *testing.T) {
	mock := &mockBifrostRoutingClient{
		createErr: fmt.Errorf("connection refused"),
	}
	r := NewRoutingRuleReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "routing-rule",
		ResourceID:   "rule-new",
		Desired: registry.RoutingRuleDesired{
			Name: "New Rule", Enabled: true, CelExpression: "true", Scope: "global",
		},
	}

	_, err := r.Apply(action)
	if err == nil {
		t.Fatal("expected error from Apply")
	}
}

func TestRoutingPlan_TargetsDiff_ReturnsUpdate(t *testing.T) {
	provA := "openai"
	provB := "anthropic"
	r := NewRoutingRuleReconciler(&mockBifrostRoutingClient{}, false)

	desired := map[string]registry.RoutingRuleDesired{
		"rule-a": {
			Name: "Rule A", Enabled: true, CelExpression: "true", Scope: "global", Priority: 10,
			Targets: []registry.RoutingTargetDesired{
				{Provider: &provB, Weight: 1.0},
			},
		},
	}
	live := []upstream.BifrostRoutingRuleResponse{
		{
			ID: "rule-a", Name: "Rule A", Enabled: true, CelExpression: "true", Scope: "global", Priority: 10,
			Targets: []upstream.BifrostRoutingTargetResp{
				{Provider: &provA, Weight: 1.0},
			},
		},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	updates := 0
	for _, a := range actions {
		if a.Type == ActionUpdate {
			updates++
		}
	}
	if updates != 1 {
		t.Errorf("expected 1 update for target diff, got %d", updates)
	}
}
