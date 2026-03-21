// Package registry defines desired-state structs for the control plane.
// These structs represent what the operator declares in YAML policy files.
// They are the typed foundation for sync (Phase 3), drift detection (Phase 4),
// and bootstrap (Phase 5).
package registry

// ModelsFile is the top-level wrapper for models.yaml / logical-models.example.yaml.
type ModelsFile struct {
	Models map[string]LogicalModel `yaml:"models" json:"models"`
}

// RoutePoliciesFile is the top-level wrapper for route-policies.yaml.
type RoutePoliciesFile struct {
	Policies map[string]RoutePolicy `yaml:"policies" json:"policies"`
}

// ProviderPoolsFile is the top-level wrapper for provider-pools.yaml.
type ProviderPoolsFile struct {
	Pools map[string]ProviderPool `yaml:"pools" json:"pools"`
}

// LogicalModel is control-plane-only metadata mapping logical names to upstreams.
type LogicalModel struct {
	DisplayName       string   `yaml:"display_name" json:"display_name"`
	BillingName       string   `yaml:"billing_name" json:"billing_name"`
	Description       string   `yaml:"description,omitempty" json:"description,omitempty"`
	UpstreamModel     string   `yaml:"upstream_model" json:"upstream_model"`
	RiskLevel         string   `yaml:"risk_level" json:"risk_level"`
	AllowedGroups     []string `yaml:"allowed_groups" json:"allowed_groups"`
	Channel           string   `yaml:"channel" json:"channel"`
	RoutePolicy       string   `yaml:"route_policy" json:"route_policy"`
	UnofficialAllowed bool     `yaml:"unofficial_allowed" json:"unofficial_allowed"`
	CachingAllowed    bool     `yaml:"caching_allowed" json:"caching_allowed"`
}

// RoutePolicy is a control-plane routing policy (translated to Bifrost routing rules at sync time).
type RoutePolicy struct {
	Description       string   `yaml:"description,omitempty" json:"description,omitempty"`
	PoolChain         []string `yaml:"pool_chain" json:"pool_chain"`
	FallbackBehavior  string   `yaml:"fallback_behavior" json:"fallback_behavior"`
	MaxRetries        int      `yaml:"max_retries" json:"max_retries"`
	TimeoutSeconds    int      `yaml:"timeout_seconds" json:"timeout_seconds"`
	CachingAllowed    bool     `yaml:"caching_allowed" json:"caching_allowed"`
	UnofficialAllowed bool     `yaml:"unofficial_allowed" json:"unofficial_allowed"`
}

// ProviderPool is a control-plane grouping of Bifrost providers.
type ProviderPool struct {
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	TrustLevel  string              `yaml:"trust_level" json:"trust_level"`
	Providers   []PoolProviderEntry `yaml:"providers" json:"providers"`
}

// PoolProviderEntry is a provider within a pool (references a BifrostProviderDesired by ID).
type PoolProviderEntry struct {
	ID            string             `yaml:"id" json:"id"`
	Type          string             `yaml:"type" json:"type"`
	BaseURL       string             `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	KeyEnv        string             `yaml:"key_env,omitempty" json:"key_env,omitempty"`
	Models        []string           `yaml:"models" json:"models"`
	Weight        float64            `yaml:"weight" json:"weight"`
	Enabled       bool               `yaml:"enabled" json:"enabled"`
	NetworkConfig *PoolNetworkConfig `yaml:"network_config,omitempty" json:"network_config,omitempty"`
}

// PoolNetworkConfig holds network settings for a provider within a pool.
type PoolNetworkConfig struct {
	MaxRetries                     int `yaml:"max_retries" json:"max_retries"`
	DefaultRequestTimeoutInSeconds int `yaml:"default_request_timeout_in_seconds" json:"default_request_timeout_in_seconds"`
	RetryBackoffInitialMs          int `yaml:"retry_backoff_initial" json:"retry_backoff_initial"`
	RetryBackoffMaxMs              int `yaml:"retry_backoff_max" json:"retry_backoff_max"`
	StreamIdleTimeoutInSeconds     int `yaml:"stream_idle_timeout_in_seconds,omitempty" json:"stream_idle_timeout_in_seconds,omitempty"`
}
