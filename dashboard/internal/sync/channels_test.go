package sync

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// mockChannelClient satisfies NewAPIChannelClient for testing.
type mockChannelClient struct {
	channels       []upstream.ChannelResponse
	createdPayload json.RawMessage
	updatedPayload json.RawMessage
	deletedID      string
	getChannelFn   func(id int) (*upstream.ChannelResponse, error)
}

func (m *mockChannelClient) ListChannelsTyped(adminToken string) ([]upstream.ChannelResponse, error) {
	return m.channels, nil
}

func (m *mockChannelClient) GetChannelTyped(adminToken string, id int) (*upstream.ChannelResponse, error) {
	if m.getChannelFn != nil {
		return m.getChannelFn(id)
	}
	for i := range m.channels {
		if m.channels[i].ID == id {
			return &m.channels[i], nil
		}
	}
	return nil, fmt.Errorf("channel %d not found", id)
}

func (m *mockChannelClient) CreateChannel(adminToken string, channel json.RawMessage) (json.RawMessage, error) {
	m.createdPayload = channel
	// Return a response with success and data containing id
	return json.RawMessage(`{"success":true,"message":"","data":{"id":99,"name":"created"}}`), nil
}

func (m *mockChannelClient) UpdateChannelTyped(adminToken string, channel json.RawMessage) error {
	m.updatedPayload = channel
	return nil
}

func (m *mockChannelClient) DeleteChannel(adminToken string, id string) error {
	m.deletedID = id
	return nil
}

func TestChannelReconciler_Plan_CreateTwo(t *testing.T) {
	r := NewChannelReconciler(nil, "token", true, func(envName string) string { return "key" })

	desired := map[string]registry.ChannelDesired{
		"ch-a": {Name: "ch-a", Type: 1, Models: "gpt-4", Group: "default", Status: 1},
		"ch-b": {Name: "ch-b", Type: 1, Models: "claude-3", Group: "default", Status: 1},
	}
	live := []upstream.ChannelResponse{}

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

func TestChannelReconciler_Plan_MatchByName(t *testing.T) {
	r := NewChannelReconciler(nil, "token", true, func(envName string) string { return "key" })

	desired := map[string]registry.ChannelDesired{
		"bifrost-main": {Name: "bifrost-main", Type: 1, Models: "gpt-4", Group: "default", Status: 1},
	}
	live := []upstream.ChannelResponse{
		{ID: 42, Name: "bifrost-main", Type: 1, Models: "gpt-4", Group: "default", Status: 1},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	// Should be no actions since they match
	for _, a := range actions {
		if a.Type != ActionNoChange {
			t.Errorf("expected no-change, got %s for %s", a.Type, a.ResourceID)
		}
	}
}

func TestChannelReconciler_Plan_UpdateOnDiff(t *testing.T) {
	r := NewChannelReconciler(nil, "token", true, func(envName string) string { return "key" })

	newURL := "https://new-url.example.com"
	oldURL := "https://old-url.example.com"
	desired := map[string]registry.ChannelDesired{
		"bifrost-main": {Name: "bifrost-main", Type: 1, BaseURL: &newURL, Models: "gpt-4", Group: "default", Status: 1},
	}
	live := []upstream.ChannelResponse{
		{ID: 10, Name: "bifrost-main", Type: 1, BaseURL: &oldURL, Models: "gpt-4", Group: "default", Status: 1},
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
		t.Errorf("expected 1 update, got %d", updates)
	}
}

func TestChannelReconciler_Plan_PruneUnused(t *testing.T) {
	r := NewChannelReconciler(nil, "token", true, func(envName string) string { return "key" })

	desired := map[string]registry.ChannelDesired{}
	live := []upstream.ChannelResponse{
		{ID: 5, Name: "stale-channel", Type: 1, Models: "gpt-4", Group: "default", Status: 1, UsedQuota: 0},
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

func TestChannelReconciler_Plan_NoPruneWithUsedQuota(t *testing.T) {
	r := NewChannelReconciler(nil, "token", true, func(envName string) string { return "key" })

	desired := map[string]registry.ChannelDesired{}
	live := []upstream.ChannelResponse{
		{ID: 5, Name: "billing-channel", Type: 1, Models: "gpt-4", Group: "default", Status: 1, UsedQuota: 100},
	}

	actions, err := r.Plan(desired, live)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, a := range actions {
		if a.Type == ActionDelete {
			t.Error("should NOT delete channel with used_quota > 0")
		}
	}
}

func TestChannelReconciler_Apply_Create(t *testing.T) {
	mock := &mockChannelClient{
		channels: []upstream.ChannelResponse{
			{ID: 99, Name: "ch-a", Type: 1, Models: "gpt-4", Group: "default", Status: 1},
		},
	}
	r := NewChannelReconciler(mock, "token", true, func(envName string) string { return "key" })

	action := Action{
		Type:         ActionCreate,
		ResourceType: "channel",
		ResourceID:   "ch-a",
		Desired:      registry.ChannelDesired{Name: "ch-a", Type: 1, KeyEnv: "K", Models: "gpt-4", Group: "default", Status: 1},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}

	// Verify it was wrapped in AddChannelRequest
	var wrapper registry.AddChannelRequest
	if err := json.Unmarshal(mock.createdPayload, &wrapper); err != nil {
		t.Fatalf("created payload is not AddChannelRequest: %v", err)
	}
	if wrapper.Mode != "single" {
		t.Errorf("mode = %q, want %q", wrapper.Mode, "single")
	}
}

func TestChannelReconciler_Apply_Update(t *testing.T) {
	mock := &mockChannelClient{
		channels: []upstream.ChannelResponse{
			{ID: 10, Name: "ch-a", Type: 1, Models: "gpt-4", Group: "default", Status: 1},
		},
	}
	r := NewChannelReconciler(mock, "token", true, func(envName string) string { return "key" })

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "channel",
		ResourceID:   "ch-a",
		Desired:      registry.ChannelDesired{Name: "ch-a", Type: 1, KeyEnv: "K", Models: "gpt-4", Group: "default", Status: 1},
		Live:         upstream.ChannelResponse{ID: 10, Name: "ch-a"},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}

	// Verify the payload includes the live channel ID
	var payload registry.ChannelAPIPayload
	if err := json.Unmarshal(mock.updatedPayload, &payload); err != nil {
		t.Fatalf("updated payload parse error: %v", err)
	}
	if payload.ID != 10 {
		t.Errorf("payload ID = %d, want 10", payload.ID)
	}
}

func TestChannelReconciler_Verify_Update(t *testing.T) {
	ch := upstream.ChannelResponse{
		ID: 10, Name: "ch-a", Type: 1, Models: "gpt-4", Group: "default", Status: 1,
	}
	mock := &mockChannelClient{
		getChannelFn: func(id int) (*upstream.ChannelResponse, error) {
			if id == 10 {
				return &ch, nil
			}
			return nil, fmt.Errorf("not found")
		},
	}
	r := NewChannelReconciler(mock, "token", true, func(envName string) string { return "key" })

	action := Action{
		Type:         ActionUpdate,
		ResourceType: "channel",
		ResourceID:   "ch-a",
		Desired:      registry.ChannelDesired{Name: "ch-a", Type: 1, KeyEnv: "K", Models: "gpt-4", Group: "default", Status: 1},
		Live:         upstream.ChannelResponse{ID: 10, Name: "ch-a"},
	}

	result := &Result{Action: action, Status: StatusOK}
	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
}

func TestChannelReconciler_Verify_Delete(t *testing.T) {
	mock := &mockChannelClient{
		channels: []upstream.ChannelResponse{}, // channel was deleted, not in list
	}
	r := NewChannelReconciler(mock, "token", true, func(envName string) string { return "key" })

	action := Action{
		Type:         ActionDelete,
		ResourceType: "channel",
		ResourceID:   "deleted-ch",
		Live:         upstream.ChannelResponse{ID: 5, Name: "deleted-ch"},
	}

	result := &Result{Action: action, Status: StatusOK}
	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
}

func TestChannelReconciler_Apply_Delete(t *testing.T) {
	mock := &mockChannelClient{
		channels: []upstream.ChannelResponse{},
	}
	r := NewChannelReconciler(mock, "token", true, func(envName string) string { return "key" })

	action := Action{
		Type:         ActionDelete,
		ResourceType: "channel",
		ResourceID:   "ch-del",
		Live:         upstream.ChannelResponse{ID: 7, Name: "ch-del"},
	}

	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if mock.deletedID != strconv.Itoa(7) {
		t.Errorf("deletedID = %q, want %q", mock.deletedID, "7")
	}
}
