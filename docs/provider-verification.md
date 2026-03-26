# Provider Verification Matrix

## Purpose

This document maps each supported provider to the verification paths that cover it, and classifies failure types so operators can diagnose issues without guessing.

## Provider Coverage Matrix

| Provider | Type | Models | Smoke Coverage | Nightly Coverage | Browser Coverage |
|----------|------|--------|----------------|------------------|------------------|
| Anthropic | Direct | claude-sonnet, claude-opus, claude-haiku | `check-logical-models.sh` (premium tier) | Nightly smoke + browser regression | CI-05 (smoke inference) |
| OpenAI | Direct | gpt-4o, gpt-4o-mini | `check-logical-models.sh` (premium tier) | Nightly smoke | — |
| OpenRouter | Aggregator | openai/gpt-5.4, anthropic/claude-opus-4.6 | `check-logical-models.sh` (openrouter tier) | Nightly smoke | — |
| ClewdR | Custom | lab-claude, lab-claude-opus (via cheap-chat, fast-chat fallback) | `check-logical-models.sh` (risky tier), `check-fallback.sh` | Nightly smoke | — |

### What Each Verification Path Proves

| Path | Script / Workflow | What It Proves |
|------|------------------|----------------|
| Smoke: logical models | `scripts/smoke/check-logical-models.sh` | Each model name resolves through New-API → Bifrost → provider |
| Smoke: fallback | `scripts/smoke/check-fallback.sh` | ClewdR fallback chain works when primary provider fails |
| Smoke: correlation | `scripts/smoke/check-correlation.sh` | X-Request-ID propagates end-to-end |
| Smoke: isolation | `scripts/smoke/check-clewdr-isolation.sh` | ClewdR is not reachable from outside Docker |
| Nightly | `.github/workflows/nightly.yml` | Health, drift, cookies, smoke inference, browser regression on schedule |
| Browser regression | `tests/browser/*.spec.ts` | Dashboard renders real data, admin UI accessible |
| Release validation | `make rc-validate` | Full 17/20 gate pipeline with evidence capture |

### OpenRouter Verification Detail

OpenRouter models are verified by the `check-logical-models.sh` smoke script:

- `openai/gpt-5.4` — sends a chat completion request via New-API
- `anthropic/claude-opus-4.6` — sends a chat completion request via New-API

The request exercises the full three-layer mapping:
1. New-API channel `bifrost-openrouter` maps the public model ID to the prefixed internal ID
2. Bifrost parses the prefix, selects provider `openrouter` and key `openrouter-primary`
3. OpenRouter routes to the upstream vendor

A 200 response confirms the entire chain works. A 404 indicates model mapping failure. A 401/403 indicates key issues. See the failure classification below.

## Failure Classification

When a smoke test or nightly check fails, classify the failure into one of four categories:

### Category 1: Config Failure

**Meaning:** The StarAPIHub configuration is wrong. The system is not sending the right request to the right place.

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| 404 from New-API | Model not in any channel's model list | Add model to `channels.yaml`, sync |
| 400 from Bifrost | Channel model mapping produces invalid model name | Fix `channels.yaml` model_mapping, sync |
| Routing to wrong provider | Missing or incorrect CEL routing rule | Fix `routing-rules.yaml`, sync |
| Bifrost uses wrong key | `allow_direct_keys: true` or wrong key in provider config | Fix `providers.yaml`, sync |

**Diagnosis:** `starapihub diff` shows drift, or `starapihub sync --dry-run` shows pending changes.

### Category 2: Provider Failure

**Meaning:** The provider's API is down, rate-limited, or rejecting requests. Our configuration is correct.

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| 401/403 from provider | API key expired or revoked | Rotate key (see `provider-secrets.md`) |
| 429 from provider | Rate limit exceeded | Wait, or reduce request volume |
| 500/502/503 from provider | Provider outage | Wait, check provider status page |
| Timeout | Provider is slow or overloaded | Increase timeout in `providers.yaml`, or wait |

**Diagnosis:** Direct curl to the provider endpoint (bypassing StarAPIHub) also fails. Provider status page shows issues.

### Category 3: External Dependency Failure

**Meaning:** Something outside both StarAPIHub and the provider is broken.

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| DNS resolution failure | DNS server down or misconfigured | Check `/etc/resolv.conf`, network config |
| Connection refused to Bifrost/New-API | Docker container crashed or OOM | `docker compose ps`, restart failed service |
| TLS handshake failure | Certificate expired or chain broken | Renew certificate, check nginx config |
| ClewdR cookies all expired | Claude.ai session expired | Rotate cookies (see `clewdr-operations.md`) |

**Diagnosis:** `starapihub health` shows service-level failures. `docker compose ps` shows unhealthy containers.

### Category 4: Product Defect

**Meaning:** StarAPIHub's own code or automation has a bug.

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Sync succeeds but Bifrost state is wrong | Sync engine bug | File issue, check sync reconciler code |
| Model mapping applied but routing still fails | Reconciler didn't push correctly | Check `starapihub sync` audit log |
| Dashboard shows stale data after sync | Dashboard cache or SSE issue | Restart dashboard, check /api/health |
| RC validation passes but smoke fails | Validation gap | Review rc-validate gates vs smoke tests |

**Diagnosis:** `starapihub sync` reports success but `starapihub diff` shows drift that shouldn't exist. Behavior doesn't match what the docs describe.

## Triage Flowchart

```
Smoke test fails
  ↓
Check: can the model be reached directly (bypass New-API)?
  → No:  Is the provider API itself responding?
         → No:  CATEGORY 2 (Provider Failure)
         → Yes: CATEGORY 3 (External — network/DNS/container)
  → Yes: CATEGORY 1 or 4
         ↓
         Check: does starapihub diff show drift?
           → Yes: CATEGORY 1 (Config Failure) — run starapihub sync
           → No:  CATEGORY 4 (Product Defect) — investigate sync/reconciler
```

## Adding Verification for a New Provider

When onboarding a new provider (see `provider-onboarding.md`), add its models to verification:

1. Add model names to `DEFAULT_MODELS` in `scripts/smoke/check-logical-models.sh`
2. Add a row to the Provider Coverage Matrix in this document
3. If the provider has special failure modes, add entries to the failure classification tables
4. Verify the models are covered in at least one nightly path

## Related Docs

- [Verification](verification.md) — full smoke test inventory (10 tests)
- [CI Guide](ci-guide.md) — which workflows run which verification
- [OpenRouter Operations](openrouter-operations.md) — OpenRouter-specific troubleshooting
- [Provider Secrets](provider-secrets.md) — credential rotation for provider failures
