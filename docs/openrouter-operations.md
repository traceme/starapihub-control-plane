# OpenRouter Operations Guide

## Overview

OpenRouter is integrated as a brokered provider through Bifrost. It exposes models from multiple upstream vendors (OpenAI, Anthropic, etc.) through a single API key and endpoint.

The request path is:

```
Client -> New-API -> Bifrost -> OpenRouter -> upstream vendor
```

OpenRouter is NOT on the same trust tier as direct official providers. It is an aggregator — pricing, latency, and availability depend on OpenRouter's own routing decisions.

## Current Operating Shape

| Property | Value |
|----------|-------|
| Bifrost provider name | `openrouter` |
| Bifrost key ID | `openrouter-primary` |
| API key env var | `OPENROUTER_API_KEY` |
| API key location | `control-plane/deploy/env/bifrost.env` |
| Global config constraint | `allow_direct_keys: false` |

### Exposed Models

| Public Model ID (New-API) | Channel-Mapped ID (Bifrost sees) | Provider-Native ID (OpenRouter receives) |
|---------------------------|----------------------------------|------------------------------------------|
| `openai/gpt-5.4` | `openrouter/openai/gpt-5.4` | `openai/gpt-5.4` |
| `anthropic/claude-opus-4.6` | `openrouter/anthropic/claude-opus-4.6` | `anthropic/claude-opus-4.6` |

See [provider-model-mapping.md](provider-model-mapping.md) for the full explanation of why three layers of names exist.

### Verified Live Results

Both models have been verified end-to-end:

- `openai/gpt-5.4` returned `OPENROUTER_GPT54_OK`
- `anthropic/claude-opus-4.6` returned `OPENROUTER_OPUS46_OK`

Bifrost logs confirmed provider `openrouter`, key `openrouter-primary`, and routing-rule-based selection.

## Policy Files

OpenRouter configuration spans five policy files:

| File | What it defines | OpenRouter entries |
|------|----------------|-------------------|
| `policies/providers.yaml` | Bifrost provider + key + model list | `openrouter` provider with `openrouter-primary` key |
| `policies/channels.yaml` | New-API channel + model mapping | `bifrost-openrouter` channel with prefix mapping |
| `policies/models.yaml` | Logical model definitions | `openai/gpt-5.4`, `anthropic/claude-opus-4.6` |
| `policies/routing-rules.yaml` | Bifrost CEL routing rules | `direct-openrouter-gpt54`, `direct-openrouter-opus46` |
| `policies/pricing.yaml` | Token cost ratios for billing | `openai/gpt-5.4`, `anthropic/claude-opus-4.6` |

## Setup From Scratch

### Prerequisites

- A valid OpenRouter API key (starts with `sk-or-v1-`)
- The StarAPIHub stack running with Bifrost accessible

### Steps

1. **Set the API key** in `control-plane/deploy/env/bifrost.env`:

   ```bash
   OPENROUTER_API_KEY=sk-or-v1-your-key-here
   ```

2. **Sync configuration** to push the OpenRouter provider (including the resolved key), channel, routing rules, and models to the live system:

   ```bash
   cd control-plane
   export OPENROUTER_API_KEY=sk-or-v1-your-key-here
   ./dashboard/starapihub sync
   ```

   The sync engine resolves `OPENROUTER_API_KEY` from the CLI's environment and pushes the actual value to Bifrost via its HTTP API. No Bifrost restart is needed.

3. **Verify sync is clean**:

   ```bash
   ./dashboard/starapihub sync --dry-run
   # Should show 0 pending changes
   ```

5. **Smoke test** both models:

   ```bash
   # GPT-5.4 via OpenRouter
   curl -s -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"model":"openai/gpt-5.4","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
     http://localhost:3000/v1/chat/completions

   # Claude Opus 4.6 via OpenRouter
   curl -s -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"model":"anthropic/claude-opus-4.6","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
     http://localhost:3000/v1/chat/completions
   ```

6. **Check Bifrost logs** to confirm routing:

   ```bash
   docker logs cp-bifrost --tail 20 | grep openrouter
   # Should show provider=openrouter, key=openrouter-primary
   ```

## Key Rotation

Provider keys are pushed to Bifrost via the sync engine's HTTP API, not consumed from env vars at Bifrost startup. Rotating a key requires a sync operation.

1. Generate a new API key at [openrouter.ai/keys](https://openrouter.ai/keys)
2. Update `OPENROUTER_API_KEY` in `control-plane/deploy/env/bifrost.env`
3. Sync the new key to Bifrost:

   ```bash
   cd control-plane
   # Ensure the env var is loaded (source or export)
   export OPENROUTER_API_KEY=sk-or-v1-your-new-key
   ./dashboard/starapihub sync --target provider
   ```

   The sync engine resolves `OPENROUTER_API_KEY` from `os.Getenv()` at CLI runtime and pushes the actual value to Bifrost via `PUT /api/providers/openrouter`. No Bifrost restart is needed — the API call updates Bifrost's live state.

4. Verify with a smoke request to either OpenRouter model:

   ```bash
   curl -s -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"model":"openai/gpt-5.4","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
     http://localhost:3000/v1/chat/completions
   ```

5. Revoke the old key in the OpenRouter dashboard

## Adding a New OpenRouter Model

To expose a new model through the OpenRouter provider:

1. **Add the model to the provider key's model list** in `policies/providers.yaml`:

   ```yaml
   openrouter:
     keys:
       - id: openrouter-primary
         models:
           - openai/gpt-5.4
           - anthropic/claude-opus-4.6
           - vendor/new-model-id        # add here
   ```

2. **Add a channel model mapping** in `policies/channels.yaml`:

   ```yaml
   bifrost-openrouter:
     models: "openai/gpt-5.4,anthropic/claude-opus-4.6,vendor/new-model-id"
     model_mapping:
       vendor/new-model-id: "openrouter/vendor/new-model-id"
   ```

3. **Add a routing rule** in `policies/routing-rules.yaml`:

   ```yaml
   direct-openrouter-newmodel:
     name: "Direct OpenRouter New Model"
     enabled: true
     cel_expression: 'model == "vendor/new-model-id"'
     targets:
       - provider: "openrouter"
         model: "vendor/new-model-id"
         weight: 1.0
     fallbacks: []
     scope: "global"
     priority: 27    # after existing OpenRouter rules
   ```

4. **Add a logical model** in `policies/models.yaml`

5. **Add pricing** in `policies/pricing.yaml`

6. **Sync and verify**:

   ```bash
   starapihub validate --config-dir policies/
   starapihub sync
   starapihub sync --dry-run   # confirm 0 pending
   # smoke test the new model
   ```

## Critical Constraints

### allow_direct_keys Must Be false

`providers.yaml` sets `allow_direct_keys: false` globally. This is required because New-API authenticates to Bifrost with a `BIFROST_API_KEY` header. If Bifrost treats that as a direct provider key, it bypasses the configured OpenRouter provider key and breaks brokered routing.

**Do not change this setting** without understanding its impact on all provider paths.

### Channel Model Mapping Is Required

The `bifrost-openrouter` channel in `channels.yaml` maps public model IDs to prefixed internal IDs:

```yaml
model_mapping:
  openai/gpt-5.4: "openrouter/openai/gpt-5.4"
  anthropic/claude-opus-4.6: "openrouter/anthropic/claude-opus-4.6"
```

This mapping exists because Bifrost's OpenRouter provider requires the `openrouter/` prefix to route correctly. Without this mapping, requests fail with model-not-found errors. See [provider-model-mapping.md](provider-model-mapping.md) for details.

### Pricing Ratios Are Operator-Defined Estimates

The ratios in `pricing.yaml` for OpenRouter models are **not audited against OpenRouter's actual billing**. They are operator-defined estimates based on vendor list pricing at the time of integration.

Before using these ratios for production billing:

1. Check current pricing at [openrouter.ai/models](https://openrouter.ai/models)
2. Compare with the ratios in `pricing.yaml`
3. Adjust ratios to match your billing policy (pass-through, markup, etc.)

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| 401 from OpenRouter | API key invalid or expired | Rotate key (see Key Rotation above) |
| Model not found | Channel mapping missing `openrouter/` prefix | Check `channels.yaml` model_mapping |
| Bifrost uses wrong key | `allow_direct_keys: true` | Set to `false` in `providers.yaml`, re-sync |
| Routing rule not matching | CEL expression mismatch or wrong priority | Check `routing-rules.yaml` priority order |
| Billing incorrect | Pricing ratios out of date | Review `pricing.yaml` against OpenRouter pricing |

## Related Docs

- [Provider Model Mapping](provider-model-mapping.md) — why three layers of model names exist
- [Runbook](runbook.md) — secret rotation, incident response
- [CI Guide](ci-guide.md) — how OpenRouter models are covered in smoke tests
- [Version Matrix](version-matrix.md) — which versions were validated together
