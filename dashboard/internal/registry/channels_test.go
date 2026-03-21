package registry

import (
	"encoding/json"
	"testing"
)

func TestToAPIPayload_ModelMapping(t *testing.T) {
	ch := ChannelDesired{
		Name:   "test-channel",
		Type:   1,
		KeyEnv: "TEST_KEY",
		Models: "gpt-4,claude-3",
		Group:  "default",
		Status: 1,
		ModelMapping: map[string]string{
			"gpt-4":    "gpt-4-turbo",
			"claude-3": "claude-3-opus",
		},
	}

	payload, err := ch.ToAPIPayload("sk-test-key-123")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}

	if payload.Key != "sk-test-key-123" {
		t.Errorf("Key = %q, want %q", payload.Key, "sk-test-key-123")
	}
	if payload.ModelMapping == nil {
		t.Fatal("ModelMapping should not be nil")
	}

	var mm map[string]string
	if err := json.Unmarshal([]byte(*payload.ModelMapping), &mm); err != nil {
		t.Fatalf("ModelMapping is not valid JSON: %v", err)
	}
	if mm["gpt-4"] != "gpt-4-turbo" {
		t.Errorf("ModelMapping[gpt-4] = %q, want %q", mm["gpt-4"], "gpt-4-turbo")
	}
}

func TestToAPIPayload_Setting(t *testing.T) {
	ch := ChannelDesired{
		Name:   "test-channel",
		Type:   1,
		KeyEnv: "TEST_KEY",
		Models: "gpt-4",
		Group:  "default",
		Status: 1,
		Setting: map[string]any{
			"temperature": 0.7,
		},
	}

	payload, err := ch.ToAPIPayload("key")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}
	if payload.Setting == nil {
		t.Fatal("Setting should not be nil")
	}

	var s map[string]any
	if err := json.Unmarshal([]byte(*payload.Setting), &s); err != nil {
		t.Fatalf("Setting is not valid JSON: %v", err)
	}
	if s["temperature"] != 0.7 {
		t.Errorf("Setting[temperature] = %v, want 0.7", s["temperature"])
	}
}

func TestToAPIPayload_ParamAndHeaderOverride(t *testing.T) {
	ch := ChannelDesired{
		Name:   "test-channel",
		Type:   1,
		KeyEnv: "TEST_KEY",
		Models: "gpt-4",
		Group:  "default",
		Status: 1,
		ParamOverride: map[string]any{
			"max_tokens": 1024,
		},
		HeaderOverride: map[string]string{
			"X-Custom": "value",
		},
	}

	payload, err := ch.ToAPIPayload("key")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}
	if payload.ParamOverride == nil {
		t.Fatal("ParamOverride should not be nil")
	}
	if payload.HeaderOverride == nil {
		t.Fatal("HeaderOverride should not be nil")
	}

	var po map[string]any
	if err := json.Unmarshal([]byte(*payload.ParamOverride), &po); err != nil {
		t.Fatalf("ParamOverride is not valid JSON: %v", err)
	}

	var ho map[string]string
	if err := json.Unmarshal([]byte(*payload.HeaderOverride), &ho); err != nil {
		t.Fatalf("HeaderOverride is not valid JSON: %v", err)
	}
	if ho["X-Custom"] != "value" {
		t.Errorf("HeaderOverride[X-Custom] = %q, want %q", ho["X-Custom"], "value")
	}
}

func TestToAPIPayload_ResolvesKey(t *testing.T) {
	ch := ChannelDesired{
		Name:   "test-channel",
		Type:   1,
		KeyEnv: "MY_API_KEY",
		Models: "gpt-4",
		Group:  "default",
		Status: 1,
	}

	payload, err := ch.ToAPIPayload("resolved-api-key-value")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}
	if payload.Key != "resolved-api-key-value" {
		t.Errorf("Key = %q, want %q", payload.Key, "resolved-api-key-value")
	}
}

func TestToAPIPayload_EmptyMapsAreNil(t *testing.T) {
	ch := ChannelDesired{
		Name:   "test-channel",
		Type:   1,
		KeyEnv: "TEST_KEY",
		Models: "gpt-4",
		Group:  "default",
		Status: 1,
	}

	payload, err := ch.ToAPIPayload("key")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}
	if payload.ModelMapping != nil {
		t.Error("ModelMapping should be nil for empty map")
	}
	if payload.Setting != nil {
		t.Error("Setting should be nil for empty map")
	}
	if payload.ParamOverride != nil {
		t.Error("ParamOverride should be nil for empty map")
	}
	if payload.HeaderOverride != nil {
		t.Error("HeaderOverride should be nil for empty map")
	}
}

func TestToAPIPayload_CopiesDirectFields(t *testing.T) {
	baseURL := "https://api.example.com"
	tag := "prod"
	priority := int64(10)
	weight := uint(5)
	autoBan := 1

	ch := ChannelDesired{
		Name:     "test-channel",
		Type:     3,
		KeyEnv:   "K",
		BaseURL:  &baseURL,
		Models:   "gpt-4,gpt-3.5",
		Group:    "vip",
		Tag:      &tag,
		Priority: &priority,
		Weight:   &weight,
		Status:   2,
		AutoBan:  &autoBan,
	}

	payload, err := ch.ToAPIPayload("key")
	if err != nil {
		t.Fatalf("ToAPIPayload error: %v", err)
	}
	if payload.Name != "test-channel" {
		t.Errorf("Name = %q", payload.Name)
	}
	if payload.Type != 3 {
		t.Errorf("Type = %d", payload.Type)
	}
	if *payload.BaseURL != baseURL {
		t.Errorf("BaseURL = %q", *payload.BaseURL)
	}
	if payload.Models != "gpt-4,gpt-3.5" {
		t.Errorf("Models = %q", payload.Models)
	}
	if payload.Group != "vip" {
		t.Errorf("Group = %q", payload.Group)
	}
	if *payload.Tag != "prod" {
		t.Errorf("Tag = %q", *payload.Tag)
	}
	if *payload.Priority != 10 {
		t.Errorf("Priority = %d", *payload.Priority)
	}
	if *payload.Weight != 5 {
		t.Errorf("Weight = %d", *payload.Weight)
	}
	if payload.Status != 2 {
		t.Errorf("Status = %d", payload.Status)
	}
	if *payload.AutoBan != 1 {
		t.Errorf("AutoBan = %d", *payload.AutoBan)
	}
}
