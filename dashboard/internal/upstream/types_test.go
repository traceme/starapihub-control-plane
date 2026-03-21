package upstream

import (
	"encoding/json"
	"testing"
)

func TestChannelResponseJSONRoundTrip(t *testing.T) {
	input := `{
		"id": 42,
		"name": "openai-main",
		"type": 1,
		"base_url": "https://api.openai.com",
		"models": "gpt-4,gpt-3.5-turbo",
		"group": "default",
		"tag": "production",
		"model_mapping": "{\"gpt-4\":\"gpt-4-0613\"}",
		"priority": 10,
		"weight": 5,
		"status": 1,
		"auto_ban": 1,
		"setting": "{\"timeout\":30}",
		"param_override": "{\"temperature\":0.7}",
		"header_override": "{\"X-Custom\":\"val\"}",
		"used_quota": 1500,
		"created_time": 1700000000,
		"test_time": 1700001000,
		"response_time": 200,
		"balance": 99.5
	}`

	var cr ChannelResponse
	if err := json.Unmarshal([]byte(input), &cr); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cr.ID != 42 {
		t.Errorf("ID = %d, want 42", cr.ID)
	}
	if cr.Name != "openai-main" {
		t.Errorf("Name = %q, want openai-main", cr.Name)
	}
	if cr.Type != 1 {
		t.Errorf("Type = %d, want 1", cr.Type)
	}
	if cr.BaseURL == nil || *cr.BaseURL != "https://api.openai.com" {
		t.Error("BaseURL mismatch")
	}
	if cr.Models != "gpt-4,gpt-3.5-turbo" {
		t.Error("Models mismatch")
	}
	if cr.Group != "default" {
		t.Error("Group mismatch")
	}
	if cr.Tag == nil || *cr.Tag != "production" {
		t.Error("Tag mismatch")
	}
	if cr.ModelMapping == nil || *cr.ModelMapping != `{"gpt-4":"gpt-4-0613"}` {
		t.Error("ModelMapping mismatch")
	}
	if cr.Priority == nil || *cr.Priority != 10 {
		t.Error("Priority mismatch")
	}
	if cr.Weight == nil || *cr.Weight != 5 {
		t.Error("Weight mismatch")
	}
	if cr.Status != 1 {
		t.Error("Status mismatch")
	}
	if cr.AutoBan == nil || *cr.AutoBan != 1 {
		t.Error("AutoBan mismatch")
	}
	if cr.Setting == nil || *cr.Setting != `{"timeout":30}` {
		t.Error("Setting mismatch")
	}
	if cr.ParamOverride == nil || *cr.ParamOverride != `{"temperature":0.7}` {
		t.Error("ParamOverride mismatch")
	}
	if cr.HeaderOverride == nil || *cr.HeaderOverride != `{"X-Custom":"val"}` {
		t.Error("HeaderOverride mismatch")
	}
	if cr.UsedQuota != 1500 {
		t.Errorf("UsedQuota = %d, want 1500", cr.UsedQuota)
	}

	// Round-trip: marshal and unmarshal again
	out, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var cr2 ChannelResponse
	if err := json.Unmarshal(out, &cr2); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	if cr2.ID != cr.ID || cr2.Name != cr.Name || cr2.UsedQuota != cr.UsedQuota {
		t.Error("round-trip mismatch")
	}
}

func TestBifrostProviderResponseJSONRoundTrip(t *testing.T) {
	input := `{
		"keys": [{
			"id": "key-1",
			"name": "main-key",
			"value": "sk-xxx",
			"models": ["gpt-4", "gpt-3.5-turbo"],
			"weight": 1.0,
			"enabled": true,
			"description": "primary key"
		}],
		"network_config": {
			"base_url": "https://api.openai.com",
			"extra_headers": {"X-Custom": "val"},
			"default_request_timeout_in_seconds": 30,
			"max_retries": 3,
			"retry_backoff_initial": 100,
			"retry_backoff_max": 5000,
			"stream_idle_timeout_in_seconds": 60
		},
		"custom_provider_config": {
			"is_key_less": false,
			"base_provider_type": "openai"
		}
	}`

	var pr BifrostProviderResponse
	if err := json.Unmarshal([]byte(input), &pr); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(pr.Keys) != 1 {
		t.Fatalf("Keys len = %d, want 1", len(pr.Keys))
	}
	if pr.Keys[0].ID != "key-1" {
		t.Error("Key ID mismatch")
	}
	if pr.Keys[0].Name != "main-key" {
		t.Error("Key Name mismatch")
	}
	if pr.Keys[0].Value != "sk-xxx" {
		t.Error("Key Value mismatch")
	}
	if len(pr.Keys[0].Models) != 2 {
		t.Error("Key Models mismatch")
	}
	if pr.NetworkConfig == nil {
		t.Fatal("NetworkConfig is nil")
	}
	if pr.NetworkConfig.MaxRetries != 3 {
		t.Error("NetworkConfig.MaxRetries mismatch")
	}
	if pr.CustomProviderConfig == nil {
		t.Fatal("CustomProviderConfig is nil")
	}
	if pr.CustomProviderConfig.BaseProviderType != "openai" {
		t.Error("CustomProviderConfig.BaseProviderType mismatch")
	}

	// Round-trip
	out, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var pr2 BifrostProviderResponse
	if err := json.Unmarshal(out, &pr2); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	if pr2.Keys[0].ID != pr.Keys[0].ID {
		t.Error("round-trip mismatch")
	}
}

func TestBifrostRoutingRuleResponseJSONRoundTrip(t *testing.T) {
	scopeID := "scope-123"
	input := `{
		"id": "rule-1",
		"name": "default-route",
		"description": "Route all traffic",
		"enabled": true,
		"cel_expression": "true",
		"targets": [{"provider": "openai", "model": "gpt-4", "weight": 1.0}],
		"fallbacks": ["anthropic"],
		"query": {"max_tokens": 1000},
		"scope": "global",
		"scope_id": "scope-123",
		"priority": 100
	}`

	var rr BifrostRoutingRuleResponse
	if err := json.Unmarshal([]byte(input), &rr); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if rr.ID != "rule-1" {
		t.Error("ID mismatch")
	}
	if rr.Name != "default-route" {
		t.Error("Name mismatch")
	}
	if !rr.Enabled {
		t.Error("Enabled mismatch")
	}
	if rr.CelExpression != "true" {
		t.Error("CelExpression mismatch")
	}
	if len(rr.Targets) != 1 {
		t.Error("Targets len mismatch")
	}
	if rr.Targets[0].Provider == nil || *rr.Targets[0].Provider != "openai" {
		t.Error("Target provider mismatch")
	}
	if len(rr.Fallbacks) != 1 || rr.Fallbacks[0] != "anthropic" {
		t.Error("Fallbacks mismatch")
	}
	if rr.ScopeID == nil || *rr.ScopeID != scopeID {
		t.Error("ScopeID mismatch")
	}
	if rr.Priority != 100 {
		t.Error("Priority mismatch")
	}

	// Round-trip
	out, err := json.Marshal(rr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var rr2 BifrostRoutingRuleResponse
	if err := json.Unmarshal(out, &rr2); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	if rr2.ID != rr.ID || rr2.Priority != rr.Priority {
		t.Error("round-trip mismatch")
	}
}

func TestOptionEntryJSONRoundTrip(t *testing.T) {
	input := `{"key":"ModelRatio","value":"{\"gpt-4\":15}"}`

	var oe OptionEntry
	if err := json.Unmarshal([]byte(input), &oe); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if oe.Key != "ModelRatio" {
		t.Error("Key mismatch")
	}
	if oe.Value != `{"gpt-4":15}` {
		t.Error("Value mismatch")
	}

	// Round-trip
	out, err := json.Marshal(oe)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var oe2 OptionEntry
	if err := json.Unmarshal(out, &oe2); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	if oe2.Key != oe.Key || oe2.Value != oe.Value {
		t.Error("round-trip mismatch")
	}
}

func TestCookieStatusTypedJSONRoundTrip(t *testing.T) {
	input := `{"cookie":"sk-ant-xxx123"}`

	var cs CookieStatusTyped
	if err := json.Unmarshal([]byte(input), &cs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cs.Cookie != "sk-ant-xxx123" {
		t.Errorf("Cookie = %q, want sk-ant-xxx123", cs.Cookie)
	}

	// Round-trip
	out, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var cs2 CookieStatusTyped
	if err := json.Unmarshal(out, &cs2); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	if cs2.Cookie != cs.Cookie {
		t.Error("round-trip mismatch")
	}
}
