# Provider Onboarding Guide

## Purpose

This document defines the repeatable process for adding a new provider to StarAPIHub through the canonical policy registries and sync engine. It generalizes the pattern proven by the OpenRouter integration.

A provider is fully onboarded when:

- requests route through it end-to-end
- its configuration is registry-driven and sync-managed
- an operator can understand and maintain it from docs alone

## Prerequisites

Before onboarding a new provider:

1. You have a valid API key for the provider
2. The StarAPIHub stack is running with Bifrost accessible
3. You understand the provider's model naming convention
4. You know whether this is a **direct provider** (Anthropic, OpenAI) or an **aggregator** (OpenRouter)

## Decision: Direct vs Aggregator

| Property | Direct Provider | Aggregator Provider |
|----------|----------------|-------------------|
| Examples | Anthropic, OpenAI | OpenRouter |
| API endpoint | Provider's own (auto-detected by Bifrost) | Provider's unified endpoint |
| Model names | Single vendor namespace | Multi-vendor namespace (e.g., `openai/gpt-5.4`) |
| Channel mapping | Public name → version-pinned model | Public name → `provider-prefix/vendor/model` |
| CEL routing | Match on model name | Match on parsed model name (prefix stripped) |
| `allow_direct_keys` impact | Usually safe either way | Must be `false` for brokered routing |

For aggregators, read [provider-model-mapping.md](provider-model-mapping.md) before proceeding — the three-layer naming contract is critical.

## Onboarding Checklist

### Step 1: Add the provider to `policies/providers.yaml`

Define the provider with its key, models, and network config.

```yaml
new-provider:
  description: "Description of the provider"
  keys:
    - id: new-provider-primary
      name: "New Provider Primary"
      value_env: NEW_PROVIDER_API_KEY    # env var name, NOT the secret
      models:
        - model-a
        - model-b
      weight: 1.0
      enabled: true
  network_config:
    default_request_timeout_in_seconds: 60
    max_retries: 2
    retry_backoff_initial_ms: 500
    retry_backoff_max_ms: 5000
```

For aggregators, also include `concurrency_and_buffer_size` if needed.

For custom/self-hosted providers, include `custom_provider_config` and `base_url` in `network_config`.

**Key rules:**
- Secrets are NEVER stored in this file — only env var names via `value_env`
- Model names in the `models` list are the **provider-native** IDs (what the provider API accepts)
- For aggregators, these may include vendor prefixes (e.g., `openai/gpt-5.4`)

### Step 2: Add the channel to `policies/channels.yaml`

Define how New-API routes to this provider through Bifrost.

```yaml
bifrost-new-provider:
  name: "Bifrost New Provider"
  type: 43                              # OpenAI-compatible
  key_env: BIFROST_API_KEY              # New-API's auth to Bifrost
  base_url: "http://bifrost:8080"
  models: "public-model-a,public-model-b"
  group: "default"
  tag: "new-provider"
  model_mapping:
    public-model-a: "model-a"           # direct provider: public → version-pinned
    public-model-b: "model-b"
  priority: 15
  weight: 3
  status: 1
  auto_ban: 1
```

**For aggregators**, the model mapping must add the provider prefix:

```yaml
model_mapping:
  vendor/model-a: "aggregator/vendor/model-a"
  vendor/model-b: "aggregator/vendor/model-b"
```

This prefix is required because Bifrost uses `ParseModelString()` to extract the provider from the model string. See [provider-model-mapping.md](provider-model-mapping.md).

### Step 3: Add routing rules to `policies/routing-rules.yaml`

Define CEL expressions that match requests to the provider.

```yaml
direct-new-provider-model-a:
  name: "Direct New Provider Model A"
  description: "Route model-a to new-provider"
  enabled: true
  cel_expression: 'model == "model-a"'
  targets:
    - provider: "new-provider"
      model: "model-a"
      weight: 1.0
  fallbacks: []
  scope: "global"
  priority: 25                          # after existing rules
```

**CEL matching rules:**
- The `model` variable in CEL is the **parsed** model name, not the raw prefixed string
- For aggregators: Bifrost strips the provider prefix before CEL evaluation, so match on the stripped name (e.g., `model == "vendor/model-a"`, not `model == "aggregator/vendor/model-a"`)
- The `provider` variable is also available for compound matches

Choose a priority that fits the routing order. Lower number = higher priority. See existing rules for reference.

### Step 4: Add logical models to `policies/models.yaml`

```yaml
public-model-a:
  display_name: "Model A via New Provider"
  billing_name: "public-model-a"
  description: "Model A routed through New Provider"
  upstream_model: "model-a"
  risk_level: medium
  allowed_groups: ["all"]
  channel: bifrost-new-provider
  route_policy: direct-new-provider-model-a
  unofficial_allowed: false
  caching_allowed: true
```

### Step 5: Add pricing to `policies/pricing.yaml`

```yaml
public-model-a:
  model_ratio: 1.0
  completion_ratio: 3.0
```

Pricing ratios are operator-defined. Review the provider's actual billing before using these for production.

### Step 6: Set the API key

Add the env var to `control-plane/deploy/env/bifrost.env`:

```bash
NEW_PROVIDER_API_KEY=sk-your-key-here
```

### Step 7: Validate, sync, and verify

```bash
cd control-plane

# Validate registry YAML
./dashboard/starapihub validate --config-dir policies/

# Export the API key for the sync CLI
export NEW_PROVIDER_API_KEY=sk-your-key-here

# Sync all config to the live system
./dashboard/starapihub sync

# Verify sync is clean
./dashboard/starapihub sync --dry-run
# Should show 0 pending changes

# Smoke test
curl -s -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"public-model-a","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
  http://localhost:3000/v1/chat/completions

# Verify in Bifrost logs
docker logs cp-bifrost --tail 20 | grep new-provider

# Check drift
./dashboard/starapihub diff
```

## Common Failure Patterns

| Symptom | Likely Cause | Diagnosis |
|---------|-------------|-----------|
| Sync fails with "key env var not set" | API key not exported in CLI environment | `echo $NEW_PROVIDER_API_KEY` — must be non-empty |
| Model not found after sync | Channel model mapping missing or incorrect | Check `channels.yaml` `model_mapping` entries |
| Request routes to wrong provider | For aggregators: missing provider prefix in channel mapping | Check [provider-model-mapping.md](provider-model-mapping.md) |
| CEL rule never matches | Rule matches against prefixed string instead of parsed model | Write CEL against the stripped model name |
| 401 from provider | API key invalid or wrong env var name | Check `providers.yaml` `value_env` matches the actual env var name |
| Request hangs | Timeout too short for the provider | Increase `default_request_timeout_in_seconds` in `providers.yaml` |
| Pricing incorrect | Ratios don't match provider's actual billing | Review and update `pricing.yaml` against provider pricing |

## Policy File Summary

Every provider touches these five files:

| File | What to add | Required? |
|------|------------|-----------|
| `policies/providers.yaml` | Provider definition, key, model list, network config | Yes |
| `policies/channels.yaml` | New-API channel with model mapping | Yes |
| `policies/routing-rules.yaml` | CEL routing rules targeting the provider | Yes |
| `policies/models.yaml` | Logical model definitions for client-facing names | Yes |
| `policies/pricing.yaml` | Token cost ratios for billing | Yes |

Plus one env file:

| File | What to add |
|------|------------|
| `deploy/env/bifrost.env` | `PROVIDER_API_KEY=value` |

## Post-Onboarding Verification

After onboarding, verify:

| Check | Command | Expected |
|-------|---------|----------|
| Sync clean | `starapihub sync --dry-run` | 0 pending changes |
| No drift | `starapihub diff` | No blocking drift |
| Model listed | `curl http://localhost:3000/v1/models -H "Authorization: Bearer $API_KEY"` | New models in list |
| Inference works | Send a chat completion request | Valid response |
| Correct provider | `docker logs cp-bifrost --tail 20` | provider=new-provider |
| Correct key | Bifrost logs | key=new-provider-primary |

## Reference Integrations

| Provider | Type | Docs |
|----------|------|------|
| OpenRouter | Aggregator | [openrouter-operations.md](openrouter-operations.md) |
| Anthropic | Direct | `providers.yaml` → `anthropic` section |
| OpenAI | Direct | `providers.yaml` → `openai` section |
| ClewdR | Custom/self-hosted | [clewdr-operations.md](clewdr-operations.md) |

## Related Docs

- [Provider Model Mapping](provider-model-mapping.md) — three-layer naming contract
- [OpenRouter Operations](openrouter-operations.md) — reference aggregator integration
- [Policies](policies.md) — registry structure and validation
- [Runbook](runbook.md) — key rotation and provider management
