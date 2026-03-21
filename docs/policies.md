# Policy Registry Documentation

## Overview

The control plane defines three external registries that together control how the platform behaves. These registries are the operator's single source of truth — they live outside the upstream systems and represent the intended state of the platform. Changes made here must be synced into New-API and Bifrost (see `config-sync.md`).

| Registry | File | Purpose |
|----------|------|---------|
| Logical Models | `policies/logical-models.example.yaml` | What models clients see and can request |
| Route Policies | `policies/route-policies.example.yaml` | How requests are routed through provider pools |
| Provider Pools | `policies/provider-pools.example.yaml` | What providers are available and how they are grouped |

### Why External Registries?

The no-source-modification constraint means we cannot embed policy logic inside New-API, Bifrost, or ClewdR. Instead, the registries serve as a declarative layer that the operator maintains. Sync scripts and manual steps translate registry entries into the native configuration formats of each upstream system. If the registries and the running systems diverge, the registries are the authority — the running systems should be corrected to match.

## Logical Model Registry

A logical model is a client-facing model name that abstracts away routing complexity. Clients request `claude-sonnet` — they never need to know whether the request goes to the official Anthropic API or a ClewdR instance. The logical model registry is the authoritative list of all models the platform offers.

### Schema

Each entry in `policies/logical-models.example.yaml` has these fields:

| Field | Type | Purpose |
|-------|------|---------|
| `name` | string | Unique identifier clients use in API requests (e.g., `claude-sonnet`) |
| `display_name` | string | Human-readable name for admin UI and documentation |
| `billing_name` | string | Name used in New-API billing records (often same as `name`) |
| `upstream_model` | string | Actual provider model ID (e.g., `claude-sonnet-4-20250514`) |
| `risk_level` | enum | `low` (official only), `medium` (unofficial fallback), `high` (unofficial primary) |
| `newapi_channel` | string | Which New-API channel this model routes through |
| `bifrost_route_policy` | string | Which route policy Bifrost applies for this model |
| `allow_unofficial` | bool | Whether ClewdR/unofficial providers may handle this model |
| `allow_caching` | bool | Whether Bifrost may cache responses for this model |
| `allowed_groups` | list | User groups permitted to use this model (empty = all users) |

### Example Entries

```yaml
models:
  - name: claude-sonnet
    display_name: "Claude Sonnet (Premium)"
    billing_name: claude-sonnet
    upstream_model: claude-sonnet-4-20250514
    risk_level: low
    newapi_channel: bifrost-premium
    bifrost_route_policy: premium
    allow_unofficial: false
    allow_caching: false
    allowed_groups: []

  - name: lab-claude
    display_name: "Claude Sonnet (Lab/Risky)"
    billing_name: lab-claude
    upstream_model: claude-sonnet-4-20250514
    risk_level: high
    newapi_channel: bifrost-risky
    bifrost_route_policy: risky
    allow_unofficial: true
    allow_caching: true
    allowed_groups: [internal, lab-users]

  - name: cheap-chat
    display_name: "Cheap Chat (Standard)"
    billing_name: cheap-chat
    upstream_model: claude-haiku-4-5-20251001
    risk_level: medium
    newapi_channel: bifrost-standard
    bifrost_route_policy: standard
    allow_unofficial: true
    allow_caching: true
    allowed_groups: []
```

### Adding a New Logical Model

1. Add an entry to `policies/logical-models.example.yaml` with all required fields.
2. Verify the `upstream_model` is supported by at least one provider in `policies/provider-pools.example.yaml`.
3. Choose the appropriate `newapi_channel` and `bifrost_route_policy` based on the model's risk level:
   - `low` risk -> `bifrost-premium` channel, `premium` policy
   - `medium` risk -> `bifrost-standard` channel, `standard` policy
   - `high` risk -> `bifrost-risky` channel, `risky` policy
4. Sync to New-API: add the model name to the appropriate channel's model list (via admin API or UI).
5. Sync to Bifrost: ensure the model is reachable through the configured route policy.
6. Run smoke tests 3, 4, or 5 from `docs/verification.md` depending on the tier.

### Removing a Logical Model

1. Remove the entry from the registry.
2. Remove the model from the relevant New-API channel's model list.
3. The model will no longer appear in `/v1/models` responses.
4. Existing billing records for the model are preserved in New-API's database.

## Route Policy Registry

Route policies define the ordered fallback chain of provider pools that Bifrost should attempt for a given request. They encode the platform's trust and cost decisions.

### Schema

Each entry in `policies/route-policies.example.yaml` has:

| Field | Type | Purpose |
|-------|------|---------|
| `name` | string | Policy identifier referenced by logical models |
| `description` | string | Operator-facing explanation |
| `pool_chain` | list | Ordered list of provider pool names (first = preferred) |
| `allow_unofficial` | bool | Whether any pool in the chain may contain unofficial providers |
| `max_retries` | int | Total retries across all pools before returning an error |
| `timeout_seconds` | int | Per-request timeout passed to Bifrost |

### Standard Policies

| Policy | Pool Chain | Unofficial? | Use Case |
|--------|-----------|-------------|----------|
| `premium` | official-anthropic -> official-openai | Never | Paying customers, SLA-sensitive |
| `standard` | official-anthropic -> official-openai -> unofficial-clewdr | Last resort only | General use, cost-optimized |
| `risky` | unofficial-clewdr -> official-anthropic | ClewdR first | Lab, internal, experimental |
| `direct-openai` | official-openai only | Never | OpenAI-specific workloads |

### The Unofficial Provider Rule

ClewdR-backed pools are **never** included in a `premium` route policy. This is enforced by:
1. Convention in this registry (the operator's intent).
2. Bifrost configuration (the pool composition and routing rules).
3. Smoke test validation (Test 4 in `docs/verification.md` confirms premium traffic avoids ClewdR).

To allow ClewdR for a previously premium-only model, the operator must consciously:
1. Change the model's `bifrost_route_policy` from `premium` to `standard` or `risky`.
2. Update the model's `risk_level` accordingly.
3. Potentially change the model's `newapi_channel` to adjust billing.
4. Re-sync config to both New-API and Bifrost.
5. Re-run smoke tests to confirm the change.

## Provider Pool Registry

Provider pools group endpoints that serve the same trust level and role. Bifrost load-balances across providers within a pool and fails over between pools according to the route policy.

### Schema

Each entry in `policies/provider-pools.example.yaml` has:

| Field | Type | Purpose |
|-------|------|---------|
| `name` | string | Pool identifier referenced by route policies |
| `trust_level` | enum | `official` or `unofficial` |
| `providers` | list | Provider entries with connection details |
| `load_balance` | string | Strategy: `round-robin`, `weighted`, `least-latency` |

Each provider within a pool has:

| Field | Type | Purpose |
|-------|------|---------|
| `id` | string | Unique provider identifier |
| `type` | string | Bifrost provider type (e.g., `anthropic`, `openai`) |
| `models` | list | Model IDs this provider supports |
| `weight` | float | Load balancing weight (higher = more traffic) |
| `key_env` | string | Environment variable containing the API key |
| `base_url` | string | Override base URL (used for ClewdR instances) |

### Design Rules

1. **Official pools contain only SLA-backed API endpoints** with real API keys managed by the provider.
2. **Unofficial pools contain only ClewdR instances** with cookie-based access.
3. **Never mix official and unofficial providers in the same pool.** This ensures the route policy fallback chain has clear trust boundaries at each step.
4. **Each ClewdR instance uses a separate cookie/account** to distribute rate limiting and isolate failures.
5. **Pool names are referenced by route policies.** Renaming a pool requires updating all route policies that reference it.

### Example Pool Configuration

```yaml
pools:
  - name: official-anthropic
    trust_level: official
    load_balance: weighted
    providers:
      - id: anthropic-key-1
        type: anthropic
        models: [claude-sonnet-4-20250514, claude-opus-4-20250514]
        weight: 1.0
        key_env: ANTHROPIC_API_KEY

  - name: unofficial-clewdr
    trust_level: unofficial
    load_balance: round-robin
    providers:
      - id: clewdr-1
        type: openai
        models: [claude-sonnet-4-20250514, claude-opus-4-20250514]
        weight: 1.0
        base_url: http://clewdr-1:8484
        key_env: CLEWDR_1_PASSWORD
      - id: clewdr-2
        type: openai
        models: [claude-sonnet-4-20250514, claude-opus-4-20250514]
        weight: 1.0
        base_url: http://clewdr-2:8484
        key_env: CLEWDR_2_PASSWORD
```

## How It All Connects

```
Client requests model "claude-sonnet"
  |
  +-- Logical Model Registry says:
  |     channel = bifrost-premium
  |     policy = premium
  |
  +-- New-API routes to channel "bifrost-premium"
  |     (base_url = http://bifrost:8080)
  |
  +-- Bifrost applies policy "premium"
  |     pool_chain = [official-anthropic, official-openai]
  |
  +-- Bifrost picks from "official-anthropic" pool
  |     providers: [anthropic-key-1]
  |
  +-- Request goes to Anthropic API
        (with real API key from Bifrost config)
```

The registries drive this entire flow without being on the request hot path. They are consulted only at config-sync time, not at request time. The running systems (New-API channels, Bifrost routing rules) are the runtime actors — the registries are the design-time authority.

## Correlation Headers in Policy Context

Route policies map to the following correlation headers (see `docs/observability.md` for propagation details):

| Header | Relationship to Policy |
|--------|----------------------|
| `X-Logical-Model` | The logical model name from this registry |
| `X-Route-Policy` | The route policy name applied by Bifrost |
| `X-End-User` | Used to check `allowed_groups` in the logical model entry |
| `X-Team-ID` | Can be used for team-level policy overrides |
| `X-Request-ID` | Not policy-related, but essential for tracing policy decisions in logs |

## Registry Maintenance

- Review registries monthly to remove deprecated models and add new ones.
- After any registry change, run the relevant sync scripts and smoke tests.
- Keep a changelog (git history) of registry changes for audit purposes.
- The `.example.yaml` files in version control are templates. Production registries (without the `.example` suffix) should be in `deploy/env/` or a secrets manager, never committed with real credentials.
