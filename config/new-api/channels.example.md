# New-API Channel Configuration Reference

## Channel Strategy Overview

New-API organizes provider access through "channels." Each channel is an upstream
endpoint with a set of supported models, priority, and authentication.

In this architecture, all channels point to the **same Bifrost instance** at
`http://bifrost:8080`. Routing differentiation happens through:

1. **Model names** — Bifrost routes based on the model name in the request
2. **Virtual keys** — Bifrost can apply different policies per virtual key
3. **Routing rules** — Bifrost's CEL-based rules match models to providers

We define three channels corresponding to the three route policy tiers:

```
bifrost-premium   → Official API providers only (Anthropic, OpenAI)
bifrost-standard  → Official preferred, ClewdR as last-resort fallback
bifrost-risky     → ClewdR primary, official as backup (lab/experimental)
```

## Channel Definitions

Create these channels in New-API's admin UI (`https://your-domain/` -> Channels)
or via the New-API admin API.

### Channel: bifrost-premium

For production, paying-customer traffic. Only official providers.

| Field | Value | Notes |
|-------|-------|-------|
| Name | `bifrost-premium` | Display name in admin UI |
| Type | `1` (OpenAI) | Bifrost exposes OpenAI-compatible endpoints |
| Base URL | `http://bifrost:8080` | Docker internal DNS |
| Key | _(virtual key or any non-empty string)_ | See "Key field" notes below |
| Models | `claude-sonnet,claude-opus,claude-haiku,gpt-4o,gpt-4o-mini` | Comma-separated logical model names |
| Model Mapping | See JSON below | Maps logical names to upstream model IDs |
| Priority | `0` (highest) | Lower number = tried first |
| Weight | `1` | For load balancing when multiple channels share models |
| Status | `1` (enabled) | |

Model mapping JSON (paste into the "Model Mapping" field):
```json
{
  "claude-sonnet": "claude-sonnet-4-20250514",
  "claude-opus": "claude-opus-4-20250514",
  "claude-haiku": "claude-haiku-4-5-20251001",
  "gpt-4o": "gpt-4o",
  "gpt-4o-mini": "gpt-4o-mini"
}
```

### Channel: bifrost-standard

For internal tools and cost-optimized workloads. Official preferred, ClewdR fallback allowed.

| Field | Value | Notes |
|-------|-------|-------|
| Name | `bifrost-standard` | |
| Type | `1` (OpenAI) | |
| Base URL | `http://bifrost:8080` | |
| Key | _(virtual key or any non-empty string)_ | |
| Models | `cheap-chat,fast-chat` | |
| Model Mapping | See JSON below | |
| Priority | `5` | Medium priority |
| Weight | `1` | |
| Status | `1` (enabled) | |

Model mapping JSON:
```json
{
  "cheap-chat": "claude-sonnet-4-20250514",
  "fast-chat": "claude-haiku-4-5-20251001"
}
```

### Channel: bifrost-risky

For lab experiments and developer testing. ClewdR primary. No SLA.

| Field | Value | Notes |
|-------|-------|-------|
| Name | `bifrost-risky` | |
| Type | `1` (OpenAI) | |
| Base URL | `http://bifrost:8080` | |
| Key | _(virtual key or any non-empty string)_ | |
| Models | `lab-claude,lab-claude-opus` | |
| Model Mapping | See JSON below | |
| Priority | `10` | Lowest priority |
| Weight | `1` | |
| Status | `1` (enabled) | |

Model mapping JSON:
```json
{
  "lab-claude": "claude-sonnet-4-20250514",
  "lab-claude-opus": "claude-opus-4-20250514"
}
```

## Notes

### Channel Type

Channel Type `1` is "OpenAI" in New-API's constant system (`constant/channel_type.go`).
This is the correct type for Bifrost since Bifrost exposes OpenAI-compatible
`/v1/chat/completions` endpoints. All traffic between New-API and Bifrost uses
the OpenAI request/response format regardless of the actual downstream provider.

### Key Field

The "Key" field in New-API channels is sent as the `Authorization: Bearer <key>`
header to the upstream (Bifrost).

- If Bifrost has `enforce_auth_on_inference: false` (default), the key can be
  any non-empty string (e.g., `sk-placeholder`).
- If Bifrost has auth enabled, use a valid Bifrost virtual key. You can create
  different virtual keys per tier in Bifrost's governance config to enable
  per-tier rate limiting and budget controls.

### Model Mapping

Model mapping translates the logical model name (what clients request) to the
upstream model ID (what Bifrost/providers expect).

- **Left side**: Logical model name visible to clients (e.g., `claude-sonnet`)
- **Right side**: Actual model ID sent to Bifrost (e.g., `claude-sonnet-4-20250514`)

When upstream model versions change, update the right side of the mapping.
Clients continue using the stable logical names.

### Priority vs Weight

- **Priority**: Determines selection order when multiple channels can serve the
  same model. Lower number = higher priority. Use this to prefer premium over
  standard channels.
- **Weight**: For load balancing between channels with the same priority. Only
  relevant when multiple channels at the same priority level serve the same model.

### Admin API Alternative

Channels can also be created via New-API's admin REST API:

```bash
# PSEUDOCONFIG: Verify endpoint path and field names against your New-API version
curl -X POST https://your-domain/api/channel/ \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "bifrost-premium",
    "type": 1,
    "base_url": "http://bifrost:8080",
    "key": "sk-placeholder",
    "models": "claude-sonnet,claude-opus,claude-haiku,gpt-4o,gpt-4o-mini",
    "model_mapping": "{\"claude-sonnet\":\"claude-sonnet-4-20250514\",\"claude-opus\":\"claude-opus-4-20250514\",\"claude-haiku\":\"claude-haiku-4-5-20251001\",\"gpt-4o\":\"gpt-4o\",\"gpt-4o-mini\":\"gpt-4o-mini\"}",
    "priority": 0,
    "weight": 1,
    "status": 1
  }'
```
