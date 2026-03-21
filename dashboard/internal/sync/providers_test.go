package sync

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// --- Mock Bifrost provider client ---

type mockBifrostProviderClient struct {
	providers      map[string]upstream.BifrostProviderResponse
	createdID      string
	createdPayload json.RawMessage
	updatedID      string
	updatedPayload json.RawMessage
	deletedID      string
	createErr      error
	updateErr      error
	deleteErr      error
	listErr        error
}

func (m *mockBifrostProviderClient) ListProvidersTyped() (map[string]upstream.BifrostProviderResponse, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.providers, nil
}

func (m *mockBifrostProviderClient) CreateProviderTyped(id string, provider json.RawMessage) error {
	m.createdID = id
	m.createdPayload = provider
	return m.createErr
}

func (m *mockBifrostProviderClient) UpdateProviderTyped(id string, provider json.RawMessage) error {
	m.updatedID = id
	m.updatedPayload = provider
	return m.updateErr
}

func (m *mockBifrostProviderClient) DeleteProvider(id string) error {
	m.deletedID = id
	return m.deleteErr
}

// --- ProviderReconciler Tests ---

func TestProviderPlan_2Desired0Live_Returns2Creates(t *testing.T) {
	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	desired := map[string]registry.BifrostProviderDesired{
		"openai": {Keys: []registry.BifrostKeyDesired{{ID: "k1", Name: "key1", ValueEnv: "K", Models: []string{"gpt-4"}, Weight: 1.0}}},
		"clewdr": {Keys: []registry.BifrostKeyDesired{{ID: "k2", Name: "key2", ValueEnv: "K", Models: []string{"claude-3"}, Weight: 1.0}}},
	}
	live := map[string]upstream.BifrostProviderResponse{}

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

func TestProviderPlan_1Desired1MatchingLive_Returns0Actions(t *testing.T) {
	enabled := true
	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	desired := map[string]registry.BifrostProviderDesired{
		"openai": {
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "K", Models: []string{"gpt-4"}, Weight: 1.0, Enabled: &enabled},
			},
		},
	}
	live := map[string]upstream.BifrostProviderResponse{
		"openai": {
			Keys: []upstream.BifrostKeyResponse{
				{ID: "k1", Name: "key1", Value: "secret-val", Models: []string{"gpt-4"}, Weight: 1.0, Enabled: &enabled},
			},
		},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (no-change), got %d", len(actions))
		for _, a := range actions {
			t.Logf("  action: %s %s diff=%q", a.Type, a.ResourceID, a.Diff)
		}
	}
}

func TestProviderPlan_1DesiredDifferingNetwork_Returns1Update(t *testing.T) {
	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	desired := map[string]registry.BifrostProviderDesired{
		"openai": {
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "K", Models: []string{"gpt-4"}, Weight: 1.0},
			},
			NetworkConfig: &registry.BifrostNetworkConfig{
				BaseURL:    "https://api.openai.com",
				MaxRetries: 5,
			},
		},
	}
	live := map[string]upstream.BifrostProviderResponse{
		"openai": {
			Keys: []upstream.BifrostKeyResponse{
				{ID: "k1", Name: "key1", Value: "secret", Models: []string{"gpt-4"}, Weight: 1.0},
			},
			NetworkConfig: &upstream.BifrostNetworkConfigResponse{
				BaseURL:    "https://api.openai.com",
				MaxRetries: 3, // different
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
			if a.Diff == "" {
				t.Error("expected non-empty Diff for update")
			}
		}
	}
	if updates != 1 {
		t.Errorf("expected 1 update, got %d", updates)
	}
}

func TestProviderPlan_PruneExtraLive_Returns1Delete(t *testing.T) {
	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, true) // prune=true

	desired := map[string]registry.BifrostProviderDesired{}
	live := map[string]upstream.BifrostProviderResponse{
		"stale-provider": {
			Keys: []upstream.BifrostKeyResponse{{ID: "k1", Name: "old-key", Value: "v", Models: []string{"m"}, Weight: 1.0}},
		},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	deletes := 0
	for _, a := range actions {
		if a.Type == ActionDelete {
			deletes++
		}
	}
	if deletes != 1 {
		t.Errorf("expected 1 delete, got %d", deletes)
	}
}

func TestProviderPlan_NoPruneExtraLive_Returns0Deletes(t *testing.T) {
	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false) // prune=false

	desired := map[string]registry.BifrostProviderDesired{}
	live := map[string]upstream.BifrostProviderResponse{
		"extra-provider": {
			Keys: []upstream.BifrostKeyResponse{{ID: "k1", Name: "key", Value: "v", Models: []string{"m"}, Weight: 1.0}},
		},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, a := range actions {
		if a.Type == ActionDelete {
			t.Error("should NOT delete when prune=false")
		}
	}
}

func TestProviderApply_Create_CallsCreateProviderTyped(t *testing.T) {
	os.Setenv("TEST_PROV_KEY", "resolved-key-value")
	defer os.Unsetenv("TEST_PROV_KEY")

	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "provider",
		ResourceID:   "openai",
		Desired: registry.BifrostProviderDesired{
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "TEST_PROV_KEY", Models: []string{"gpt-4"}, Weight: 1.0},
			},
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s (err: %v)", result.Status, result.Error)
	}
	if mock.createdID != "openai" {
		t.Errorf("expected createdID=openai, got %s", mock.createdID)
	}

	// Verify the payload contains the resolved key value
	var payload map[string]any
	if err := json.Unmarshal(mock.createdPayload, &payload); err != nil {
		t.Fatalf("payload parse error: %v", err)
	}
	keys, ok := payload["keys"].([]any)
	if !ok || len(keys) == 0 {
		t.Fatal("expected keys in payload")
	}
	keyMap, ok := keys[0].(map[string]any)
	if !ok {
		t.Fatal("expected key to be a map")
	}
	if keyMap["value"] != "resolved-key-value" {
		t.Errorf("expected resolved key value, got %v", keyMap["value"])
	}
}

func TestProviderApply_Update_CallsUpdateProviderTyped(t *testing.T) {
	os.Setenv("TEST_PROV_KEY", "resolved-key")
	defer os.Unsetenv("TEST_PROV_KEY")

	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "provider",
		ResourceID:   "openai",
		Desired: registry.BifrostProviderDesired{
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "TEST_PROV_KEY", Models: []string{"gpt-4"}, Weight: 1.0},
			},
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if mock.updatedID != "openai" {
		t.Errorf("expected updatedID=openai, got %s", mock.updatedID)
	}
}

func TestProviderVerify_ReadBackMatches(t *testing.T) {
	enabled := true
	mock := &mockBifrostProviderClient{
		providers: map[string]upstream.BifrostProviderResponse{
			"openai": {
				Keys: []upstream.BifrostKeyResponse{
					{ID: "k1", Name: "key1", Value: "secret", Models: []string{"gpt-4"}, Weight: 1.0, Enabled: &enabled},
				},
			},
		},
	}
	r := NewProviderReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "provider",
		ResourceID:   "openai",
		Desired: registry.BifrostProviderDesired{
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "K", Models: []string{"gpt-4"}, Weight: 1.0, Enabled: &enabled},
			},
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

func TestProviderVerify_DriftDetected(t *testing.T) {
	mock := &mockBifrostProviderClient{
		providers: map[string]upstream.BifrostProviderResponse{
			"openai": {
				Keys: []upstream.BifrostKeyResponse{
					{ID: "k1", Name: "key1", Value: "secret", Models: []string{"gpt-4"}, Weight: 1.0},
				},
				NetworkConfig: &upstream.BifrostNetworkConfigResponse{
					MaxRetries: 3, // different from desired
				},
			},
		},
	}
	r := NewProviderReconciler(mock, false)

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "provider",
		ResourceID:   "openai",
		Desired: registry.BifrostProviderDesired{
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "key1", ValueEnv: "K", Models: []string{"gpt-4"}, Weight: 1.0},
			},
			NetworkConfig: &registry.BifrostNetworkConfig{
				MaxRetries: 5, // we wanted 5
			},
		},
	}
	result := &Result{Action: action, Status: StatusOK}
	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusAppliedWithDrift {
		t.Errorf("expected StatusAppliedWithDrift, got %s", result.Status)
	}
	if result.DriftMsg == "" {
		t.Error("expected non-empty DriftMsg")
	}
}

func TestProviderApply_EnvVarResolution(t *testing.T) {
	os.Setenv("MY_API_KEY", "sk-test-123")
	defer os.Unsetenv("MY_API_KEY")

	mock := &mockBifrostProviderClient{providers: map[string]upstream.BifrostProviderResponse{}}
	r := NewProviderReconciler(mock, false)

	action := Action{
		Type:         ActionCreate,
		ResourceType: "provider",
		ResourceID:   "test-prov",
		Desired: registry.BifrostProviderDesired{
			Keys: []registry.BifrostKeyDesired{
				{ID: "k1", Name: "main-key", ValueEnv: "MY_API_KEY", Models: []string{"gpt-4"}, Weight: 1.0},
			},
		},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s (err: %v)", result.Status, result.Error)
	}

	// Verify the payload contains the resolved env var value
	var payload struct {
		Keys []struct {
			Value string `json:"value"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(mock.createdPayload, &payload); err != nil {
		t.Fatalf("payload parse: %v", err)
	}
	if len(payload.Keys) == 0 || payload.Keys[0].Value != "sk-test-123" {
		t.Errorf("expected resolved env var value sk-test-123 in payload, got %v", payload.Keys)
	}
}
