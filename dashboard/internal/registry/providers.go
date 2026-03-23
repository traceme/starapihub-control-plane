package registry

// ProvidersFile is the top-level YAML wrapper for providers.yaml.
type ProvidersFile struct {
	Providers map[string]BifrostProviderDesired `yaml:"providers" json:"providers"`
	Config    *BifrostClientConfig              `yaml:"config,omitempty" json:"config,omitempty"`
}

// BifrostProviderDesired is the desired state for a Bifrost provider.
// Derived from capability-audit.md ProviderResponse struct.
type BifrostProviderDesired struct {
	Keys                     []BifrostKeyDesired       `yaml:"keys" json:"keys"`
	NetworkConfig            *BifrostNetworkConfig     `yaml:"network_config,omitempty" json:"network_config,omitempty"`
	ConcurrencyAndBufferSize *ConcurrencyAndBufferSize `yaml:"concurrency_and_buffer_size,omitempty" json:"concurrency_and_buffer_size,omitempty"`
	CustomProviderConfig     *CustomProviderConfig     `yaml:"custom_provider_config,omitempty" json:"custom_provider_config,omitempty"`
	Description              string                    `yaml:"description,omitempty" json:"description,omitempty"`
}

// BifrostKeyDesired is a key within a Bifrost provider.
type BifrostKeyDesired struct {
	ID               string            `yaml:"id" json:"id"`
	Name             string            `yaml:"name" json:"name"`
	ValueEnv         string            `yaml:"value_env" json:"-"`
	Models           []string          `yaml:"models" json:"models"`
	Weight           float64           `yaml:"weight" json:"weight"`
	Enabled          *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	BedrockKeyConfig *BedrockKeyConfig `yaml:"bedrock_key_config,omitempty" json:"bedrock_key_config,omitempty"`
	Description      string            `yaml:"description,omitempty" json:"description,omitempty"`
}

// BedrockKeyConfig holds AWS Bedrock-specific key configuration.
type BedrockKeyConfig struct {
	AwsAccessKeyEnv string `yaml:"aws_access_key_env" json:"-"`
	AwsSecretKeyEnv string `yaml:"aws_secret_key_env" json:"-"`
	AwsRegion       string `yaml:"aws_region" json:"aws_region"`
}

// BifrostNetworkConfig holds network settings for a Bifrost provider.
type BifrostNetworkConfig struct {
	BaseURL                        string            `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	ExtraHeaders                   map[string]string `yaml:"extra_headers,omitempty" json:"extra_headers,omitempty"`
	DefaultRequestTimeoutInSeconds int               `yaml:"default_request_timeout_in_seconds" json:"default_request_timeout_in_seconds"`
	MaxRetries                     int               `yaml:"max_retries" json:"max_retries"`
	RetryBackoffInitialMs          int               `yaml:"retry_backoff_initial_ms" json:"retry_backoff_initial"`
	RetryBackoffMaxMs              int               `yaml:"retry_backoff_max_ms" json:"retry_backoff_max"`
	StreamIdleTimeoutInSeconds     int               `yaml:"stream_idle_timeout_in_seconds,omitempty" json:"stream_idle_timeout_in_seconds,omitempty"`
}

// ConcurrencyAndBufferSize holds concurrency settings for a Bifrost provider.
type ConcurrencyAndBufferSize struct {
	Concurrency int `yaml:"concurrency" json:"concurrency"`
	BufferSize  int `yaml:"buffer_size" json:"buffer_size"`
}

// CustomProviderConfig holds configuration for custom providers (e.g., ClewdR).
type CustomProviderConfig struct {
	IsKeyLess        bool   `yaml:"is_key_less" json:"is_key_less"`
	BaseProviderType string `yaml:"base_provider_type" json:"base_provider_type"`
}

// BifrostClientConfig is the desired state for Bifrost global config (PUT /api/config).
// Field-level merge semantics: Bifrost only updates non-zero fields.
type BifrostClientConfig struct {
	MaxRetries                     *int    `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryBackoffInitialMs          *int    `yaml:"retry_backoff_initial_ms,omitempty" json:"retry_backoff_initial,omitempty"`
	RetryBackoffMaxMs              *int    `yaml:"retry_backoff_max_ms,omitempty" json:"retry_backoff_max,omitempty"`
	DefaultRequestTimeoutInSeconds *int    `yaml:"default_request_timeout_in_seconds,omitempty" json:"default_request_timeout_in_seconds,omitempty"`
	StreamIdleTimeoutInSeconds     *int    `yaml:"stream_idle_timeout_in_seconds,omitempty" json:"stream_idle_timeout_in_seconds,omitempty"`
	InitialPoolSize                *int    `yaml:"initial_pool_size,omitempty" json:"initial_pool_size,omitempty"`
	MaxIdleConnsPerHost            *int    `yaml:"max_idle_conns_per_host,omitempty" json:"max_idle_conns_per_host,omitempty"`
	ProxyURL                       *string `yaml:"proxy_url,omitempty" json:"proxy_url,omitempty"`
}
