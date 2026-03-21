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

---

### Governance API

Governance endpoints are registered in `GovernanceHandler.RegisterRoutes` (governance.go:255-299). The governance system provides hierarchical access control: Customers -> Teams -> Virtual Keys, with budgets and rate limits at each level. Routing rules provide request-level routing with CEL expressions.

#### Routing Rules

Routing rules use CEL (Common Expression Language) for matching and weighted targets for routing decisions.

##### TableRoutingRule Struct

The DB/response model (configstore/tables/routing_rules.go:13-38):

```go
type TableRoutingRule struct {
    ID              string                `json:"id"`
    ConfigHash      string                `json:"config_hash"`
    Name            string                `json:"name"`
    Description     string                `json:"description"`
    Enabled         bool                  `json:"enabled"`
    CelExpression   string                `json:"cel_expression"`
    Targets         []TableRoutingTarget  `json:"targets,omitempty"`
    Fallbacks       *string               `json:"-"`
    ParsedFallbacks []string              `json:"fallbacks,omitempty"`
    Query           *string               `json:"-"`
    ParsedQuery     map[string]any        `json:"query,omitempty"`
    Scope           string                `json:"scope"`
    ScopeID         *string               `json:"scope_id"`
    Priority        int                   `json:"priority"`
    CreatedAt       time.Time             `json:"created_at"`
}
```

##### TableRoutingTarget Struct

```go
type TableRoutingTarget struct {
    RuleID   string  `json:"-"`
    Provider *string `json:"provider,omitempty"`
    Model    *string `json:"model,omitempty"`
    KeyID    *string `json:"key_id,omitempty"`
    Weight   float64 `json:"weight"`
}
```

**Key semantics:**
- Rules reference providers by **name** (the `ModelProvider` string, e.g., "anthropic", "clewdr-1")
- Model matching uses **CEL expressions** (`cel_expression` field) — not prefix/regex built-in
- Load balancing uses **target weights** that must sum to 1.0 across all targets in a rule
- Scope values: `"global"`, `"team"`, `"customer"`, `"virtual_key"`
- Priority: lower = evaluated first within scope (default 0)
- Fallbacks: array of provider name strings for fallback chain
- `scope_id` is required for non-global scopes; must be nil for global

##### GET /api/governance/routing-rules

- **Handler:** `getRoutingRules` (governance.go:2806)
- **Query params:**
  - `scope` — filter by scope type
  - `scope_id` — filter by scope entity ID
  - `from_memory=true` — read from in-memory store instead of DB
  - `limit`, `offset`, `search` — pagination params
- **Response:**
  ```json
  {
    "rules": [ ...TableRoutingRule... ],
    "count": 5,
    "total_count": 50,
    "limit": 10,
    "offset": 0
  }
  ```

##### GET /api/governance/routing-rules/{rule_id}

- **Handler:** `getRoutingRule` (governance.go:2939)
- **Response:** Single `TableRoutingRule`

##### POST /api/governance/routing-rules

- **Handler:** `createRoutingRule` (governance.go:2985)
- **Request body:** `CreateRoutingRuleRequest`
  ```go
  type CreateRoutingRuleRequest struct {
      Name          string          `json:"name"`
      Description   string          `json:"description,omitempty"`
      Enabled       *bool           `json:"enabled,omitempty"`
      CelExpression string          `json:"cel_expression"`
      Targets       []RoutingTarget `json:"targets"`
      Fallbacks     []string        `json:"fallbacks,omitempty"`
      Scope         string          `json:"scope,omitempty"`
      ScopeID       *string         `json:"scope_id,omitempty"`
      Query         map[string]any  `json:"query,omitempty"`
      Priority      int             `json:"priority,omitempty"`
  }
  ```
- **Validation:** Name required, at least one target required, weights must sum to 1
- **ID generation:** UUIDs generated server-side (governance.go:3030)
- **Response:** `{"message": "...", "rule": ...TableRoutingRule...}`

##### PUT /api/governance/routing-rules/{rule_id}

- **Handler:** `updateRoutingRule` (governance.go:3080)
- **Request body:** `UpdateRoutingRuleRequest` — partial update (all fields optional via pointers)
  ```go
  type UpdateRoutingRuleRequest struct {
      Name          *string         `json:"name,omitempty"`
      Description   *string         `json:"description,omitempty"`
      Enabled       *bool           `json:"enabled,omitempty"`
      CelExpression *string         `json:"cel_expression,omitempty"`
      Targets       []RoutingTarget `json:"targets,omitempty"`
      Fallbacks     []string        `json:"fallbacks,omitempty"`
      Query         map[string]any  `json:"query,omitempty"`
      Priority      *int            `json:"priority,omitempty"`
      Scope         *string         `json:"scope,omitempty"`
      ScopeID       *string         `json:"scope_id,omitempty"`
  }
  ```
- **Notes:** If targets provided, replaces all existing targets (weights must sum to 1)

##### DELETE /api/governance/routing-rules/{rule_id}

- **Handler:** `deleteRoutingRule` (governance.go:3182)

---

#### Virtual Keys

Virtual keys provide per-consumer access control with provider restrictions, budgets, and rate limits.

##### TableVirtualKey Struct

```go
type TableVirtualKey struct {
    ID              string                          `json:"id"`
    Name            string                          `json:"name"`
    Description     string                          `json:"description,omitempty"`
    Value           string                          `json:"value"`
    IsActive        bool                            `json:"is_active"`
    ProviderConfigs []TableVirtualKeyProviderConfig `json:"provider_configs"`
    MCPConfigs      []TableVirtualKeyMCPConfig      `json:"mcp_configs"`
    TeamID          *string                         `json:"team_id,omitempty"`
    CustomerID      *string                         `json:"customer_id,omitempty"`
    BudgetID        *string                         `json:"budget_id,omitempty"`
    RateLimitID     *string                         `json:"rate_limit_id,omitempty"`
    Team            *TableTeam                      `json:"team,omitempty"`
    Customer        *TableCustomer                  `json:"customer,omitempty"`
    Budget          *TableBudget                    `json:"budget,omitempty"`
    RateLimit       *TableRateLimit                 `json:"rate_limit,omitempty"`
    ConfigHash      string                          `json:"config_hash"`
}
```

**Key semantics:**
- Virtual keys are scoped to either a team OR a customer (mutually exclusive)
- Provider configs restrict which providers (and models within providers) the key can access
- Empty `provider_configs` = all providers allowed
- Each provider config can have its own budget and rate limit
- VK value is auto-generated (UUID-based)

##### GET /api/governance/virtual-keys

- **Handler:** `getVirtualKeys` (governance.go:304)
- **Query params:** `from_memory=true`
- **Response:** Array of `TableVirtualKey` with all relationships

##### POST /api/governance/virtual-keys

- **Handler:** `createVirtualKey` (governance.go)
- **Request body:** `CreateVirtualKeyRequest`
  ```go
  type CreateVirtualKeyRequest struct {
      Name            string `json:"name"`
      Description     string `json:"description,omitempty"`
      ProviderConfigs []struct {
          Provider      string                  `json:"provider"`
          Weight        float64                 `json:"weight,omitempty"`
          AllowedModels []string                `json:"allowed_models,omitempty"`
          Budget        *CreateBudgetRequest    `json:"budget,omitempty"`
          RateLimit     *CreateRateLimitRequest `json:"rate_limit,omitempty"`
          KeyIDs        []string                `json:"key_ids,omitempty"`
      } `json:"provider_configs,omitempty"`
      MCPConfigs []struct {
          MCPClientName  string   `json:"mcp_client_name"`
          ToolsToExecute []string `json:"tools_to_execute,omitempty"`
      } `json:"mcp_configs,omitempty"`
      TeamID     *string                 `json:"team_id,omitempty"`
      CustomerID *string                 `json:"customer_id,omitempty"`
      Budget     *CreateBudgetRequest    `json:"budget,omitempty"`
      RateLimit  *CreateRateLimitRequest `json:"rate_limit,omitempty"`
      IsActive   *bool                   `json:"is_active,omitempty"`
  }
  ```

##### GET /api/governance/virtual-keys/{vk_id}

- **Handler:** `getVirtualKey` (governance.go)
- **Response:** Single `TableVirtualKey`

##### PUT /api/governance/virtual-keys/{vk_id}

- **Handler:** `updateVirtualKey` (governance.go)
- **Request body:** `UpdateVirtualKeyRequest` — partial update

##### DELETE /api/governance/virtual-keys/{vk_id}

- **Handler:** `deleteVirtualKey` (governance.go)

---

#### Model Configs

Per-model budget and rate limit controls.

##### TableModelConfig Struct

```go
type TableModelConfig struct {
    ID          string          `json:"id"`
    ModelName   string          `json:"model_name"`
    Provider    *string         `json:"provider,omitempty"`
    BudgetID    *string         `json:"budget_id,omitempty"`
    RateLimitID *string         `json:"rate_limit_id,omitempty"`
    Budget      *TableBudget    `json:"budget,omitempty"`
    RateLimit   *TableRateLimit `json:"rate_limit,omitempty"`
    ConfigHash  string          `json:"config_hash"`
}
```

##### Endpoints

- `GET /api/governance/model-configs` — list all model configs
- `POST /api/governance/model-configs` — create (CreateModelConfigRequest: `model_name` required, optional `provider`, `budget`, `rate_limit`)
- `GET /api/governance/model-configs/{mc_id}` — get single
- `PUT /api/governance/model-configs/{mc_id}` — update (UpdateModelConfigRequest: all fields optional)
- `DELETE /api/governance/model-configs/{mc_id}` — delete

---

#### Provider Governance

Budget and rate limit controls at the provider level. This is separate from `/api/providers` — it manages governance policies attached to providers, not the provider configuration itself.

##### Endpoints

- `GET /api/governance/providers` — list providers with governance (only providers with budget or rate limit are returned)
  - **Response:** `{"providers": [...ProviderGovernanceResponse...], "count": N}`
  - **ProviderGovernanceResponse:** `{"provider": "name", "budget": {...}, "rate_limit": {...}}`
- `PUT /api/governance/providers/{provider_name}` — update budget/rate limit for a provider
  - **Request:** `UpdateProviderGovernanceRequest` with optional `budget` and `rate_limit`
- `DELETE /api/governance/providers/{provider_name}` — remove governance from a provider

---

#### Teams and Customers

Organizational hierarchy for virtual key management.

##### TableTeam Struct

```go
type TableTeam struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    CustomerID  *string                `json:"customer_id,omitempty"`
    BudgetID    *string                `json:"budget_id,omitempty"`
    RateLimitID *string                `json:"rate_limit_id,omitempty"`
    Customer    *TableCustomer         `json:"customer,omitempty"`
    Budget      *TableBudget           `json:"budget,omitempty"`
    RateLimit   *TableRateLimit        `json:"rate_limit,omitempty"`
    VirtualKeys []TableVirtualKey      `json:"virtual_keys"`
    Profile     *string                `json:"-"`
    ParsedProfile map[string]interface{} `json:"profile"`
}
```

##### TableCustomer Struct

```go
type TableCustomer struct {
    ID          string             `json:"id"`
    Name        string             `json:"name"`
    BudgetID    *string            `json:"budget_id,omitempty"`
    RateLimitID *string            `json:"rate_limit_id,omitempty"`
    Budget      *TableBudget       `json:"budget,omitempty"`
    RateLimit   *TableRateLimit    `json:"rate_limit,omitempty"`
    Teams       []TableTeam        `json:"teams"`
    VirtualKeys []TableVirtualKey  `json:"virtual_keys"`
}
```

##### Team Endpoints

- `GET /api/governance/teams` — list all teams
- `POST /api/governance/teams` — create (name required, optional customer_id, budget, rate_limit)
- `GET /api/governance/teams/{team_id}` — get single
- `PUT /api/governance/teams/{team_id}` — update
- `DELETE /api/governance/teams/{team_id}` — delete

##### Customer Endpoints

- `GET /api/governance/customers` — list all customers
- `POST /api/governance/customers` — create (name required, optional budget, rate_limit)
- `GET /api/governance/customers/{customer_id}` — get single
- `PUT /api/governance/customers/{customer_id}` — update
- `DELETE /api/governance/customers/{customer_id}` — delete

---

#### Budget and Rate Limit Read Endpoints

- `GET /api/governance/budgets` — list all budgets (governance.go:1989)
- `GET /api/governance/rate-limits` — list all rate limits (governance.go:2017)

Budget request shape:
```go
type CreateBudgetRequest struct {
    MaxLimit      float64 `json:"max_limit"`
    ResetDuration string  `json:"reset_duration"`  // e.g., "30s", "5m", "1h", "1d", "1w", "1M"
}
```

Rate limit request shape:
```go
type CreateRateLimitRequest struct {
    TokenMaxLimit        *int64  `json:"token_max_limit,omitempty"`
    TokenResetDuration   *string `json:"token_reset_duration,omitempty"`
    RequestMaxLimit      *int64  `json:"request_max_limit,omitempty"`
    RequestResetDuration *string `json:"request_reset_duration,omitempty"`
}
```

---

### PSEUDOCONFIG Resolution Table

This table resolves every PSEUDOCONFIG label found in the control-plane codebase against actual Bifrost, New-API, and ClewdR source code.

| # | File:Line | Current Assumption | Verified Reality | Evidence | Status |
|---|---|---|---|---|---|
| 1 | config.example.json:4 | "Bifrost config.json template" — providers section is a JSON object keyed by provider name | **CONFIRMED** — Bifrost config.json `providers` is an object keyed by `ModelProvider` string. The `configstore.GetProvidersConfig` returns `map[schemas.ModelProvider]*ProviderConfig`. | providers.go:107, configstore types | CONFIRMED CORRECT |
| 2 | config.example.json:8 | "Fields may need adjustment based on Bifrost version" — key fields: `id`, `name`, `value`, `models`, `weight`, `enabled` | **CONFIRMED** — All six fields match the `schemas.Key` struct JSON tags exactly. Additional fields exist (`azure_key_config`, `status`, `description`, etc.) but the documented ones are correct. | core/schemas/account.go:15-32 | CONFIRMED CORRECT |
| 3 | config.example.json:85 | Routing rule example with `id`, `name`, `enabled`, `cel_expression`, `targets`, `fallbacks`, `scope`, `priority` | **CORRECTED** — The `id` field is not user-settable in the API (UUIDs auto-generated by server). Config.json loading may accept it, but API calls ignore it. Target shape confirmed. `scope_id` field is missing from example but is part of the schema. | governance.go:3030, tables/routing_rules.go:13-38 | CORRECTED: remove `id` from manual config; add `scope_id` field |
| 4 | bifrost-integration.md:130 | "Custom provider approach for ClewdR" — shows ClewdR under `"openai"` key with `custom_provider_config` | **CORRECTED** — Custom providers MUST use a unique name (not "openai"). Each ClewdR instance should be registered as a separate custom provider like `"clewdr-1"`, `"clewdr-2"` with `custom_provider_config.base_provider_type: "openai"`. The validation explicitly rejects custom providers with standard provider names. | providers.go:200-203 (`IsStandardProvider` check) | CORRECTED: use unique names like "clewdr-1", not "openai" |
| 5 | bifrost-integration.md:155 | `custom_provider_config: { base_provider: "openai" }` | **CORRECTED** — The correct field name is `base_provider_type` (not `base_provider`). Full struct: `{"is_key_less": false, "base_provider_type": "openai"}`. Also supports `allowed_requests` and `request_path_overrides`. | core/schemas/provider.go:382-388 | CORRECTED: field is `base_provider_type`, not `base_provider` |
| 6 | provider-pools.example.yaml:54 | Health check with `method: passive`, `failure_threshold`, `recovery_period_seconds` | **CORRECTED** — Bifrost has NO per-provider health check configuration in its API or config schema. Bifrost handles health monitoring internally via response status codes and circuit breaker logic. The `health_check` section in provider-pools.yaml is a control-plane-only concept that does not map to any Bifrost config field. | health.go (only /health endpoint for whole server), providers.go (no health_check field in any struct) | CORRECTED: health_check is a control-plane policy concept, not a Bifrost config field |
| 7 | generate-config.sh:9 | "A Bifrost config.json fragment" — output is JSON | Format is correct (JSON). However, the script produces `"providers": [...]` (an **array**) but Bifrost expects `"providers": {...}` (an **object** keyed by provider name). | providers.go:107 (map return), generate-config.sh:89 (array output) | CORRECTED: output format must be object, not array |
| 8 | generate-config.sh:75 | "PSEUDOCONFIG: Bifrost config.json fragment" | Same as #7 — the fragment is structurally wrong (array vs object). Also, the script output contains JSON comments (`//`) which are not valid JSON. | config.go (standard JSON parsing), generate-config.sh:74-83 | CORRECTED: invalid JSON (comments), wrong providers structure (array vs object) |
| 9 | generate-config.sh:78 | "exact JSON schema must be verified against Bifrost's actual config.json format" | The warning is warranted. The schema is available at `bifrost/transports/config.schema.json`. Key corrections needed: providers is object not array, `base_provider` -> `base_provider_type`, no per-key `base_url`. | config.schema.json, provider.go, account.go | CONFIRMED WARNING IS NEEDED |
| 10 | scripts/sync/README.md:25 | "PSEUDOCONFIG for Bifrost providers (fill in API keys manually)" | Correct that API keys need manual fill. However, the generated format (array) is wrong. Provider entries should be object properties keyed by provider name. | providers.go:107 (map-based storage) | CORRECTED: format is object not array |
| 11 | plan-sync.md:43 | "Verify field names against actual Bifrost schema" | All field names in config.example.json are correct for the `Key` and `NetworkConfig` structs. The main issue is structural (providers object vs array) and the `custom_provider_config` field name (`base_provider_type` not `base_provider`). | core/schemas/account.go, core/schemas/provider.go | CONFIRMED: field names correct, structure wrong |
| 12 | channels.example.md:147 | "Verify endpoint path and field names" for `POST /api/channel/` | **CONFIRMED** — New-API registers `channelRoute.POST("/", controller.AddChannel)` under the `/api/channel` group. The endpoint path `POST /api/channel/` is correct. Field names (`name`, `type`, `base_url`) need verification against the Channel model but the path is accurate. | new-api/router/api-router.go:217 | CONFIRMED: endpoint path is correct |
| 13 | instances.example.md:35 | "Verify field names against your ClewdR version" for `clewdr.toml` fields | **CONFIRMED** — ClewdR config uses TOML format. The admin API routes are: `GET /api/cookies`, `POST /api/cookie`, `PUT /api/cookie`, `DELETE /api/cookie`, `GET /api/config`, `POST /api/config`, `GET /api/version`. Field names in TOML (`ip`, `port`, `password`, `admin_password`) match the config struct. | clewdr/src/router.rs:96-118, clewdr/src/config/clewdr_config.rs | CONFIRMED CORRECT |
| 14 | config-sync.md:171 | "Always check the PSEUDOCONFIG label" — meta guidance | This guidance remains correct and important. The resolution of items #4, #5, #6, #7, #8 above demonstrates that several assumptions were wrong. | This audit | CONFIRMED: guidance is warranted |

---

### ClewdR-in-Bifrost Registration (PSEUDOCONFIG Resolution)

**Question:** Can ClewdR instances be registered as custom providers in Bifrost?

**Answer: YES** — using the `CustomProviderConfig` mechanism with these constraints:

1. **Each ClewdR instance MUST be a separate custom provider.** Custom provider names cannot reuse standard provider names (providers.go:200-203). Use names like `clewdr-1`, `clewdr-2`, `clewdr-3`.

2. **Per-key `base_url` is NOT supported.** The `base_url` field is on `NetworkConfig` (per-provider level, not per-key). This means you cannot put multiple ClewdR instances under a single provider entry. Each instance needs its own provider.

3. **Correct JSON payload to register a ClewdR instance via `POST /api/providers`:**

```json
{
  "provider": "clewdr-1",
  "keys": [
    {
      "id": "clewdr-1-key",
      "name": "ClewdR Instance 1",
      "value": "the-clewdr-password",
      "models": ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
      "weight": 1.0,
      "enabled": true
    }
  ],
  "network_config": {
    "base_url": "http://clewdr-1:8484",
    "default_request_timeout_in_seconds": 120,
    "max_retries": 0,
    "stream_idle_timeout_in_seconds": 120
  },
  "custom_provider_config": {
    "is_key_less": false,
    "base_provider_type": "openai",
    "allowed_requests": {
      "chat_completion": true,
      "chat_completion_stream": true
    }
  }
}
```

4. **The config.example.json providers section format is correct** (object keyed by provider name). However, ClewdR entries should NOT be under the `"openai"` key — they should be separate top-level entries like `"clewdr-1"`, `"clewdr-2"`.

5. **Routing rules can distribute traffic across ClewdR instances** using weighted targets:

```json
{
  "name": "Route lab models to ClewdR pool",
  "cel_expression": "model.startsWith('lab-')",
  "targets": [
    { "provider": "clewdr-1", "weight": 0.34 },
    { "provider": "clewdr-2", "weight": 0.33 },
    { "provider": "clewdr-3", "weight": 0.33 }
  ],
  "fallbacks": ["anthropic"],
  "scope": "global",
  "priority": 10
}
```

6. **The `PUT /api/providers/{provider}` endpoint supports full config replacement** (providers.go:317-319), which the sync engine can use for updating ClewdR provider configs without container restart.

---

### Bifrost API Summary Table

| # | Method | Path | Handler File | Handler Function |
|---|--------|------|-------------|-----------------|
| 1 | GET | /health | health.go | getHealth |
| 2 | GET | /api/providers | providers.go | listProviders |
| 3 | GET | /api/providers/{provider} | providers.go | getProvider |
| 4 | POST | /api/providers | providers.go | addProvider |
| 5 | PUT | /api/providers/{provider} | providers.go | updateProvider |
| 6 | DELETE | /api/providers/{provider} | providers.go | deleteProvider |
| 7 | GET | /api/keys | providers.go | listKeys |
| 8 | GET | /api/models | providers.go | listModels |
| 9 | GET | /api/models/parameters | providers.go | getModelParameters |
| 10 | GET | /api/models/base | providers.go | listBaseModels |
| 11 | GET | /api/config | config.go | getConfig |
| 12 | PUT | /api/config | config.go | updateConfig |
| 13 | GET | /api/version | config.go | getVersion |
| 14 | GET | /api/proxy-config | config.go | getProxyConfig |
| 15 | PUT | /api/proxy-config | config.go | updateProxyConfig |
| 16 | POST | /api/pricing/force-sync | config.go | forceSyncPricing |
| 17 | GET | /api/governance/virtual-keys | governance.go | getVirtualKeys |
| 18 | POST | /api/governance/virtual-keys | governance.go | createVirtualKey |
| 19 | GET | /api/governance/virtual-keys/{vk_id} | governance.go | getVirtualKey |
| 20 | PUT | /api/governance/virtual-keys/{vk_id} | governance.go | updateVirtualKey |
| 21 | DELETE | /api/governance/virtual-keys/{vk_id} | governance.go | deleteVirtualKey |
| 22 | GET | /api/governance/teams | governance.go | getTeams |
| 23 | POST | /api/governance/teams | governance.go | createTeam |
| 24 | GET | /api/governance/teams/{team_id} | governance.go | getTeam |
| 25 | PUT | /api/governance/teams/{team_id} | governance.go | updateTeam |
| 26 | DELETE | /api/governance/teams/{team_id} | governance.go | deleteTeam |
| 27 | GET | /api/governance/customers | governance.go | getCustomers |
| 28 | POST | /api/governance/customers | governance.go | createCustomer |
| 29 | GET | /api/governance/customers/{customer_id} | governance.go | getCustomer |
| 30 | PUT | /api/governance/customers/{customer_id} | governance.go | updateCustomer |
| 31 | DELETE | /api/governance/customers/{customer_id} | governance.go | deleteCustomer |
| 32 | GET | /api/governance/budgets | governance.go | getBudgets |
| 33 | GET | /api/governance/rate-limits | governance.go | getRateLimits |
| 34 | GET | /api/governance/routing-rules | governance.go | getRoutingRules |
| 35 | POST | /api/governance/routing-rules | governance.go | createRoutingRule |
| 36 | GET | /api/governance/routing-rules/{rule_id} | governance.go | getRoutingRule |
| 37 | PUT | /api/governance/routing-rules/{rule_id} | governance.go | updateRoutingRule |
| 38 | DELETE | /api/governance/routing-rules/{rule_id} | governance.go | deleteRoutingRule |
| 39 | GET | /api/governance/model-configs | governance.go | getModelConfigs |
| 40 | POST | /api/governance/model-configs | governance.go | createModelConfig |
| 41 | GET | /api/governance/model-configs/{mc_id} | governance.go | getModelConfig |
| 42 | PUT | /api/governance/model-configs/{mc_id} | governance.go | updateModelConfig |
| 43 | DELETE | /api/governance/model-configs/{mc_id} | governance.go | deleteModelConfig |
| 44 | GET | /api/governance/providers | governance.go | getProviderGovernance |
| 45 | PUT | /api/governance/providers/{provider_name} | governance.go | updateProviderGovernance |
| 46 | DELETE | /api/governance/providers/{provider_name} | governance.go | deleteProviderGovernance |
