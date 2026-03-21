package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// policiesDir returns the path to the control-plane/policies/ directory
// relative to the test file location.
func policiesDir(t *testing.T) string {
	t.Helper()
	// The test file is at control-plane/dashboard/internal/registry/loader_test.go
	// policies/ is at control-plane/policies/
	dir := filepath.Join("..", "..", "..", "policies")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("policies directory not found at %s: %v", dir, err)
	}
	return dir
}

// writeTempYAML writes YAML content to a temporary file and returns the path.
func writeTempYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp yaml %s: %v", path, err)
	}
	return path
}

func TestLoadModels(t *testing.T) {
	dir := policiesDir(t)
	f, err := LoadModels(filepath.Join(dir, "logical-models.example.yaml"))
	if err != nil {
		t.Fatalf("LoadModels: %v", err)
	}
	if f == nil {
		t.Fatal("LoadModels returned nil")
	}
	if len(f.Models) != 9 {
		t.Errorf("expected 9 models, got %d", len(f.Models))
	}
	cs, ok := f.Models["claude-sonnet"]
	if !ok {
		t.Fatal("missing model claude-sonnet")
	}
	if cs.DisplayName != "Claude Sonnet" {
		t.Errorf("claude-sonnet DisplayName = %q, want %q", cs.DisplayName, "Claude Sonnet")
	}
	if cs.UpstreamModel != "claude-sonnet-4-20250514" {
		t.Errorf("claude-sonnet UpstreamModel = %q, want %q", cs.UpstreamModel, "claude-sonnet-4-20250514")
	}
}

func TestLoadRoutePolicies(t *testing.T) {
	dir := policiesDir(t)
	f, err := LoadRoutePolicies(filepath.Join(dir, "route-policies.example.yaml"))
	if err != nil {
		t.Fatalf("LoadRoutePolicies: %v", err)
	}
	if len(f.Policies) != 5 {
		t.Errorf("expected 5 policies, got %d", len(f.Policies))
	}
	p, ok := f.Policies["premium"]
	if !ok {
		t.Fatal("missing policy premium")
	}
	if p.FallbackBehavior != "fail" {
		t.Errorf("premium FallbackBehavior = %q, want %q", p.FallbackBehavior, "fail")
	}
}

func TestLoadProviderPools(t *testing.T) {
	dir := policiesDir(t)
	f, err := LoadProviderPools(filepath.Join(dir, "provider-pools.example.yaml"))
	if err != nil {
		t.Fatalf("LoadProviderPools: %v", err)
	}
	if len(f.Pools) != 4 {
		t.Errorf("expected 4 pools, got %d", len(f.Pools))
	}
	p, ok := f.Pools["official-anthropic"]
	if !ok {
		t.Fatal("missing pool official-anthropic")
	}
	if p.TrustLevel != "high" {
		t.Errorf("official-anthropic TrustLevel = %q, want %q", p.TrustLevel, "high")
	}
}

func TestLoadChannels(t *testing.T) {
	dir := t.TempDir()
	writeTempYAML(t, dir, "channels.yaml", `
channels:
  bifrost-premium:
    name: "Bifrost Premium"
    type: 43
    key_env: BIFROST_API_KEY
    base_url: "http://bifrost:8080"
    models: "claude-sonnet,claude-opus"
    group: "default"
    model_mapping:
      claude-sonnet: "claude-sonnet-4-20250514"
    priority: 0
    weight: 10
    status: 1
`)
	f, err := LoadChannels(filepath.Join(dir, "channels.yaml"))
	if err != nil {
		t.Fatalf("LoadChannels: %v", err)
	}
	ch, ok := f.Channels["bifrost-premium"]
	if !ok {
		t.Fatal("missing channel bifrost-premium")
	}
	if ch.Name != "Bifrost Premium" {
		t.Errorf("Name = %q, want %q", ch.Name, "Bifrost Premium")
	}
	if ch.Type != 43 {
		t.Errorf("Type = %d, want 43", ch.Type)
	}
	if ch.ModelMapping == nil {
		t.Fatal("ModelMapping is nil")
	}
	if ch.ModelMapping["claude-sonnet"] != "claude-sonnet-4-20250514" {
		t.Errorf("ModelMapping[claude-sonnet] = %q", ch.ModelMapping["claude-sonnet"])
	}
}

func TestLoadProviders(t *testing.T) {
	dir := t.TempDir()
	writeTempYAML(t, dir, "providers.yaml", `
providers:
  anthropic:
    keys:
      - id: key-1
        name: "Primary"
        value_env: ANTHROPIC_API_KEY
        models: ["claude-sonnet-4-20250514"]
        weight: 1.0
    network_config:
      max_retries: 2
      default_request_timeout_in_seconds: 60
      retry_backoff_initial_ms: 500
      retry_backoff_max_ms: 5000
  clewdr-1:
    keys:
      - id: clewdr-key-1
        name: "ClewdR 1"
        value_env: CLEWDR_1_PASSWORD
        models: ["claude-sonnet-4-20250514"]
        weight: 1.0
    custom_provider_config:
      is_key_less: false
      base_provider_type: "openai"
    network_config:
      base_url: "http://clewdr-1:8484"
      max_retries: 0
      default_request_timeout_in_seconds: 120
`)
	f, err := LoadProviders(filepath.Join(dir, "providers.yaml"))
	if err != nil {
		t.Fatalf("LoadProviders: %v", err)
	}
	if len(f.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(f.Providers))
	}
	clewdr, ok := f.Providers["clewdr-1"]
	if !ok {
		t.Fatal("missing provider clewdr-1")
	}
	if clewdr.CustomProviderConfig == nil {
		t.Fatal("CustomProviderConfig is nil")
	}
	if clewdr.CustomProviderConfig.BaseProviderType != "openai" {
		t.Errorf("BaseProviderType = %q, want %q", clewdr.CustomProviderConfig.BaseProviderType, "openai")
	}
}

func TestLoadRoutingRules(t *testing.T) {
	dir := t.TempDir()
	writeTempYAML(t, dir, "routing-rules.yaml", `
routing_rules:
  route-claude-premium:
    name: "Route Claude Premium"
    enabled: true
    cel_expression: 'request.model.startsWith("claude") && request.tier == "premium"'
    targets:
      - provider: "anthropic"
        weight: 1.0
    scope: global
    priority: 100
`)
	f, err := LoadRoutingRules(filepath.Join(dir, "routing-rules.yaml"))
	if err != nil {
		t.Fatalf("LoadRoutingRules: %v", err)
	}
	rule, ok := f.Rules["route-claude-premium"]
	if !ok {
		t.Fatal("missing rule route-claude-premium")
	}
	if rule.Name != "Route Claude Premium" {
		t.Errorf("Name = %q", rule.Name)
	}
	if !rule.Enabled {
		t.Error("expected Enabled = true")
	}
}

func TestLoadPricing(t *testing.T) {
	dir := t.TempDir()
	writeTempYAML(t, dir, "pricing.yaml", `
pricing:
  claude-sonnet:
    model_ratio: 1.5
    completion_ratio: 3.0
    cache_ratio: 0.1
  gpt-4o:
    model_price: 2.5
`)
	f, err := LoadPricing(filepath.Join(dir, "pricing.yaml"))
	if err != nil {
		t.Fatalf("LoadPricing: %v", err)
	}
	cs, ok := f.Pricing["claude-sonnet"]
	if !ok {
		t.Fatal("missing pricing claude-sonnet")
	}
	if cs.ModelRatio == nil {
		t.Fatal("ModelRatio is nil")
	}
	if *cs.ModelRatio != 1.5 {
		t.Errorf("ModelRatio = %f, want 1.5", *cs.ModelRatio)
	}
	gpt, ok := f.Pricing["gpt-4o"]
	if !ok {
		t.Fatal("missing pricing gpt-4o")
	}
	if gpt.ModelPrice == nil {
		t.Fatal("ModelPrice is nil for gpt-4o")
	}
	if gpt.ModelRatio != nil {
		t.Error("ModelRatio should be nil for gpt-4o (not specified)")
	}
}

func TestLoadAllDuplicateIDs(t *testing.T) {
	dir := t.TempDir()

	// Create a YAML file with duplicate top-level keys.
	// go.yaml.in/yaml/v3 silently overwrites duplicate map keys,
	// so we use yaml.Node decoding to detect them.
	dupContent := `
providers:
  anthropic:
    keys:
      - id: key-1
        name: "Primary"
        value_env: ANTHROPIC_API_KEY
        models: ["claude-sonnet-4-20250514"]
        weight: 1.0
  anthropic:
    keys:
      - id: key-2
        name: "Secondary"
        value_env: ANTHROPIC_API_KEY_2
        models: ["claude-sonnet-4-20250514"]
        weight: 1.0
`
	writeTempYAML(t, dir, "providers.yaml", dupContent)

	_, err := LoadProviders(filepath.Join(dir, "providers.yaml"))
	if err == nil {
		t.Fatal("expected error for duplicate provider IDs, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Errorf("error should mention 'duplicate': %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	original := LogicalModel{
		DisplayName:       "Test Model",
		BillingName:       "test-model",
		Description:       "A test model",
		UpstreamModel:     "test-upstream-v1",
		RiskLevel:         "low",
		AllowedGroups:     []string{"all", "admin"},
		Channel:           "test-channel",
		RoutePolicy:       "premium",
		UnofficialAllowed: false,
		CachingAllowed:    true,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded LogicalModel
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.DisplayName != original.DisplayName {
		t.Errorf("DisplayName mismatch: got %q, want %q", decoded.DisplayName, original.DisplayName)
	}
	if decoded.UpstreamModel != original.UpstreamModel {
		t.Errorf("UpstreamModel mismatch: got %q, want %q", decoded.UpstreamModel, original.UpstreamModel)
	}
	if decoded.CachingAllowed != original.CachingAllowed {
		t.Errorf("CachingAllowed mismatch: got %v, want %v", decoded.CachingAllowed, original.CachingAllowed)
	}
	if len(decoded.AllowedGroups) != len(original.AllowedGroups) {
		t.Errorf("AllowedGroups length mismatch: got %d, want %d", len(decoded.AllowedGroups), len(original.AllowedGroups))
	}
}
