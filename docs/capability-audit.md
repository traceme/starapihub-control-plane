# Capability Audit

This document is the authoritative reference for the actual API shapes, structs, and semantics of each upstream system. It is produced by reading source code directly and supersedes any PSEUDOCONFIG assumptions in other control-plane docs.

_Generated: 2026-03-21_
_Source: Upstream source code reading (new-api, clewdr, bifrost at HEAD)_

---

## New-API Admin API

### Authentication Levels

New-API uses four middleware tiers defined in `new-api/middleware/auth.go`. Each calls `authHelper(c, minRole)` which validates either a session cookie or an `Authorization` header (access token).

| Middleware | Min Role | Description |
|---|---|---|
| `middleware.UserAuth()` | `RoleCommonUser` | Any authenticated user (session or access token) |
| `middleware.AdminAuth()` | `RoleAdminUser` | Admin-level user required |
| `middleware.RootAuth()` | `RoleRootUser` | Root (superadmin) required |
| `middleware.TryUserAuth()` | _(none)_ | Best-effort: sets `id` in context if session exists, never blocks |
| `middleware.TokenAuthReadOnly()` | _(token)_ | Validates API token key exists (Bearer `sk-...`); does NOT check token status, expiry, or quota; still checks user ban status |

All admin endpoints also require the `New-Api-User` header matching the authenticated user ID.

---

### Channel Management

All channel endpoints are under `/api/channel/` and protected by **AdminAuth**, except where noted.

#### Channel Model Struct

Extracted from `new-api/model/channel.go` -- this is the critical struct for Phase 2 type definitions:

```go
// From new-api/model/channel.go
type Channel struct {
    Id                 int     `json:"id"`
    Type               int     `json:"type" gorm:"default:0"`
    Key                string  `json:"key" gorm:"not null"`
    OpenAIOrganization *string `json:"openai_organization"`
    TestModel          *string `json:"test_model"`
    Status             int     `json:"status" gorm:"default:1"`
    Name               string  `json:"name" gorm:"index"`
    Weight             *uint   `json:"weight" gorm:"default:0"`
    CreatedTime        int64   `json:"created_time" gorm:"bigint"`
    TestTime           int64   `json:"test_time" gorm:"bigint"`
    ResponseTime       int     `json:"response_time"`
    BaseURL            *string `json:"base_url" gorm:"column:base_url;default:''"`
    Other              string  `json:"other"`
    Balance            float64 `json:"balance"`
    BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
    Models             string  `json:"models"`
    Group              string  `json:"group" gorm:"type:varchar(64);default:'default'"`
    UsedQuota          int64   `json:"used_quota" gorm:"bigint;default:0"`
    ModelMapping       *string `json:"model_mapping" gorm:"type:text"`
    StatusCodeMapping  *string `json:"status_code_mapping" gorm:"type:varchar(1024);default:''"`
    Priority           *int64  `json:"priority" gorm:"bigint;default:0"`
    AutoBan            *int    `json:"auto_ban" gorm:"default:1"`
    OtherInfo          string  `json:"other_info"`
    Tag                *string `json:"tag" gorm:"index"`
    Setting            *string `json:"setting" gorm:"type:text"`
    ParamOverride      *string `json:"param_override" gorm:"type:text"`
    HeaderOverride     *string `json:"header_override" gorm:"type:text"`
    Remark             *string `json:"remark" gorm:"type:varchar(255)"`
    ChannelInfo        ChannelInfo `json:"channel_info" gorm:"type:json"`
    OtherSettings      string  `json:"settings" gorm:"column:settings"`
}

type ChannelInfo struct {
    IsMultiKey             bool                  `json:"is_multi_key"`
    MultiKeySize           int                   `json:"multi_key_size"`
    MultiKeyStatusList     map[int]int           `json:"multi_key_status_list"`
    MultiKeyDisabledReason map[int]string        `json:"multi_key_disabled_reason,omitempty"`
    MultiKeyDisabledTime   map[int]int64         `json:"multi_key_disabled_time,omitempty"`
    MultiKeyPollingIndex   int                   `json:"multi_key_polling_index"`
    MultiKeyMode           constant.MultiKeyMode `json:"multi_key_mode"`
}
```

**Key fields for sync:** `id`, `name`, `type`, `key`, `base_url`, `models`, `status`, `priority`, `weight`, `group`, `tag`, `model_mapping`, `setting`, `param_override`, `header_override`, `channel_info`.

---

#### GET /api/channel/

- **Handler:** `controller.GetAllChannels` (channel.go:71)
- **Auth:** AdminAuth
- **Query params:** `p` (page), `page_size`, `id_sort` (bool), `tag_mode` (bool), `status` (enabled/disabled/1/0), `type` (int)
- **Response:**
  ```json
  {
    "success": true, "message": "",
    "data": {
      "items": ["<Channel objects, key field omitted>"],
      "total": 100,
      "page": 1,
      "page_size": 20,
      "type_counts": {"1": 5, "3": 10}
    }
  }
  ```
- **Notes:** The `key` field is explicitly omitted from list results via `.Omit("key")`. ChannelInfo multi-key details are cleared for security.

#### GET /api/channel/search

- **Handler:** `controller.SearchChannels` (channel.go:248)
- **Auth:** AdminAuth
- **Query params:** `keyword`, `group`, `model`, `status`, `id_sort`, `tag_mode`, `type`, `p`, `page_size`
- **Response:** Same shape as GET `/api/channel/` -- `{"success": true, "data": {"items": [...], "total": N, "type_counts": {...}}}`
- **Notes:** Client-side pagination over full search results.

#### GET /api/channel/:id

- **Handler:** `controller.GetChannel` (channel.go:361)
- **Auth:** AdminAuth
- **Response:**
  ```json
  {"success": true, "message": "", "data": "<Channel object>"}
  ```
- **Notes:** Returns single channel. ChannelInfo multi-key details are cleared. Key field is NOT included (use the separate key endpoint).

#### POST /api/channel/

- **Handler:** `controller.AddChannel` (channel.go:566)
- **Auth:** AdminAuth
- **Request:**
  ```go
  // From controller/channel.go
  type AddChannelRequest struct {
      Mode                      string                `json:"mode"`          // "single", "batch", "multi_to_single"
      MultiKeyMode              constant.MultiKeyMode `json:"multi_key_mode"`
      BatchAddSetKeyPrefix2Name bool                  `json:"batch_add_set_key_prefix_2_name"`
      Channel                   *model.Channel        `json:"channel"`
  }
  ```
- **Response:** `{"success": true, "message": ""}`
- **Notes:** Three modes: `single` (one channel), `batch` (one channel per key, split by newline), `multi_to_single` (one channel with multi-key support). Vertex AI keys can be JSON arrays.

#### PUT /api/channel/

- **Handler:** `controller.UpdateChannel` (channel.go:842)
- **Auth:** AdminAuth
- **Request:**
  ```go
  // From controller/channel.go
  type PatchChannel struct {
      model.Channel                          // embedded Channel struct
      MultiKeyMode *string `json:"multi_key_mode"`
      KeyMode      *string `json:"key_mode"` // "append" or "replace" for multi-key channels
  }
  ```
- **Response:** `{"success": true, "message": ""}`
- **Notes:** Preserves existing ChannelInfo. Multi-key channels support `key_mode: "append"` to add keys without overwriting, or `"replace"` to overwrite. Deduplicates keys on append.

#### DELETE /api/channel/:id

- **Handler:** `controller.DeleteChannel` (channel.go:666)
- **Auth:** AdminAuth
- **Response:** `{"success": true, "message": ""}`

#### POST /api/channel/:id/key

- **Handler:** `controller.GetChannelKey` (channel.go:385)
- **Auth:** **RootAuth** + CriticalRateLimit + DisableCache + SecureVerificationRequired
- **Response:** `{"success": true, "message": "", "data": {"key": "..."}}`
- **Notes:** Requires secure verification (2FA/passkey). This is the ONLY way to retrieve the actual channel key. Audit-logged with user ID.

#### GET /api/channel/test

- **Handler:** `controller.TestAllChannels` (channel.go)
- **Auth:** AdminAuth
- **Response:** `{"success": true, "message": ""}`
- **Notes:** Triggers async test of all channels.

#### GET /api/channel/test/:id

- **Handler:** `controller.TestChannel` (channel.go)
- **Auth:** AdminAuth
- **Response:** Test result with response time, model tested, error if any.

#### GET /api/channel/fetch_models/:id

- **Handler:** `controller.FetchUpstreamModels` (channel.go:203)
- **Auth:** AdminAuth
- **Response:** `{"success": true, "message": "", "data": ["model-1", "model-2"]}`
- **Notes:** Fetches model list from the upstream provider for the given channel.

#### POST /api/channel/upstream_updates/detect_all

- **Handler:** `controller.DetectAllChannelUpstreamModelUpdates` (channel.go)
- **Auth:** AdminAuth
- **Response:** Detection results for model changes across all channels.

#### POST /api/channel/upstream_updates/apply_all

- **Handler:** `controller.ApplyAllChannelUpstreamModelUpdates` (channel.go)
- **Auth:** AdminAuth
- **Response:** Application results.

#### Additional Channel Endpoints

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/channel/models` | `controller.ChannelListModels` | AdminAuth | List all models across channels |
| GET | `/api/channel/models_enabled` | `controller.EnabledListModels` | AdminAuth | List models from enabled channels only |
| GET | `/api/channel/update_balance` | `controller.UpdateAllChannelsBalance` | AdminAuth | Refresh balance for all channels |
| GET | `/api/channel/update_balance/:id` | `controller.UpdateChannelBalance` | AdminAuth | Refresh balance for single channel |
| DELETE | `/api/channel/disabled` | `controller.DeleteDisabledChannel` | AdminAuth | Delete all disabled channels |
| POST | `/api/channel/tag/disabled` | `controller.DisableTagChannels` | AdminAuth | Disable all channels with a given tag |
| POST | `/api/channel/tag/enabled` | `controller.EnableTagChannels` | AdminAuth | Enable all channels with a given tag |
| PUT | `/api/channel/tag` | `controller.EditTagChannels` | AdminAuth | Edit properties of all channels with a tag |
| POST | `/api/channel/batch` | `controller.DeleteChannelBatch` | AdminAuth | Batch delete channels by ID list |
| POST | `/api/channel/fix` | `controller.FixChannelsAbilities` | AdminAuth | Rebuild channel-model ability table |
| POST | `/api/channel/fetch_models` | `controller.FetchModels` | AdminAuth | Fetch models (batch) |
| POST | `/api/channel/batch/tag` | `controller.BatchSetChannelTag` | AdminAuth | Set tag for multiple channels |
| GET | `/api/channel/tag/models` | `controller.GetTagModels` | AdminAuth | Get models grouped by tag |
| POST | `/api/channel/copy/:id` | `controller.CopyChannel` | AdminAuth | Duplicate a channel |
| POST | `/api/channel/multi_key/manage` | `controller.ManageMultiKeys` | AdminAuth | Manage multi-key channel keys |
| POST | `/api/channel/upstream_updates/detect` | `controller.DetectChannelUpstreamModelUpdates` | AdminAuth | Detect upstream updates for one channel |
| POST | `/api/channel/upstream_updates/apply` | `controller.ApplyChannelUpstreamModelUpdates` | AdminAuth | Apply upstream updates for one channel |
| POST | `/api/channel/codex/oauth/start` | `controller.StartCodexOAuth` | AdminAuth | Start Codex OAuth flow |
| POST | `/api/channel/codex/oauth/complete` | `controller.CompleteCodexOAuth` | AdminAuth | Complete Codex OAuth flow |
| POST | `/api/channel/:id/codex/oauth/start` | `controller.StartCodexOAuthForChannel` | AdminAuth | Start Codex OAuth for specific channel |
| POST | `/api/channel/:id/codex/oauth/complete` | `controller.CompleteCodexOAuthForChannel` | AdminAuth | Complete Codex OAuth for specific channel |
| POST | `/api/channel/:id/codex/refresh` | `controller.RefreshCodexChannelCredential` | AdminAuth | Refresh Codex credentials |
| GET | `/api/channel/:id/codex/usage` | `controller.GetCodexChannelUsage` | AdminAuth | Get Codex usage stats |
| POST | `/api/channel/ollama/pull` | `controller.OllamaPullModel` | AdminAuth | Pull Ollama model |
| POST | `/api/channel/ollama/pull/stream` | `controller.OllamaPullModelStream` | AdminAuth | Pull Ollama model (streaming) |
| DELETE | `/api/channel/ollama/delete` | `controller.OllamaDeleteModel` | AdminAuth | Delete Ollama model |
| GET | `/api/channel/ollama/version/:id` | `controller.OllamaVersion` | AdminAuth | Get Ollama version for channel |

---

### System Options (Pricing)

Options are under `/api/option/` and protected by **RootAuth**.

#### GET /api/option/

- **Handler:** `controller.GetOptions` (option.go:63)
- **Auth:** RootAuth
- **Response:**
  ```json
  {
    "success": true, "message": "",
    "data": [
      {"key": "ModelRatio", "value": "{\"gpt-4\": 15, \"gpt-3.5-turbo\": 0.75}"},
      {"key": "ModelPrice", "value": "{\"gpt-4\": 0.03}"},
      {"key": "CompletionRatio", "value": "{\"gpt-4\": 2}"}
    ]
  }
  ```
- **Notes:** Returns ALL options as `{key, value}` pairs. Sensitive keys (ending in `Token`, `Secret`, `Key`, `secret`, `api_key`) are filtered out. A synthetic `CompletionRatioMeta` key is appended with per-model ratio metadata.

#### PUT /api/option/

- **Handler:** `controller.UpdateOption` (option.go:105)
- **Auth:** RootAuth
- **Request:**
  ```go
  // From controller/option.go
  type OptionUpdateRequest struct {
      Key   string `json:"key"`
      Value any    `json:"value"`
  }
  ```
  Example:
  ```json
  {"key": "ModelRatio", "value": "{\"gpt-4\": 15, \"gpt-3.5-turbo\": 0.75}"}
  ```
- **Response:** `{"success": true, "message": ""}`
- **Notes:** The `value` field accepts bool, float64, int, or string -- all are coerced to string for storage. Specific keys have validation logic (e.g., `GroupRatio` is validated, OAuth keys check prerequisites).

**Pricing-relevant option keys** (from `controller/option.go`):

| Key | Format | Description |
|---|---|---|
| `ModelRatio` | JSON string: `{"model-name": float, ...}` | Token cost multiplier per model (relative to GPT-3.5) |
| `ModelPrice` | JSON string: `{"model-name": float, ...}` | Absolute price per 1K tokens (USD) |
| `CompletionRatio` | JSON string: `{"model-name": float, ...}` | Output/input token cost ratio |
| `CacheRatio` | JSON string: `{"model-name": float, ...}` | Cache hit discount ratio |
| `CreateCacheRatio` | JSON string: `{"model-name": float, ...}` | Cache creation cost ratio |
| `ImageRatio` | JSON string: `{"model-name": float, ...}` | Image generation cost ratio |
| `AudioRatio` | JSON string: `{"model-name": float, ...}` | Audio input cost ratio |
| `AudioCompletionRatio` | JSON string: `{"model-name": float, ...}` | Audio output cost ratio |
| `GroupRatio` | JSON string: `{"group-name": float, ...}` | Per-group cost multiplier |

All ratio values are stored as JSON-encoded strings in the `options` table. The `value` in PUT request is the JSON string itself (not double-encoded).

#### Additional Option Endpoints

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/option/channel_affinity_cache` | `controller.GetChannelAffinityCacheStats` | RootAuth | Cache statistics |
| DELETE | `/api/option/channel_affinity_cache` | `controller.ClearChannelAffinityCache` | RootAuth | Clear affinity cache |
| POST | `/api/option/rest_model_ratio` | `controller.ResetModelRatio` | RootAuth | Reset model ratios to defaults |
| POST | `/api/option/migrate_console_setting` | `controller.MigrateConsoleSetting` | RootAuth | Legacy migration (temporary) |

---

### User Management

All admin user endpoints are under `/api/user/` with **AdminAuth**.

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/user/` | `controller.GetAllUsers` | AdminAuth | List all users (paginated) |
| GET | `/api/user/search` | `controller.SearchUsers` | AdminAuth | Search users |
| GET | `/api/user/:id` | `controller.GetUser` | AdminAuth | Get single user |
| POST | `/api/user/` | `controller.CreateUser` | AdminAuth | Create user |
| POST | `/api/user/manage` | `controller.ManageUser` | AdminAuth | Manage user (enable/disable/set role) |
| PUT | `/api/user/` | `controller.UpdateUser` | AdminAuth | Update user |
| DELETE | `/api/user/:id` | `controller.DeleteUser` | AdminAuth | Delete user |
| DELETE | `/api/user/:id/reset_passkey` | `controller.AdminResetPasskey` | AdminAuth | Reset user passkey |
| GET | `/api/user/topup` | `controller.GetAllTopUps` | AdminAuth | List all top-ups |
| POST | `/api/user/topup/complete` | `controller.AdminCompleteTopUp` | AdminAuth | Complete a top-up |
| GET | `/api/user/:id/oauth/bindings` | `controller.GetUserOAuthBindingsByAdmin` | AdminAuth | View OAuth bindings |
| DELETE | `/api/user/:id/oauth/bindings/:provider_id` | `controller.UnbindCustomOAuthByAdmin` | AdminAuth | Unbind OAuth |
| DELETE | `/api/user/:id/bindings/:binding_type` | `controller.AdminClearUserBinding` | AdminAuth | Clear binding |
| GET | `/api/user/2fa/stats` | `controller.Admin2FAStats` | AdminAuth | 2FA usage statistics |
| DELETE | `/api/user/:id/2fa` | `controller.AdminDisable2FA` | AdminAuth | Disable 2FA for user |

---

### Token Management

Token endpoints are under `/api/token/` with **UserAuth** (each user manages their own tokens).

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/token/` | `controller.GetAllTokens` | UserAuth | List user's tokens |
| GET | `/api/token/search` | `controller.SearchTokens` | UserAuth + SearchRateLimit | Search tokens |
| GET | `/api/token/:id` | `controller.GetToken` | UserAuth | Get single token |
| POST | `/api/token/:id/key` | `controller.GetTokenKey` | UserAuth + CriticalRateLimit + DisableCache | Retrieve token key |
| POST | `/api/token/` | `controller.AddToken` | UserAuth | Create token |
| PUT | `/api/token/` | `controller.UpdateToken` | UserAuth | Update token |
| DELETE | `/api/token/:id` | `controller.DeleteToken` | UserAuth | Delete token |
| POST | `/api/token/batch` | `controller.DeleteTokenBatch` | UserAuth | Batch delete tokens |

---

### Other Admin Endpoints

#### Logs

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/log/` | `controller.GetAllLogs` | AdminAuth | List all logs |
| DELETE | `/api/log/` | `controller.DeleteHistoryLogs` | AdminAuth | Delete old logs |
| GET | `/api/log/stat` | `controller.GetLogsStat` | AdminAuth | Log statistics |
| GET | `/api/log/search` | `controller.SearchAllLogs` | AdminAuth | Search logs |
| GET | `/api/log/channel_affinity_usage_cache` | `controller.GetChannelAffinityUsageCacheStats` | AdminAuth | Affinity usage cache stats |

#### Redemption Codes

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/redemption/` | `controller.GetAllRedemptions` | AdminAuth | List codes |
| GET | `/api/redemption/search` | `controller.SearchRedemptions` | AdminAuth | Search codes |
| GET | `/api/redemption/:id` | `controller.GetRedemption` | AdminAuth | Get code |
| POST | `/api/redemption/` | `controller.AddRedemption` | AdminAuth | Create code |
| PUT | `/api/redemption/` | `controller.UpdateRedemption` | AdminAuth | Update code |
| DELETE | `/api/redemption/invalid` | `controller.DeleteInvalidRedemption` | AdminAuth | Delete invalid codes |
| DELETE | `/api/redemption/:id` | `controller.DeleteRedemption` | AdminAuth | Delete code |

#### Subscription Plans

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/subscription/admin/plans` | `controller.AdminListSubscriptionPlans` | AdminAuth | List plans |
| POST | `/api/subscription/admin/plans` | `controller.AdminCreateSubscriptionPlan` | AdminAuth | Create plan |
| PUT | `/api/subscription/admin/plans/:id` | `controller.AdminUpdateSubscriptionPlan` | AdminAuth | Update plan |
| PATCH | `/api/subscription/admin/plans/:id` | `controller.AdminUpdateSubscriptionPlanStatus` | AdminAuth | Toggle plan status |
| POST | `/api/subscription/admin/bind` | `controller.AdminBindSubscription` | AdminAuth | Bind subscription to user |
| GET | `/api/subscription/admin/users/:id/subscriptions` | `controller.AdminListUserSubscriptions` | AdminAuth | List user subscriptions |
| POST | `/api/subscription/admin/users/:id/subscriptions` | `controller.AdminCreateUserSubscription` | AdminAuth | Create user subscription |
| POST | `/api/subscription/admin/user_subscriptions/:id/invalidate` | `controller.AdminInvalidateUserSubscription` | AdminAuth | Invalidate subscription |
| DELETE | `/api/subscription/admin/user_subscriptions/:id` | `controller.AdminDeleteUserSubscription` | AdminAuth | Delete subscription |

#### Performance (Root Only)

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/performance/stats` | `controller.GetPerformanceStats` | RootAuth | System performance stats |
| DELETE | `/api/performance/disk_cache` | `controller.ClearDiskCache` | RootAuth | Clear disk cache |
| POST | `/api/performance/reset_stats` | `controller.ResetPerformanceStats` | RootAuth | Reset stats counters |
| POST | `/api/performance/gc` | `controller.ForceGC` | RootAuth | Force garbage collection |

#### Ratio Sync (Root Only)

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/ratio_sync/channels` | `controller.GetSyncableChannels` | RootAuth | List channels eligible for ratio sync |
| POST | `/api/ratio_sync/fetch` | `controller.FetchUpstreamRatios` | RootAuth | Fetch ratios from upstream providers |

#### Groups

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/group/` | `controller.GetGroups` | AdminAuth | List groups |

#### Prefill Groups

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/prefill_group/` | `controller.GetPrefillGroups` | AdminAuth | List prefill groups |
| POST | `/api/prefill_group/` | `controller.CreatePrefillGroup` | AdminAuth | Create prefill group |
| PUT | `/api/prefill_group/` | `controller.UpdatePrefillGroup` | AdminAuth | Update prefill group |
| DELETE | `/api/prefill_group/:id` | `controller.DeletePrefillGroup` | AdminAuth | Delete prefill group |

#### Vendors

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/vendors/` | `controller.GetAllVendors` | AdminAuth | List vendors |
| GET | `/api/vendors/search` | `controller.SearchVendors` | AdminAuth | Search vendors |
| GET | `/api/vendors/:id` | `controller.GetVendorMeta` | AdminAuth | Get vendor |
| POST | `/api/vendors/` | `controller.CreateVendorMeta` | AdminAuth | Create vendor |
| PUT | `/api/vendors/` | `controller.UpdateVendorMeta` | AdminAuth | Update vendor |
| DELETE | `/api/vendors/:id` | `controller.DeleteVendorMeta` | AdminAuth | Delete vendor |

#### Models Meta

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/models/` | `controller.GetAllModelsMeta` | AdminAuth | List model metadata |
| GET | `/api/models/search` | `controller.SearchModelsMeta` | AdminAuth | Search models |
| GET | `/api/models/:id` | `controller.GetModelMeta` | AdminAuth | Get model |
| POST | `/api/models/` | `controller.CreateModelMeta` | AdminAuth | Create model |
| PUT | `/api/models/` | `controller.UpdateModelMeta` | AdminAuth | Update model |
| DELETE | `/api/models/:id` | `controller.DeleteModelMeta` | AdminAuth | Delete model |
| GET | `/api/models/sync_upstream/preview` | `controller.SyncUpstreamPreview` | AdminAuth | Preview upstream sync |
| POST | `/api/models/sync_upstream` | `controller.SyncUpstreamModels` | AdminAuth | Sync models from upstream |
| GET | `/api/models/missing` | `controller.GetMissingModels` | AdminAuth | Find models without metadata |

#### Custom OAuth Providers (Root Only)

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| POST | `/api/custom-oauth-provider/discovery` | `controller.FetchCustomOAuthDiscovery` | RootAuth | Fetch OIDC discovery |
| GET | `/api/custom-oauth-provider/` | `controller.GetCustomOAuthProviders` | RootAuth | List providers |
| GET | `/api/custom-oauth-provider/:id` | `controller.GetCustomOAuthProvider` | RootAuth | Get provider |
| POST | `/api/custom-oauth-provider/` | `controller.CreateCustomOAuthProvider` | RootAuth | Create provider |
| PUT | `/api/custom-oauth-provider/:id` | `controller.UpdateCustomOAuthProvider` | RootAuth | Update provider |
| DELETE | `/api/custom-oauth-provider/:id` | `controller.DeleteCustomOAuthProvider` | RootAuth | Delete provider |

#### Deployments (AdminAuth)

| Method | Path | Handler | Auth | Description |
|---|---|---|---|---|
| GET | `/api/deployments/settings` | `controller.GetModelDeploymentSettings` | AdminAuth | Deployment settings |
| POST | `/api/deployments/settings/test-connection` | `controller.TestIoNetConnection` | AdminAuth | Test io.net connection |
| GET | `/api/deployments/` | `controller.GetAllDeployments` | AdminAuth | List deployments |
| GET | `/api/deployments/search` | `controller.SearchDeployments` | AdminAuth | Search deployments |
| POST | `/api/deployments/` | `controller.CreateDeployment` | AdminAuth | Create deployment |
| GET | `/api/deployments/:id` | `controller.GetDeployment` | AdminAuth | Get deployment |
| PUT | `/api/deployments/:id` | `controller.UpdateDeployment` | AdminAuth | Update deployment |
| DELETE | `/api/deployments/:id` | `controller.DeleteDeployment` | AdminAuth | Delete deployment |

---

## ClewdR Automation Surfaces

### Authentication

ClewdR uses a Bearer token system for admin endpoints. All `/api/*` endpoints (except `/api/version`) require the `Authorization: Bearer <token>` header. The token is validated against `admin_password` in `clewdr.toml` via `CLEWDR_CONFIG.load().admin_auth(&token)`.

Three auth extractors are used in the router (`clewdr/src/router.rs`):

| Extractor | Description |
|---|---|
| `RequireAdminAuth` | Admin token required -- used for `/api/*` admin endpoints |
| `RequireBearerAuth` | Bearer token required -- used for OpenAI-compatible inference endpoints |
| `RequireFlexibleAuth` | Flexible auth -- used for Anthropic-native inference endpoints |

---

### Route Inventory

Extracted from `clewdr/src/router.rs`:

| Method | Path | Handler | Auth | Category |
|---|---|---|---|---|
| GET | `/api/cookies` | `api_get_cookies` | RequireAdminAuth | Admin - Cookie mgmt |
| POST | `/api/cookie` | `api_post_cookie` | RequireAdminAuth | Admin - Cookie mgmt |
| PUT | `/api/cookie` | `api_put_cookie` | RequireAdminAuth | Admin - Cookie mgmt |
| DELETE | `/api/cookie` | `api_delete_cookie` | RequireAdminAuth | Admin - Cookie mgmt |
| GET | `/api/auth` | `api_auth` | RequireAdminAuth | Admin - Auth verify |
| GET | `/api/config` | `api_get_config` | RequireAdminAuth | Admin - Config |
| POST | `/api/config` | `api_post_config` | RequireAdminAuth | Admin - Config |
| GET | `/api/version` | `api_version` | None | Public |
| POST | `/v1/messages` | `api_claude_web` | RequireFlexibleAuth | Inference - Claude.ai |
| POST | `/v1/chat/completions` | `api_claude_web` | RequireBearerAuth | Inference - Claude.ai OAI |
| GET | `/v1/models` | `api_get_models` | RequireBearerAuth | Inference - Model list |
| POST | `/code/v1/messages` | `api_claude_code` | RequireFlexibleAuth | Inference - Claude Code |
| POST | `/code/v1/messages/count_tokens` | `api_claude_code_count_tokens` | RequireFlexibleAuth | Inference - Token counting |
| POST | `/code/v1/chat/completions` | `api_claude_code` | RequireBearerAuth | Inference - Claude Code OAI |
| GET | `/code/v1/models` | `api_get_models` | RequireBearerAuth | Inference - Model list |

---

### Cookie Management Endpoints

#### CookieStatus Struct

Extracted from `clewdr/src/config/cookie.rs` -- the core data model for cookie management:

```rust
// From clewdr/src/config/cookie.rs
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct CookieStatus {
    pub cookie: ClewdrCookie,                    // The session cookie string
    pub token: Option<TokenInfo>,                // Associated token info
    pub reset_time: Option<i64>,                 // Epoch seconds
    pub supports_claude_1m_sonnet: Option<bool>, // 1M context for Sonnet
    pub supports_claude_1m_opus: Option<bool>,   // 1M context for Opus
    pub count_tokens_allowed: Option<bool>,      // Token counting support

    // Per-period usage breakdown
    pub session_usage: UsageBreakdown,
    pub weekly_usage: UsageBreakdown,
    pub weekly_sonnet_usage: UsageBreakdown,
    pub weekly_opus_usage: UsageBreakdown,
    pub lifetime_usage: UsageBreakdown,

    // Reset boundaries (epoch seconds, UTC)
    pub session_resets_at: Option<i64>,
    pub weekly_resets_at: Option<i64>,
    pub weekly_sonnet_resets_at: Option<i64>,
    pub weekly_opus_resets_at: Option<i64>,

    pub resets_last_checked_at: Option<i64>,
}

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct UsageBreakdown {
    pub total_input_tokens: u64,
    pub total_output_tokens: u64,
    pub sonnet_input_tokens: u64,
    pub sonnet_output_tokens: u64,
    pub opus_input_tokens: u64,
    pub opus_output_tokens: u64,
}
```

#### GET /api/cookies

- **Handler:** `api_get_cookies` (misc.rs:139)
- **Auth:** RequireAdminAuth (Bearer token)
- **Query params:** `refresh` (bool, default false) -- force cache bypass
- **Response:**
  ```json
  {
    "valid": [
      {
        "cookie": "...",
        "token": null,
        "reset_time": null,
        "supports_claude_1m_sonnet": true,
        "supports_claude_1m_opus": true,
        "session_utilization": 45,
        "session_resets_at": "2026-03-21T15:00:00Z",
        "seven_day_utilization": 12,
        "seven_day_resets_at": "2026-03-25T00:00:00Z",
        "seven_day_opus_utilization": 5,
        "seven_day_opus_resets_at": "...",
        "seven_day_sonnet_utilization": 8,
        "seven_day_sonnet_resets_at": "..."
      }
    ],
    "exhausted": ["...same shape as valid..."],
    "invalid": ["...CookieStatus objects without utilization enrichment..."]
  }
  ```
- **Headers:** `X-Cache-Status` (HIT/MISS), `X-Cache-Timestamp` (epoch seconds)
- **Notes:** Response is cached for 5 minutes. The `valid` and `exhausted` arrays are augmented with live utilization percentages fetched from Anthropic's console API (session_utilization, seven_day_utilization, seven_day_opus_utilization, seven_day_sonnet_utilization, plus their resets_at timestamps). The `invalid` array contains raw CookieStatus objects without utilization.

#### POST /api/cookie

- **Handler:** `api_post_cookie` (misc.rs:61)
- **Auth:** RequireAdminAuth (Bearer token)
- **Request:** `CookieStatus` JSON body. Minimal required:
  ```json
  {
    "cookie": "<claude-session-cookie-value>"
  }
  ```
  Optional fields: `supports_claude_1m_sonnet`, `supports_claude_1m_opus` (default `true` if omitted).
- **Response:** `200 OK` (no body) on success
- **Notes:** `reset_time` is always cleared on submit. Invalidates the cookie status cache.

#### PUT /api/cookie

- **Handler:** `api_put_cookie` (misc.rs:97)
- **Auth:** RequireAdminAuth (Bearer token)
- **Request:** `CookieStatus` JSON body with `cookie` field to identify which cookie, plus updated 1M support flags:
  ```json
  {
    "cookie": "<existing-cookie>",
    "supports_claude_1m_sonnet": false,
    "supports_claude_1m_opus": true
  }
  ```
- **Response:** `200 OK` (no body) on success
- **Notes:** Only updates `supports_claude_1m_sonnet` and `supports_claude_1m_opus` flags. Does not change other cookie properties.

#### DELETE /api/cookie

- **Handler:** `api_delete_cookie` (misc.rs:230)
- **Auth:** RequireAdminAuth (Bearer token)
- **Request:** `CookieStatus` JSON body with `cookie` field identifying the cookie to delete:
  ```json
  {"cookie": "<cookie-to-delete>"}
  ```
- **Response:** `204 No Content` on success
- **Notes:** Removes cookie from all collections (valid, exhausted, invalid). Invalidates cache.

---

### Config Endpoints

#### GET /api/config

- **Handler:** `api_get_config` (config.rs:17)
- **Auth:** RequireAdminAuth (Bearer token)
- **Response:** Full `ClewdrConfig` as JSON, with `cookie_array` and `wasted_cookie` fields removed.
  Key fields returned:
  ```json
  {
    "ip": "0.0.0.0",
    "port": 8484,
    "check_update": true,
    "auto_update": false,
    "no_fs": false,
    "log_to_file": false,
    "password": "...",
    "admin_password": "...",
    "proxy": null,
    "rproxy": null,
    "max_retries": 3,
    "preserve_chats": false,
    "web_search": false,
    "enable_web_count_tokens": false,
    "sanitize_messages": false,
    "skip_first_warning": false,
    "skip_second_warning": false,
    "skip_restricted": false,
    "skip_non_pro": false,
    "skip_rate_limit": true,
    "skip_normal_pro": false,
    "use_real_roles": true,
    "custom_h": null,
    "custom_a": null,
    "custom_prompt": "",
    "claude_code_client_id": null,
    "custom_system": null
  }
  ```
- **Notes:** Sensitive cookie data is stripped. Server restart required for `ip`/`port` changes.

#### POST /api/config

- **Handler:** `api_post_config` (config.rs:43)
- **Auth:** RequireAdminAuth (Bearer token)
- **Request:** Full `ClewdrConfig` JSON body. Cookie arrays are preserved from the existing config -- only non-cookie fields are updated.
- **Response:**
  ```json
  {
    "message": "Config updated successfully",
    "config": {}
  }
  ```
- **Notes:** The submitted config is validated via `.validate()`. Existing `cookie_array` and `wasted_cookie` are carried over from the running config. Config is persisted to `clewdr.toml`.

---

### Other Endpoints

#### GET /api/version

- **Handler:** `api_version` (misc.rs:261)
- **Auth:** None (public)
- **Response:** Plain text version string (e.g., `"clewdr v1.2.3"`)

#### GET /api/auth

- **Handler:** `api_auth` (misc.rs:273)
- **Auth:** RequireAdminAuth (Bearer token)
- **Response:** `200 OK` if token valid, `401 Unauthorized` if not
- **Notes:** Used for token verification -- a simple health check for auth.

#### GET /v1/models and GET /code/v1/models

- **Handler:** `api_get_models` (misc.rs:312)
- **Auth:** RequireBearerAuth
- **Response:**
  ```json
  {
    "object": "list",
    "data": [
      {"id": "claude-sonnet-4-6", "object": "model", "created": 0, "owned_by": "clewdr"},
      {"id": "claude-opus-4-6", "object": "model", "created": 0, "owned_by": "clewdr"}
    ]
  }
  ```
- **Notes:** Returns a hardcoded list of 26 Claude models. OpenAI-compatible format.

---

### Inference Endpoints (Bifrost Integration Reference)

| Method | Path | Format | Auth | Handler |
|---|---|---|---|---|
| POST | `/v1/messages` | Anthropic Messages API | RequireFlexibleAuth | `api_claude_web` |
| POST | `/v1/chat/completions` | OpenAI Chat Completions | RequireBearerAuth | `api_claude_web` (with `to_oai` response transform) |
| POST | `/code/v1/messages` | Anthropic Messages API | RequireFlexibleAuth | `api_claude_code` |
| POST | `/code/v1/messages/count_tokens` | Anthropic Token Count | RequireFlexibleAuth | `api_claude_code_count_tokens` |
| POST | `/code/v1/chat/completions` | OpenAI Chat Completions | RequireBearerAuth | `api_claude_code` (with `to_oai` response transform) |

**Middleware pipeline for Claude.ai endpoints:** `RequireFlexibleAuth` or `RequireBearerAuth` -> `CompressionLayer` -> `check_overloaded` -> `apply_stop_sequences` -> `add_usage_info` (web only)

**Middleware pipeline for Claude Code endpoints:** `RequireFlexibleAuth` or `RequireBearerAuth` -> `CompressionLayer` -> (OAI: `to_oai`)

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
- `CustomProviderKey`: Internal field (JSON:"-"), set by Bifrost -- not in request payloads.

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
- **Path param:** `provider` -- the `ModelProvider` string (e.g., "anthropic", "clewdr-1")
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
- **Semantics:** **FULL REPLACE** -- "This endpoint expects ALL fields to be provided in the request body, including both edited and non-edited fields. Partial updates are not supported." (providers.go:317-319)
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
  - `query` -- case-insensitive fuzzy filter on model name
  - `provider` -- filter by specific provider
  - `keys` -- comma-separated key IDs to filter by key access
  - `limit` -- max results (default: 5)
  - `unfiltered` -- if "true", returns all models including filtered ones
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
- **Query param:** `model` (required) -- model name
- **Response:** Raw JSON string from `GetModelParameters()` -- the model's parameter schema

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
- **Query param:** `from_db=true` -- if set, reads from database instead of in-memory store
- **Response:** Map with these top-level keys:
  ```json
  {
    "client_config": {},
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
    "proxy_config": {},
    "restart_required": { "required": false, "reason": "" }
  }
  ```

#### PUT /api/config -- Merge vs Replace Semantics

- **Handler:** `updateConfig` (config.go:200)
- **Auth:** Middleware chain

**CRITICAL FINDING -- FIELD-LEVEL MERGE, NOT FULL REPLACE:**

The `updateConfig` handler does NOT replace the entire config. It performs **field-by-field comparison and selective update**. Evidence from config.go:242-400:

1. Reads current in-memory `ClientConfig` (line 242: `currentConfig := h.store.ClientConfig`)
2. Creates `updatedConfig` as a copy of current (line 243: `updatedConfig := currentConfig`)
3. Compares each field individually:
   - `DropExcessRequests`: Updated only if different from current (line 247)
   - `InitialPoolSize`: Updated only if > 0 and different (line 304)
   - `EnableLogging`: Always applied (line 314)
   - `MCPAgentDepth`: Updated only if > 0 and different (line 263)
   - `PrometheusLabels`, `AllowedOrigins`, `AllowedHeaders`: Updated only if arrays differ; triggers restart flag (lines 288-301)
4. Some fields trigger `restartReasons` list -- changes that require server restart
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
- **Request body:** `GlobalProxyConfig` -- full replace
- **Validation:** URL required when enabled, only HTTP type currently supported
- **Notes:** Triggers restart_required flag. Redacted password values are preserved from existing config.

---

### Health API

#### GET /health

- **Handler:** `getHealth` (health.go:32)
- **Response:** `{"status": "ok", "components": {"db_pings": "ok"}}` or `{"db_pings": "disabled"}`
- **Notes:** Pings config store, log store, and vector store concurrently with 10s timeout. Returns 503 if any store is unavailable. DB pings can be disabled via `DisableDBPingsInHealth` in ClientConfig.
