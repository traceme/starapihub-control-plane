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

    // IMPORTANT: ClewdR instances are registered as separate "openai" providers
    // with custom base URLs. Since Bifrost's provider key is the provider type
    // (e.g., "openai"), and you can only have one config per type,
    // ClewdR instances should be registered using Bifrost's custom provider
    // feature or as separate keys within the "openai" provider.
    //
    // Option A: Add ClewdR as extra keys under an OpenAI-compatible custom provider
    // Option B: Use Bifrost's Web UI to create custom provider entries

    // Using custom provider approach (PSEUDOCONFIG — verify with Bifrost docs):
    "openai": {
      "keys": [
        // Official OpenAI key
        {
          "id": "openai-official",
          "name": "OpenAI Official",
          "value": "sk-...",
          "models": ["gpt-4o", "gpt-4o-mini"],
          "weight": 1.0
        },
        // ClewdR instances as OpenAI-compatible keys with base_url override
        // NOTE: Per-key base_url may require custom_provider_config
        // Check your Bifrost version for exact support
        {
          "id": "clewdr-1",
          "name": "ClewdR Instance 1",
          "value": "clewdr-admin-password-1",
          "models": ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
          "weight": 0.5
        }
      ],
      "network_config": {
        "base_url": ""  // Default for official OpenAI
      },
      // Custom provider config for ClewdR (PSEUDOCONFIG)
      "custom_provider_config": {
        "base_provider": "openai"
      }
    }
  }
}
```

### Recommended Approach: Bifrost Web UI

Given the complexity of mixing official and unofficial providers, the recommended approach for initial setup is:

1. Start Bifrost with a minimal config.json containing official provider keys
2. Use the Bifrost Web UI (http://bifrost:8080) to:
   - Add ClewdR instances as custom OpenAI-compatible providers
   - Configure routing rules per model
   - Set up virtual keys for New-API to use
3. Export the resulting config for version control

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

The control plane generates config.json templates in `config/bifrost/`. See `docs/config-sync.md` for the sync workflow.
