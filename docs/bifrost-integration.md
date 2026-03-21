# Bifrost Integration Guide

## Role

Bifrost is the **internal routing engine** and **execution truth**. It handles:
- Provider selection and load balancing
- Retry logic and fallback across providers
- Circuit breaking for unhealthy providers
- Optional response caching (semantic cache)
- Provider-side observability and logging

## Integration Strategy: config.json

Bifrost HTTP transport loads configuration from a `config.json` file in its app data directory (default: `/app/data/config.json`). It can also be configured through its Web UI and stored in a SQLite database (`config.db`).

The control plane's job is to generate or guide the creation of this config.

## config.json Structure

Bifrost HTTP transport loads `config.json` from the `-app-dir` directory (default `/app/data/`). It also stores state in `config.db` (SQLite) alongside it.

The config has three main sections: `providers`, `governance`, and `client_config`.

### Providers Section

Keyed by Bifrost's `ModelProvider` string. Each provider has `keys` and `network_config`:

```jsonc
{
  "$schema": "https://www.getbifrost.ai/schema",
  "providers": {
    "anthropic": {
      "keys": [
        {
          "id": "anthropic-key-1",          // Unique ID
          "name": "Primary Anthropic Key",  // Display name
          "value": "env.ANTHROPIC_API_KEY", // "env.VAR" syntax loads from env
          "models": ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
          "weight": 1.0,                    // Load balancing weight
          "enabled": true                   // Can be disabled without removing
        }
      ],
      "network_config": {
        "default_request_timeout_in_seconds": 60,
        "max_retries": 2,
        "retry_backoff_initial": 500,       // milliseconds in JSON
        "retry_backoff_max": 5000,          // milliseconds in JSON
        "stream_idle_timeout_in_seconds": 60
      }
    }
  }
}
```

Supported provider types: `openai`, `anthropic`, `azure`, `bedrock`, `vertex`, `gemini`, `cohere`, `mistral`, `groq`, `ollama`, `openrouter`, `perplexity`, `cerebras`, `elevenlabs`, `huggingface`, `nebius`, `xai`, `replicate`, `vllm`, `runway`, `sgl`, `parasail`.

### Governance Section

Contains virtual keys, **routing rules with CEL expressions**, model configs, and auth:

```jsonc
{
  "governance": {
    "virtual_keys": [
      // Per-consumer keys with allowed providers, budgets, rate limits
    ],
    "routing_rules": [
      {
        "id": "rule-lab-to-clewdr",
        "name": "Route lab models to ClewdR",
        "enabled": true,
        "cel_expression": "model.startsWith('lab-')",  // CEL expression for matching
        "targets": [
          { "provider": "clewdr-custom", "weight": 1.0 }
        ],
        "fallbacks": ["anthropic"],   // Fallback provider chain
        "scope": "global",            // "global" | "team" | "customer" | "virtual_key"
        "priority": 10                // Lower = evaluated first
      }
    ],
    "model_configs": [],
    "auth_config": {
      "is_enabled": false,
      "admin_username": { "env": "BIFROST_ADMIN_USER" },
      "admin_password": { "env": "BIFROST_ADMIN_PASS" }
    }
  }
}
```

**CEL expression examples** (for routing_rules):
- `model.startsWith('gpt-4')` — match by model prefix
- `model == 'claude-sonnet-4-20250514'` — exact model match
- `request_type == 'embedding'` — match by request type
- `virtual_key_name == 'premium-tier'` — match by virtual key

### Key Go Types (for reference)

| Go Type | File | JSON Section |
|---------|------|-------------|
| `configstore.ProviderConfig` | `framework/configstore/clientconfig.go` | `providers.<type>` |
| `schemas.Key` | `core/schemas/account.go` | `providers.<type>.keys[]` |
| `schemas.NetworkConfig` | `core/schemas/provider.go` | `providers.<type>.network_config` |
| `configstore.GovernanceConfig` | `framework/configstore/clientconfig.go` | `governance` |
| `tables.TableRoutingRule` | `framework/configstore/tables/routing_rules.go` | `governance.routing_rules[]` |
| `configstore.ClientConfig` | `framework/configstore/clientconfig.go` | `client_config` |

### Auto-Detection

If no `config.json` exists, Bifrost auto-detects provider keys from environment variables: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `MISTRAL_API_KEY`, etc.

## ClewdR as a Bifrost Provider

ClewdR exposes OpenAI-compatible endpoints, so it appears to Bifrost as an OpenAI-type provider with a custom `base_url`:

```jsonc
{
  "providers": {
    // ... official providers above ...

    // ClewdR instances MUST be registered as SEPARATE custom providers.
    // Bifrost Keys do NOT have a per-key base_url field -- base_url is on
    // NetworkConfig at the provider level. Therefore, each ClewdR instance
    // needs its own provider entry with its own NetworkConfig.BaseURL.
    //
    // Register each ClewdR instance via POST /api/providers with
    // custom_provider_config.base_provider_type = "openai".
    // Custom provider names must NOT match standard provider names.
    //
    // _Corrected 2026-03-21: verified against core/schemas/provider.go:382-388
    // and core/schemas/account.go:15-32. See capability-audit.md for full
    // struct definitions._

    // Official OpenAI provider (standard provider, not custom):
    "openai": {
      "keys": [
        {
          "id": "openai-official",
          "name": "OpenAI Official",
          "value": "env.OPENAI_API_KEY",
          "models": ["gpt-4o", "gpt-4o-mini"],
          "weight": 1.0,
          "enabled": true
        }
      ],
      "network_config": {
        "default_request_timeout_in_seconds": 30,
        "max_retries": 2,
        "retry_backoff_initial": 500,
        "retry_backoff_max": 5000
      }
    }
    // ClewdR instances are added separately via POST /api/providers:
    //
    // curl -X POST http://bifrost:8080/api/providers \
    //   -H "Content-Type: application/json" \
    //   -d '{
    //     "provider": "clewdr-1",
    //     "keys": [{
    //       "id": "clewdr-1-key",
    //       "name": "ClewdR Instance 1",
    //       "value": "env.CLEWDR_1_PASSWORD",
    //       "models": ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
    //       "weight": 1.0,
    //       "enabled": true
    //     }],
    //     "network_config": {
    //       "base_url": "http://clewdr-1:8484",
    //       "default_request_timeout_in_seconds": 120,
    //       "max_retries": 0,
    //       "stream_idle_timeout_in_seconds": 120
    //     },
    //     "custom_provider_config": {
    //       "base_provider_type": "openai"
    //     }
    //   }'
  }
}
```

### Recommended Approach: API-Based Registration

ClewdR instances are registered as custom providers via the Bifrost admin API. Each instance is a separate provider with `custom_provider_config.base_provider_type = "openai"` and its own `network_config.base_url`. The sync engine (Phase 3) will automate this via `POST /api/providers` for new instances and `PUT /api/providers/{provider}` for updates.

For initial manual setup, either:
1. Use the Bifrost Web UI (http://bifrost:8080) to add custom providers interactively
2. Use `curl` commands as shown in the config example above

_Corrected 2026-03-21: verified against providers.go:177 (POST /api/providers) and core/schemas/provider.go:382-388 (CustomProviderConfig struct). See capability-audit.md for full API documentation._

### Provider Isolation

To enforce the premium/standard/risky separation at the Bifrost level:

1. **Virtual keys**: Create separate Bifrost virtual keys for each tier. New-API's `bifrost-premium` channel uses VK-1 (which only allows official providers), `bifrost-risky` uses VK-3 (which allows ClewdR).

2. **Routing rules**: Configure routing rules that restrict which providers can be used based on the virtual key or model name.

3. **Model configs**: Use model-level configuration to set per-model behavior (caching, timeout overrides).

## Health Monitoring

Bifrost health endpoint: `GET /health`

Bifrost automatically monitors provider health via:
- Response status codes
- Timeout tracking
- Circuit breaker state

No external health checks needed for individual providers — Bifrost handles this internally.

## Config Sync Options

| Method | When to Use |
|--------|------------|
| **Mount config.json** | Initial deployment, CI/CD pipelines. Mount the file into the container. |
| **Web UI** | Ad-hoc changes, initial provider setup. Access at http://bifrost:8080. |
| **Database** | Persistent config stored in config.db. Survives container restarts. |

The control plane generates config.json templates in `config/bifrost/`. See `docs/config-sync.md` for the sync workflow and `docs/capability-audit.md` for the authoritative API reference with verified struct definitions.
