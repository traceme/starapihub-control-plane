# New-API Integration Guide

## Role

New-API is the **public-facing gateway** and **billing truth**. It handles:
- User authentication (JWT, API keys, OAuth)
- Token/balance management and quota enforcement
- Model visibility (which models users can see)
- Usage recording and billing
- Admin dashboard

## Integration Strategy: Channel-Based Routing

Instead of configuring one New-API channel per provider (which would bypass Bifrost), we configure a **small number of channels that all point to Bifrost**. Bifrost then handles the complex provider routing.

### Recommended Channels

| Channel Name | Channel Type | Base URL | Purpose |
|-------------|-------------|----------|---------|
| `bifrost-premium` | OpenAI-compatible (type 1) | `http://bifrost:8080/v1` | Premium models — official providers only |
| `bifrost-standard` | OpenAI-compatible (type 1) | `http://bifrost:8080/v1` | Standard models — official preferred, unofficial fallback |
| `bifrost-risky` | OpenAI-compatible (type 1) | `http://bifrost:8080/v1` | Lab/experimental — ClewdR primary |

All three channels use the same Bifrost endpoint. The distinction is:
1. **Model mapping**: Each channel is configured with different model names
2. **Billing rate**: Each channel can have different token pricing
3. **Bifrost routing**: Bifrost uses the model name (or headers) to apply different route policies

### Channel Configuration (Manual via Admin UI)

New-API channels are created through the admin UI at `https://your-domain/`. There is also an admin API.

**To create a channel via the Admin UI**:

1. Log in as admin
2. Go to Channels → Add Channel
3. Configure:
   - **Name**: `bifrost-premium`
   - **Type**: Select `OpenAI` (type 1 — OpenAI-compatible)
   - **Base URL**: `http://bifrost:8080`
   - **Key**: A shared key or empty (Bifrost can be configured to accept unauthenticated requests from the internal network)
   - **Models**: List the logical model names for this tier (e.g., `claude-sonnet`, `claude-opus`, `gpt-4o`)
   - **Model Mapping**: Map logical names to upstream names if needed (e.g., `claude-sonnet` → `claude-sonnet-4-20250514`)
4. Set priority and weight if using multiple channels of the same tier
5. Save and test

**To create a channel via the Admin API**:

```bash
# New-API admin API for channel management
# Endpoint: POST /api/channel/ (uses AddChannelRequest wrapper)
# Auth: AdminAuth (session cookie or Authorization: Bearer <admin-token>)
# Full CRUD: GET/POST/PUT/DELETE /api/channel/
# Verified: controller/channel.go AddChannel handler, see capability-audit.md

curl -X POST https://your-domain/api/channel/ \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "single",
    "channel": {
      "name": "bifrost-premium",
      "type": 1,
      "base_url": "http://bifrost:8080",
      "key": "sk-placeholder",
      "models": "claude-sonnet,claude-opus,claude-haiku,gpt-4o,gpt-4o-mini",
      "model_mapping": "{\"claude-sonnet\":\"claude-sonnet-4-20250514\",\"claude-opus\":\"claude-opus-4-20250514\"}",
      "priority": 0,
      "weight": 1,
      "group": "default",
      "status": 1
    }
  }'
```

_Corrected 2026-03-21: POST /api/channel/ uses AddChannelRequest struct with `mode` and `channel` fields (verified: controller/channel.go:566). See capability-audit.md._

**Channel model fields** (defined in `new-api/model/channel.go`):

| Field | Type | Purpose |
|-------|------|---------|
| `type` | int | Channel type — `1` = OpenAI-compatible (correct for Bifrost) |
| `key` | string | API key (can be empty if Bifrost auth is off) |
| `base_url` | string | Upstream URL (e.g., `http://bifrost:8080`) |
| `models` | string | Comma-separated model list |
| `model_mapping` | string | JSON string mapping logical→upstream names |
| `priority` | int | Selection priority (lower = preferred) |
| `weight` | int | Load balancing weight |
| `status` | int | `1`=enabled, `2`=manually disabled, `3`=auto-disabled |
| `group` | string | Channel group (default: `"default"`) |
| `auto_ban` | int | Auto-disable on errors (`1`=yes, `0`=no) |
| `tag` | string | Optional tag for categorization |
| `setting` | string | JSON: `ChannelSettings` (force_format, proxy, system_prompt, etc.) |
| `param_override` | string | JSON: request parameter overrides |
| `header_override` | string | JSON: HTTP header overrides |

Other useful admin API endpoints:
- `PUT /api/channel/` — update existing channel
- `DELETE /api/channel/:id` — delete channel
- `GET /api/channel/test/:id` — test channel connectivity
- `GET /api/channel/search` — search channels
- `POST /api/channel/copy/:id` — duplicate a channel

### Model Mapping Strategy

New-API supports model mapping per channel. This is where logical model names are translated to provider model IDs:

```json
{
  "claude-sonnet": "claude-sonnet-4-20250514",
  "claude-opus": "claude-opus-4-20250514",
  "claude-haiku": "claude-haiku-4-5-20251001",
  "gpt-4o": "gpt-4o",
  "gpt-4o-mini": "gpt-4o-mini",
  "cheap-chat": "claude-sonnet-4-20250514",
  "fast-chat": "claude-haiku-4-5-20251001",
  "lab-claude": "claude-sonnet-4-20250514",
  "lab-claude-opus": "claude-opus-4-20250514"
}
```

This mapping should be consistent with `policies/logical-models.example.yaml`.

### Billing Configuration

Token pricing is set per-model in New-API via the admin API. Recommended pricing tiers:

| Channel | Pricing Strategy |
|---------|-----------------|
| `bifrost-premium` | Market rate (matches official API pricing) |
| `bifrost-standard` | 70% of market rate (reflects potential unofficial fallback) |
| `bifrost-risky` | 30% of market rate (primarily uses unofficial providers) |

Set model prices via `PUT /api/option/` with `key=ModelRatio` and a JSON-encoded map as the value. The endpoint accepts RootAuth. See `docs/capability-audit.md` -- System Options section for the full list of pricing-relevant option keys (ModelRatio, ModelPrice, CompletionRatio, CacheRatio, etc.).

_Corrected 2026-03-21: PUT /api/option/ verified in controller/option.go:105. See capability-audit.md for full API documentation._

### What Stays in New-API

- User accounts and authentication
- Token management and API key issuance
- Balance and quota management
- Usage logging and billing records
- Model visibility rules (which users see which models)
- Rate limiting per token/user

### What Stays Out of New-API

- Provider API keys (these go in Bifrost)
- Provider routing logic (Bifrost handles this)
- ClewdR cookie management (ClewdR handles this)
- Failover and load balancing decisions (Bifrost handles this)

## Config Files Reference

See `config/new-api/` for:
- `channels.example.md` — Channel configuration reference
- `model-mapping.example.md` — Model name mapping reference

For authoritative API shapes (channel struct, option struct, auth levels), see `docs/capability-audit.md` -- New-API Admin API section.

_Corrected 2026-03-21: All endpoint paths, request shapes, and auth levels verified against new-api source code. See capability-audit.md for full documentation._
