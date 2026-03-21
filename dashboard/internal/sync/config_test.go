package sync

import (
	"fmt"
	"testing"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// --- Mock client ---

type mockBifrostConfigClient struct {
	config       *upstream.BifrostConfigResponse
	lastUpdate   *upstream.BifrostConfigResponse
	updateErr    error
	getErr       error
	postUpdateCfg *upstream.BifrostConfigResponse // if set, GetConfigTyped returns this after first update
	updated      bool
}

func (m *mockBifrostConfigClient) GetConfigTyped() (*upstream.BifrostConfigResponse, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.updated && m.postUpdateCfg != nil {
		return m.postUpdateCfg, nil
	}
	return m.config, nil
}

func (m *mockBifrostConfigClient) UpdateConfigTyped(config *upstream.BifrostConfigResponse) error {
	m.lastUpdate = config
	m.updated = true
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

// --- Compile-time check ---

var _ Reconciler = (*ConfigReconciler)(nil)

// --- Tests ---

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestConfigPlan_NoChanges(t *testing.T) {
	r := NewConfigReconciler(&mockBifrostConfigClient{})
	desired := &registry.BifrostClientConfig{
		MaxRetries: intPtr(3),
	}
	live := &upstream.BifrostConfigResponse{
		MaxRetries: intPtr(3),
	}
	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestConfigPlan_OneFieldChanged(t *testing.T) {
	r := NewConfigReconciler(&mockBifrostConfigClient{})
	desired := &registry.BifrostClientConfig{
		MaxRetries: intPtr(5),
	}
	live := &upstream.BifrostConfigResponse{
		MaxRetries: intPtr(3),
	}
	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", actions[0].Type)
	}
	if actions[0].ResourceType != "config" {
		t.Errorf("expected resource type 'config', got %s", actions[0].ResourceType)
	}
	if actions[0].ResourceID != "bifrost-global" {
		t.Errorf("expected resource ID 'bifrost-global', got %s", actions[0].ResourceID)
	}
	// Diff should mention max_retries
	if actions[0].Diff == "" {
		t.Error("expected non-empty Diff")
	}
}

func TestConfigPlan_NilDesiredFieldNoUpdate(t *testing.T) {
	// If desired field is nil and live field is non-nil, no update should be generated.
	// We only sync what operator explicitly declares.
	r := NewConfigReconciler(&mockBifrostConfigClient{})
	desired := &registry.BifrostClientConfig{
		MaxRetries: nil, // operator didn't declare this
	}
	live := &upstream.BifrostConfigResponse{
		MaxRetries: intPtr(3), // live has a value
	}
	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (nil desired should not generate update), got %d", len(actions))
	}
}

func TestConfigPlan_MultipleFieldsChanged(t *testing.T) {
	r := NewConfigReconciler(&mockBifrostConfigClient{})
	desired := &registry.BifrostClientConfig{
		MaxRetries:        intPtr(5),
		InitialPoolSize:   intPtr(20),
		ProxyURL:          strPtr("http://proxy:8080"),
	}
	live := &upstream.BifrostConfigResponse{
		MaxRetries:        intPtr(3),
		InitialPoolSize:   intPtr(10),
		ProxyURL:          strPtr("http://old-proxy:8080"),
	}
	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action (single config update), got %d", len(actions))
	}
	if actions[0].Type != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %s", actions[0].Type)
	}
}

func TestConfigApply_SendsOnlyChangedFields(t *testing.T) {
	mock := &mockBifrostConfigClient{
		config: &upstream.BifrostConfigResponse{
			MaxRetries: intPtr(5),
		},
		postUpdateCfg: &upstream.BifrostConfigResponse{
			MaxRetries: intPtr(5),
		},
	}
	r := NewConfigReconciler(mock)

	// Build an action with only MaxRetries changed
	desired := &registry.BifrostClientConfig{MaxRetries: intPtr(5)}
	live := &upstream.BifrostConfigResponse{MaxRetries: intPtr(3)}

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "config",
		ResourceID:   "bifrost-global",
		Desired:      desired,
		Live:         live,
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}

	// Verify only changed fields were sent
	if mock.lastUpdate == nil {
		t.Fatal("expected UpdateConfigTyped to be called")
	}
	if mock.lastUpdate.MaxRetries == nil || *mock.lastUpdate.MaxRetries != 5 {
		t.Error("expected MaxRetries=5 in update payload")
	}
	// Fields not in desired should be nil (not sent)
	if mock.lastUpdate.InitialPoolSize != nil {
		t.Error("expected InitialPoolSize to be nil in update payload")
	}
}

func TestConfigVerify_Success(t *testing.T) {
	mock := &mockBifrostConfigClient{
		config: &upstream.BifrostConfigResponse{
			MaxRetries: intPtr(5),
		},
	}
	r := NewConfigReconciler(mock)

	desired := &registry.BifrostClientConfig{MaxRetries: intPtr(5)}
	action := Action{
		Type:         ActionUpdate,
		ResourceType: "config",
		ResourceID:   "bifrost-global",
		Desired:      desired,
	}
	result := &Result{
		Action: action,
		Status: StatusOK,
	}

	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
}

func TestConfigVerify_DetectsSilentRejection(t *testing.T) {
	// Bifrost accepted the PUT but ignored the field (silent rejection)
	mock := &mockBifrostConfigClient{
		config: &upstream.BifrostConfigResponse{
			MaxRetries: intPtr(3), // still old value
		},
	}
	r := NewConfigReconciler(mock)

	desired := &registry.BifrostClientConfig{MaxRetries: intPtr(5)}
	action := Action{
		Type:         ActionUpdate,
		ResourceType: "config",
		ResourceID:   "bifrost-global",
		Desired:      desired,
	}
	result := &Result{
		Action: action,
		Status: StatusOK,
	}

	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if result.Status != StatusAppliedWithDrift {
		t.Errorf("expected StatusAppliedWithDrift, got %s", result.Status)
	}
	if result.DriftMsg == "" {
		t.Error("expected non-empty DriftMsg")
	}
}

func TestConfigName(t *testing.T) {
	r := NewConfigReconciler(&mockBifrostConfigClient{})
	if r.Name() != "config" {
		t.Errorf("expected name 'config', got %s", r.Name())
	}
}

func TestConfigApply_ClientError(t *testing.T) {
	mock := &mockBifrostConfigClient{
		updateErr: fmt.Errorf("connection refused"),
	}
	r := NewConfigReconciler(mock)

	desired := &registry.BifrostClientConfig{MaxRetries: intPtr(5)}
	action := Action{
		Type:         ActionUpdate,
		ResourceType: "config",
		ResourceID:   "bifrost-global",
		Desired:      desired,
		Live:         &upstream.BifrostConfigResponse{MaxRetries: intPtr(3)},
	}

	result, err := r.Apply(action)
	if err == nil {
		t.Fatal("expected error from Apply")
	}
	if result != nil && result.Status != StatusFailed {
		t.Errorf("expected StatusFailed or nil result")
	}
}
