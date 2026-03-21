# Capability Audit

This document is the authoritative reference for the actual API shapes, structs, and semantics of each upstream system. It is produced by reading source code directly and supersedes any PSEUDOCONFIG assumptions in other control-plane docs.

---

## Bifrost Admin API

Bifrost HTTP transport registers routes across four handler files: `providers.go`, `config.go`, `governance.go`, and `health.go`. All routes use fasthttp and are optionally wrapped with auth middleware. The route prefix is `/api/` (no versioning).

### Provider Management

Provider endpoints are registered in `ProviderHandler.RegisterRoutes` (providers.go:91-102).

#### ProviderResponse Struct

The canonical response shape for all provider read operations (providers.go:62-76):

```go
type ProviderResponse struct {
    Name                     schemas.ModelProvider             `json:"name"`
    Keys                     []schemas.Key                     `json:"keys"`
    NetworkConfig            schemas.NetworkConfig             `json:"network_config"`
    ConcurrencyAndBufferSize schemas.ConcurrencyAndBufferSize  `json:"concurrency_and_buffer_size"`
    ProxyConfig              *schemas.ProxyConfig              `json:"proxy_config"`
    SendBackRawRequest       bool                              `json:"send_back_raw_request"`
    SendBackRawResponse      bool                              `json:"send_back_raw_response"`
    CustomProviderConfig     *schemas.CustomProviderConfig     `json:"custom_provider_config,omitempty"`
    PricingOverrides         []schemas.ProviderPricingOverride `json:"pricing_overrides,omitempty"`
    ProviderStatus           ProviderStatus                    `json:"provider_status"`
    Status                   string                            `json:"status,omitempty"`
    Description              string                            `json:"description,omitempty"`
    ConfigHash               string                            `json:"config_hash,omitempty"`
}
```

`ProviderStatus` is a string enum: `"active"`, `"error"`, `"deleted"` (providers.go:54-59).

#### Key Struct

Each provider holds an array of `schemas.Key` (core/schemas/account.go:15-32):

```go
type Key struct {
    ID                   string                `json:"id"`
    Name                 string                `json:"name"`
    Value                EnvVar                `json:"value"`
    Models               []string              `json:"models"`
    Weight               float64               `json:"weight"`
    AzureKeyConfig       *AzureKeyConfig       `json:"azure_key_config,omitempty"`
    VertexKeyConfig      *VertexKeyConfig      `json:"vertex_key_config,omitempty"`
    BedrockKeyConfig     *BedrockKeyConfig     `json:"bedrock_key_config,omitempty"`
    HuggingFaceKeyConfig *HuggingFaceKeyConfig `json:"huggingface_key_config,omitempty"`
    ReplicateKeyConfig   *ReplicateKeyConfig   `json:"replicate_key_config,omitempty"`
    VLLMKeyConfig        *VLLMKeyConfig        `json:"vllm_key_config,omitempty"`
    Enabled              *bool                 `json:"enabled,omitempty"`
    UseForBatchAPI       *bool                 `json:"use_for_batch_api,omitempty"`
    ConfigHash           string                `json:"config_hash,omitempty"`
    Status               KeyStatusType         `json:"status,omitempty"`
    Description          string                `json:"description,omitempty"`
}
```

**Critical finding:** Keys do NOT have a per-key `base_url` field. The `base_url` is on `NetworkConfig` (per-provider, not per-key). For per-instance URLs (e.g., multiple ClewdR instances), each must be a separate custom provider with its own `NetworkConfig.BaseURL`. The `VLLMKeyConfig` has a `URL` field but that is specific to VLLM providers only.

#### NetworkConfig Struct

Per-provider network settings (core/schemas/provider.go:50-61):

```go
type NetworkConfig struct {
    BaseURL                        string            `json:"base_url,omitempty"`
    ExtraHeaders                   map[string]string `json:"extra_headers,omitempty"`
    DefaultRequestTimeoutInSeconds int               `json:"default_request_timeout_in_seconds"`
    MaxRetries                     int               `json:"max_retries"`
    RetryBackoffInitial            time.Duration     `json:"retry_backoff_initial"`
    RetryBackoffMax                time.Duration     `json:"retry_backoff_max"`
    InsecureSkipVerify             bool              `json:"insecure_skip_verify,omitempty"`
    CACertPEM                      string            `json:"ca_cert_pem,omitempty"`
    StreamIdleTimeoutInSeconds     int               `json:"stream_idle_timeout_in_seconds,omitempty"`
}
```

Note: `RetryBackoffInitial` and `RetryBackoffMax` are stored as `time.Duration` (nanoseconds) internally but serialized as **milliseconds** in JSON (custom UnmarshalJSON at provider.go:63).

#### ConcurrencyAndBufferSize Struct

```go
type ConcurrencyAndBufferSize struct {
    Concurrency int `json:"concurrency"`
    BufferSize  int `json:"buffer_size"`
}
```

#### ProxyConfig Struct

Per-provider proxy (core/schemas/provider.go:187-193):

```go
type ProxyConfig struct {
    Type      ProxyType `json:"type"`
    URL       string    `json:"url"`
    Username  string    `json:"username"`
    Password  string    `json:"password"`
    CACertPEM string    `json:"ca_cert_pem"`
}
```

#### CustomProviderConfig Struct

This is the mechanism for registering ClewdR and other non-standard providers (core/schemas/provider.go:382-388):

```go
type CustomProviderConfig struct {
    CustomProviderKey    string                 `json:"-"`
    IsKeyLess            bool                   `json:"is_key_less"`
    BaseProviderType     ModelProvider          `json:"base_provider_type"`
    AllowedRequests      *AllowedRequests       `json:"allowed_requests,omitempty"`
    RequestPathOverrides map[RequestType]string `json:"request_path_overrides,omitempty"`
}
```

**Key fields:**
- `BaseProviderType`: Must be a standard provider from `SupportedBaseProviders` (anthropic, bedrock, openai, azure, vertex, gemini, cohere, mistral, groq, ollama, openrouter, perplexity, cerebras, elevenlabs, huggingface, nebius, xai, replicate, vllm, runway, sgl, parasail).
- `IsKeyLess`: If true, no API key is required (not allowed for Bedrock).
- `AllowedRequests`: Fine-grained control over which request types the provider supports.
- `RequestPathOverrides`: Override default endpoint paths per request type (not allowed for Bedrock).
- `CustomProviderKey`: Internal field (JSON:"-"), set by Bifrost — not in request payloads.

Validation rules (providers.go:199-213):
- Custom provider name MUST NOT match any standard provider name (e.g., cannot use "openai" as a custom provider name).
- `BaseProviderType` is required when `CustomProviderConfig` is provided.
- `BaseProviderType` must be a standard provider.

#### ModelProvider Enum Values

Standard providers (core/schemas/bifrost.go:35-60):

`openai`, `azure`, `anthropic`, `bedrock`, `cohere`, `vertex`, `mistral`, `ollama`, `groq`, `sgl`, `parasail`, `perplexity`, `cerebras`, `gemini`, `openrouter`, `elevenlabs`, `huggingface`, `nebius`, `xai`, `replicate`, `vllm`, `runway`

#### GET /api/providers

- **Handler:** `listProviders` (providers.go:105)
- **Auth:** Middleware chain (session auth if auth_config enabled)
- **Response:** `ListProvidersResponse`
  ```go
  type ListProvidersResponse struct {
      Providers []ProviderResponse `json:"providers"`
      Total     int                `json:"total"`
  }
  ```
- **Notes:** Providers are sorted alphabetically by name. Keys are redacted in the response. Provider status is computed by checking whether the provider is in the active Bifrost client.

#### GET /api/providers/{provider}

- **Handler:** `getProvider` (providers.go:141)
- **Auth:** Middleware chain
- **Path param:** `provider` — the `ModelProvider` string (e.g., "anthropic", "clewdr-1")
- **Response:** Single `ProviderResponse`
- **Error:** 404 if provider not found, 400 if invalid provider name

#### POST /api/providers

- **Handler:** `addProvider` (providers.go:177)
- **Auth:** Middleware chain
- **NOTE:** Only called for adding NEW custom providers (providers.go:176 comment)
- **Request body:**
  ```go
  struct {
      Provider                 schemas.ModelProvider             `json:"provider"`
      Keys                     []schemas.Key                     `json:"keys"`
      NetworkConfig            *schemas.NetworkConfig            `json:"network_config,omitempty"`
      ConcurrencyAndBufferSize *schemas.ConcurrencyAndBufferSize `json:"concurrency_and_buffer_size,omitempty"`
      ProxyConfig              *schemas.ProxyConfig              `json:"proxy_config,omitempty"`
      SendBackRawRequest       *bool                             `json:"send_back_raw_request,omitempty"`
      SendBackRawResponse      *bool                             `json:"send_back_raw_response,omitempty"`
      CustomProviderConfig     *schemas.CustomProviderConfig     `json:"custom_provider_config,omitempty"`
      PricingOverrides         []schemas.ProviderPricingOverride `json:"pricing_overrides,omitempty"`
  }
  ```
- **Response:** `ProviderResponse` with status `"active"`
- **Error:** 409 if provider already exists, 400 if validation fails
- **Validation:**
  - `provider` field is required
  - Custom providers cannot use standard provider names
  - `BaseProviderType` required when `CustomProviderConfig` provided
  - Concurrency must be > 0 and <= BufferSize

#### PUT /api/providers/{provider}

- **Handler:** `updateProvider` (providers.go:321)
- **Auth:** Middleware chain
- **Semantics:** **FULL REPLACE** — "This endpoint expects ALL fields to be provided in the request body, including both edited and non-edited fields. Partial updates are not supported." (providers.go:317-319)
- **Request body:** Same shape as POST but without `provider` field (provider is in URL path)
- **Key merge behavior:** Keys are identified by `id` field. New keys (IDs not in existing config) are added. Existing keys are updated. Keys in old config but not in request are deleted. Redacted key values are preserved from the raw (non-redacted) config.
- **Upsert:** If provider does not exist in memory, it is created first then updated (providers.go:477-494).
- **Response:** `ProviderResponse`

#### DELETE /api/providers/{provider}

- **Handler:** `deleteProvider` (providers.go:542)
- **Auth:** Middleware chain
- **Response:** `ProviderResponse` with just the `Name` field set
- **Notes:** Removes provider from models manager. Does not return error if provider does not exist.

#### GET /api/keys

- **Handler:** `listKeys` (providers.go:567)
- **Auth:** Middleware chain
- **Response:** All keys across all providers (flat list from `GetAllKeys()`)

#### GET /api/models

- **Handler:** `listModels` (providers.go:596)
- **Auth:** Middleware chain
- **Query params:**
  - `query` — case-insensitive fuzzy filter on model name
  - `provider` — filter by specific provider
  - `keys` — comma-separated key IDs to filter by key access
  - `limit` — max results (default: 5)
  - `unfiltered` — if "true", returns all models including filtered ones
- **Response:**
  ```go
  type ListModelsResponse struct {
      Models []ModelResponse `json:"models"`
      Total  int             `json:"total"`
  }
  type ModelResponse struct {
      Name             string   `json:"name"`
      Provider         string   `json:"provider"`
      AccessibleByKeys []string `json:"accessible_by_keys,omitempty"`
  }
  ```

#### GET /api/models/parameters

- **Handler:** `getModelParameters` (providers.go:709)
- **Query param:** `model` (required) — model name
- **Response:** Raw JSON string from `GetModelParameters()` — the model's parameter schema

#### GET /api/models/base

- **Handler:** `listBaseModels` (providers.go:795)
- **Query params:** `query` (filter), `limit` (default: 20)
- **Response:**
  ```go
  type ListBaseModelsResponse struct {
      Models []string `json:"models"`
      Total  int      `json:"total"`
  }
  ```

---

### Config API

Config endpoints are registered in `ConfigHandler.RegisterRoutes` (config.go:75-82).

#### GET /api/config

- **Handler:** `getConfig` (config.go:90)
- **Auth:** Middleware chain
- **Query param:** `from_db=true` — if set, reads from database instead of in-memory store
- **Response:** Map with these top-level keys:
  ```json
  {
    "client_config": { ... ClientConfig fields ... },
    "framework_config": { "pricing_url": "...", "pricing_sync_interval": 3600 },
    "auth_config": {
      "admin_username": { "val": "", "env_var": "", "from_env": false },
      "admin_password": { "val": "<redacted>", "env_var": "", "from_env": false },
      "is_enabled": false,
      "disable_auth_on_inference": false
    },
    "is_db_connected": true,
    "is_cache_connected": true,
    "is_logs_connected": true,
    "proxy_config": { ... GlobalProxyConfig ... },
    "restart_required": { "required": false, "reason": "" }
  }
  ```

#### PUT /api/config — Merge vs Replace Semantics

- **Handler:** `updateConfig` (config.go:200)
- **Auth:** Middleware chain

**CRITICAL FINDING — FIELD-LEVEL MERGE, NOT FULL REPLACE:**

The `updateConfig` handler does NOT replace the entire config. It performs **field-by-field comparison and selective update**. Evidence from config.go:242-400:

1. Reads current in-memory `ClientConfig` (line 242: `currentConfig := h.store.ClientConfig`)
2. Creates `updatedConfig` as a copy of current (line 243: `updatedConfig := currentConfig`)
3. Compares each field individually:
   - `DropExcessRequests`: Updated only if different from current (line 247)
   - `InitialPoolSize`: Updated only if > 0 and different (line 304)
   - `EnableLogging`: Always applied (line 314)
   - `MCPAgentDepth`: Updated only if > 0 and different (line 263)
   - `PrometheusLabels`, `AllowedOrigins`, `AllowedHeaders`: Updated only if arrays differ; triggers restart flag (lines 288-301)
4. Some fields trigger `restartReasons` list — changes that require server restart
5. Framework config is fetched separately and compared field-by-field (lines 416-486)
6. Auth config is compared with existing and only updated if changed (lines 488-603)
7. Final write: `h.store.ConfigStore.UpdateClientConfig(ctx, &updatedConfig)` (line 405)

**Request body structure:**
```go
struct {
    ClientConfig    configstore.ClientConfig               `json:"client_config"`
    FrameworkConfig configstoreTables.TableFrameworkConfig `json:"framework_config"`
    AuthConfig      *configstore.AuthConfig                `json:"auth_config"`
}
```

**ClientConfig struct** (framework/configstore/clientconfig.go:38-64):
```go
type ClientConfig struct {
    DropExcessRequests              bool     `json:"drop_excess_requests"`
    InitialPoolSize                 int      `json:"initial_pool_size"`
    PrometheusLabels                []string `json:"prometheus_labels"`
    EnableLogging                   bool     `json:"enable_logging"`
    DisableContentLogging           bool     `json:"disable_content_logging"`
    DisableDBPingsInHealth          bool     `json:"disable_db_pings_in_health"`
    LogRetentionDays                int      `json:"log_retention_days"`
    EnforceAuthOnInference          bool     `json:"enforce_auth_on_inference"`
    AllowDirectKeys                 bool     `json:"allow_direct_keys"`
    AllowedOrigins                  []string `json:"allowed_origins,omitempty"`
    AllowedHeaders                  []string `json:"allowed_headers,omitempty"`
    MaxRequestBodySizeMB            int      `json:"max_request_body_size_mb"`
    EnableLiteLLMFallbacks          bool     `json:"enable_litellm_fallbacks"`
    MCPAgentDepth                   int      `json:"mcp_agent_depth"`
    MCPToolExecutionTimeout         int      `json:"mcp_tool_execution_timeout"`
    MCPCodeModeBindingLevel         string   `json:"mcp_code_mode_binding_level"`
    MCPToolSyncInterval             int      `json:"mcp_tool_sync_interval"`
    HeaderFilterConfig              *GlobalHeaderFilterConfig `json:"header_filter_config,omitempty"`
    AsyncJobResultTTL               int      `json:"async_job_result_ttl"`
    RequiredHeaders                 []string `json:"required_headers,omitempty"`
    LoggingHeaders                  []string `json:"logging_headers,omitempty"`
    HideDeletedVirtualKeysInFilters bool     `json:"hide_deleted_virtual_keys_in_filters"`
}
```

**Sync engine implication:** The sync engine CAN safely call `PUT /api/config` with the full desired state because the handler merges field-by-field. Zero-value fields (int 0, bool false, empty slices) are handled with "only update if explicitly provided (> 0)" guards for numeric fields. The sync engine should always send the complete config to avoid confusion.

#### GET /api/version

- **Handler:** `getVersion` (config.go:85)
- **Response:** Plain string (the version set via `SetVersion()`)

#### POST /api/pricing/force-sync

- **Handler:** `forceSyncPricing` (config.go:624)
- **Response:** `{"status": "success", "message": "pricing sync triggered"}`
- **Notes:** Triggers immediate pricing data reload from the configured pricing URL

#### GET /api/proxy-config

- **Handler:** `getProxyConfig` (config.go:644)
- **Response:** `GlobalProxyConfig` struct with password redacted
  ```json
  {
    "enabled": false,
    "type": "http",
    "url": "",
    "username": "",
    "password": "<redacted>",
    "no_proxy": "",
    "timeout": 0,
    "skip_tls_verify": false
  }
  ```

#### PUT /api/proxy-config

- **Handler:** `updateProxyConfig` (config.go:670)
- **Request body:** `GlobalProxyConfig` — full replace
- **Validation:** URL required when enabled, only HTTP type currently supported
- **Notes:** Triggers restart_required flag. Redacted password values are preserved from existing config.

---

### Health API

#### GET /health

- **Handler:** `getHealth` (health.go:32)
- **Response:** `{"status": "ok", "components": {"db_pings": "ok"}}` or `{"db_pings": "disabled"}`
- **Notes:** Pings config store, log store, and vector store concurrently with 10s timeout. Returns 503 if any store is unavailable. DB pings can be disabled via `DisableDBPingsInHealth` in ClientConfig.
