package upstream

// ChannelResponse matches New-API Channel JSON response (GET /api/channel/:id).
// Fields aligned with capability-audit.md and new-api/model/channel.go.
type ChannelResponse struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Type           int      `json:"type"`
	BaseURL        *string  `json:"base_url"`
	Models         string   `json:"models"`
	Group          string   `json:"group"`
	Tag            *string  `json:"tag"`
	ModelMapping   *string  `json:"model_mapping"`
	Priority       *int64   `json:"priority"`
	Weight         *uint    `json:"weight"`
	Status         int      `json:"status"`
	AutoBan        *int     `json:"auto_ban"`
	Setting        *string  `json:"setting"`
	ParamOverride  *string  `json:"param_override"`
	HeaderOverride *string  `json:"header_override"`
	UsedQuota      int64    `json:"used_quota"`
	CreatedTime    int64    `json:"created_time"`
	TestTime       int64    `json:"test_time"`
	ResponseTime   int      `json:"response_time"`
	Balance        float64  `json:"balance"`
}

// ChannelListResponse is the paginated response from GET /api/channel/.
type ChannelListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Items    []ChannelResponse `json:"items"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
	} `json:"data"`
}

// SingleChannelResponse wraps GET /api/channel/:id.
type SingleChannelResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    ChannelResponse `json:"data"`
}

// OptionEntry is a single key-value from GET /api/option/.
type OptionEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// OptionListResponse wraps GET /api/option/.
type OptionListResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Data    []OptionEntry `json:"data"`
}

// BifrostProviderResponse matches Bifrost provider JSON (GET /api/providers response values).
type BifrostProviderResponse struct {
	Keys                     []BifrostKeyResponse          `json:"keys"`
	NetworkConfig            *BifrostNetworkConfigResponse `json:"network_config,omitempty"`
	ConcurrencyAndBufferSize *ConcurrencyBufferResponse    `json:"concurrency_and_buffer_size,omitempty"`
	CustomProviderConfig     *CustomProviderConfigResponse `json:"custom_provider_config,omitempty"`
}

// BifrostKeyResponse is a key within a Bifrost provider response.
type BifrostKeyResponse struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Value            string                `json:"value"`
	Models           []string              `json:"models"`
	Weight           float64               `json:"weight"`
	Enabled          *bool                 `json:"enabled,omitempty"`
	BedrockKeyConfig *BedrockKeyConfigResp `json:"bedrock_key_config,omitempty"`
	Description      string                `json:"description,omitempty"`
}

// BedrockKeyConfigResp holds AWS Bedrock key config from API response.
type BedrockKeyConfigResp struct {
	AwsAccessKey string `json:"aws_access_key"`
	AwsSecretKey string `json:"aws_secret_key"`
	AwsRegion    string `json:"aws_region"`
}

// BifrostNetworkConfigResponse holds network config from API response.
type BifrostNetworkConfigResponse struct {
	BaseURL                        string            `json:"base_url,omitempty"`
	ExtraHeaders                   map[string]string `json:"extra_headers,omitempty"`
	DefaultRequestTimeoutInSeconds int               `json:"default_request_timeout_in_seconds"`
	MaxRetries                     int               `json:"max_retries"`
	RetryBackoffInitialMs          int               `json:"retry_backoff_initial"`
	RetryBackoffMaxMs              int               `json:"retry_backoff_max"`
	StreamIdleTimeoutInSeconds     int               `json:"stream_idle_timeout_in_seconds,omitempty"`
}

// ConcurrencyBufferResponse holds concurrency settings from API response.
type ConcurrencyBufferResponse struct {
	Concurrency int `json:"concurrency"`
	BufferSize  int `json:"buffer_size"`
}

// CustomProviderConfigResponse holds custom provider config from API response.
type CustomProviderConfigResponse struct {
	IsKeyLess        bool   `json:"is_key_less"`
	BaseProviderType string `json:"base_provider_type"`
}

// BifrostProvidersMapResponse wraps GET /api/providers (map of provider ID to provider).
type BifrostProvidersMapResponse map[string]BifrostProviderResponse

// BifrostConfigResponse matches GET /api/config.
type BifrostConfigResponse struct {
	MaxRetries                     *int    `json:"max_retries,omitempty"`
	RetryBackoffInitialMs          *int    `json:"retry_backoff_initial,omitempty"`
	RetryBackoffMaxMs              *int    `json:"retry_backoff_max,omitempty"`
	DefaultRequestTimeoutInSeconds *int    `json:"default_request_timeout_in_seconds,omitempty"`
	StreamIdleTimeoutInSeconds     *int    `json:"stream_idle_timeout_in_seconds,omitempty"`
	InitialPoolSize                *int    `json:"initial_pool_size,omitempty"`
	MaxIdleConnsPerHost            *int    `json:"max_idle_conns_per_host,omitempty"`
	ProxyURL                       *string `json:"proxy_url,omitempty"`
}

// BifrostRoutingRuleResponse matches Bifrost routing rule JSON.
type BifrostRoutingRuleResponse struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description,omitempty"`
	Enabled       bool                       `json:"enabled"`
	CelExpression string                     `json:"cel_expression"`
	Targets       []BifrostRoutingTargetResp `json:"targets,omitempty"`
	Fallbacks     []string                   `json:"fallbacks,omitempty"`
	Query         map[string]any             `json:"query,omitempty"`
	Scope         string                     `json:"scope"`
	ScopeID       *string                    `json:"scope_id,omitempty"`
	Priority      int                        `json:"priority"`
}

// BifrostRoutingTargetResp is a target within a routing rule response.
type BifrostRoutingTargetResp struct {
	Provider *string `json:"provider,omitempty"`
	Model    *string `json:"model,omitempty"`
	KeyID    *string `json:"key_id,omitempty"`
	Weight   float64 `json:"weight"`
}

// CookieStatusTyped is the typed version of a ClewdR cookie entry.
type CookieStatusTyped struct {
	Cookie string `json:"cookie"`
}

// CookieResponseTyped replaces the json.RawMessage CookieResponse for typed usage.
type CookieResponseTyped struct {
	Valid     []CookieStatusTyped `json:"valid"`
	Exhausted []CookieStatusTyped `json:"exhausted"`
	Invalid   []CookieStatusTyped `json:"invalid"`
}
