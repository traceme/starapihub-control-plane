# Provider Model Mapping Contract

## Purpose

StarAPIHub uses three distinct layers of model names. This document explains what each layer is, why all three exist, and what breaks if they are confused.

This is especially important for aggregator providers like OpenRouter where the public model ID, the internal routing ID, and the provider-native ID are all different strings.

## The Three Layers

```
Layer 1: Public Model ID     (what clients send to New-API)
Layer 2: Channel-Mapped ID   (what Bifrost receives after New-API channel mapping)
Layer 3: Provider-Native ID   (what the upstream provider API actually accepts)
```

### Layer 1: Public Model ID

- **Defined in:** `policies/models.yaml` (logical model name) and `policies/channels.yaml` (channel `models` list)
- **Seen by:** Clients, New-API `/v1/models` endpoint, New-API billing
- **Example:** `openai/gpt-5.4`

This is the stable, operator-chosen name exposed to API consumers. It does not need to match any vendor's naming convention.

### Layer 2: Channel-Mapped ID

- **Defined in:** `policies/channels.yaml` → `model_mapping` field
- **Seen by:** Bifrost (this is the model ID Bifrost receives in the request)
- **Example:** `openrouter/openai/gpt-5.4`

New-API rewrites the model ID before forwarding to Bifrost. The mapping is configured per-channel. This is how one public model name can route to different Bifrost providers depending on which channel handles the request.

### Layer 3: Provider-Native ID

- **Defined in:** `policies/providers.yaml` (key `models` list) and `policies/routing-rules.yaml` (CEL `model` field and target `model` override)
- **Seen by:** The upstream provider API (OpenRouter, Anthropic, OpenAI, etc.)
- **Example:** `openai/gpt-5.4` (what OpenRouter's API expects)

This is what the provider's API actually accepts. For direct providers (Anthropic, OpenAI), this is the vendor model ID. For aggregators (OpenRouter), this includes the vendor prefix that OpenRouter uses internally.

## Why Three Layers?

### Direct Providers (Anthropic, OpenAI)

For direct providers, layers are simple:

| Layer | Example (Claude Sonnet) |
|-------|------------------------|
| Public | `claude-sonnet` |
| Channel-mapped | `claude-sonnet-4-20250514` |
| Provider-native | `claude-sonnet-4-20250514` |

The channel mapping translates a stable public name to a specific model version. Layers 2 and 3 are the same.

### Aggregator Providers (OpenRouter)

For aggregators, Bifrost needs a provider-qualified model name to route correctly:

| Layer | Example (GPT-5.4 via OpenRouter) |
|-------|----------------------------------|
| Public | `openai/gpt-5.4` |
| Channel-mapped | `openrouter/openai/gpt-5.4` |
| Provider-native | `openai/gpt-5.4` |

The `openrouter/` prefix tells Bifrost which provider to use. Bifrost strips the prefix before sending the request to the actual OpenRouter API. Without this prefix, Bifrost cannot distinguish `openai/gpt-5.4` (direct OpenAI) from `openai/gpt-5.4` (via OpenRouter).

## OpenRouter Mapping in Detail

```
Client sends to New-API:
  model: "openai/gpt-5.4"
         ↓
New-API channel bifrost-openrouter applies model_mapping:
  "openai/gpt-5.4" → "openrouter/openai/gpt-5.4"
         ↓
Bifrost receives the prefixed string:
  "openrouter/openai/gpt-5.4"
         ↓
Bifrost calls ParseModelString() (core/schemas/utils.go):
  splits on first "/" only when prefix is a known provider
  result: provider = "openrouter", model = "openai/gpt-5.4"
         ↓
CEL routing rules evaluate against the parsed fields:
  model = "openai/gpt-5.4"   (stripped, not prefixed)
  provider = "openrouter"
         ↓
Rule matches: model == "openai/gpt-5.4"
  target: provider "openrouter", model "openai/gpt-5.4"
         ↓
OpenRouter provider sends to OpenRouter API:
  model: "openai/gpt-5.4"    (the provider-native ID)
```

## What the Routing Rules Actually Match

CEL routing rules match against **parsed** fields, not the raw prefixed string. Bifrost's `ParseModelString()` splits the channel-mapped string `openrouter/openai/gpt-5.4` into:

- `provider` = `"openrouter"` (the known-provider prefix)
- `model` = `"openai/gpt-5.4"` (everything after the first `/`)

These parsed values are injected into the CEL evaluation context (`plugins/governance/routing.go`). So:

```yaml
direct-openrouter-gpt54:
  cel_expression: 'model == "openai/gpt-5.4"'
  targets:
    - provider: "openrouter"
      model: "openai/gpt-5.4"
```

The CEL expression matches `model` = `"openai/gpt-5.4"` — the **stripped** model name, not `"openrouter/openai/gpt-5.4"`.

**Do not write CEL expressions against the prefixed string** (e.g., `model == "openrouter/openai/gpt-5.4"`). That will never match because Bifrost has already parsed the prefix into the `provider` field.

If you need to match on provider in CEL, use: `provider == "openrouter" && model == "openai/gpt-5.4"`.

## What Breaks If the Mapping Is Wrong

| Mistake | Symptom |
|---------|---------|
| Missing `openrouter/` prefix in channel mapping | Bifrost receives raw `openai/gpt-5.4`, `ParseModelString` checks if `openai` is a known provider — if it is, wrong provider selected; if not, no provider extracted, routing fails |
| Wrong prefix (e.g., `openai/openai/gpt-5.4`) | Bifrost parses provider=`openai`, model=`openai/gpt-5.4`, routes to OpenAI instead of OpenRouter |
| CEL rule matches the prefixed string (`openrouter/openai/gpt-5.4`) | Rule never matches — the `model` CEL variable is always the stripped value |
| `allow_direct_keys: true` in providers.yaml | Bifrost treats New-API's auth header as a direct provider key, bypasses OpenRouter provider config |
| Channel mapping present but routing rule missing | Bifrost parses the model correctly but no CEL rule matches, falls through to default routing |
| Provider key model list missing the model | Bifrost may reject the request even if routing matches |

## Complete Mapping Table

| Public Model ID | Channel | Channel-Mapped ID | Routing Rule | Provider | Provider-Native ID |
|----------------|---------|-------------------|-------------|----------|-------------------|
| `claude-sonnet` | bifrost-premium | `claude-sonnet-4-20250514` | premium-claude | anthropic | `claude-sonnet-4-20250514` |
| `claude-opus` | bifrost-premium | `claude-opus-4-20250514` | premium-claude | anthropic | `claude-opus-4-20250514` |
| `claude-haiku` | bifrost-premium | `claude-haiku-4-5-20251001` | premium-claude | anthropic | `claude-haiku-4-5-20251001` |
| `gpt-4o` | bifrost-premium | `gpt-4o` | premium-openai | openai | `gpt-4o` |
| `gpt-4o-mini` | bifrost-premium | `gpt-4o-mini` | premium-openai | openai | `gpt-4o-mini` |
| `openai/gpt-5.4` | bifrost-openrouter | `openrouter/openai/gpt-5.4` | direct-openrouter-gpt54 | openrouter | `openai/gpt-5.4` |
| `anthropic/claude-opus-4.6` | bifrost-openrouter | `openrouter/anthropic/claude-opus-4.6` | direct-openrouter-opus46 | openrouter | `anthropic/claude-opus-4.6` |
| `cheap-chat` | bifrost-standard | `claude-sonnet-4-20250514` | standard-claude | anthropic (ClewdR fallback) | `claude-sonnet-4-20250514` |
| `fast-chat` | bifrost-standard | `claude-haiku-4-5-20251001` | standard-claude | anthropic (ClewdR fallback) | `claude-haiku-4-5-20251001` |
| `lab-claude` | bifrost-risky | `claude-sonnet-4-20250514` | risky-claude | clewdr-{1,2,3} (anthropic fallback) | `claude-sonnet-4-20250514` |
| `lab-claude-opus` | bifrost-risky | `claude-opus-4-20250514` | risky-claude | clewdr-{1,2,3} (anthropic fallback) | `claude-opus-4-20250514` |

## Rules for Adding New Mappings

1. **Choose the public model ID** — this is what clients see. Keep it stable.
2. **Add channel model mapping** — map public ID to the provider-qualified internal ID.
3. **Add a routing rule** — match the internal ID and target the correct provider.
4. **Add the model to the provider key's model list** — Bifrost checks this for authorization.
5. **Add pricing** — New-API needs ratios for billing.
6. **Sync and smoke test** — `starapihub sync` then verify with a real request.

## Related Docs

- [OpenRouter Operations](openrouter-operations.md) — setup, rotation, troubleshooting
- [Policies](policies.md) — registry structure and validation
- [Bifrost Integration](bifrost-integration.md) — provider and routing config
- [New-API Integration](new-api-integration.md) — channel and model visibility
