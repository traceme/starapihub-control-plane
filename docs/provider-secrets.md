# Provider Secrets and Recovery

## Purpose

This document is the single operator-facing reference for where provider credentials live, how they are consumed, and how to recover provider-related state after failures.

## Credential Map

All secrets are stored in env files under `control-plane/deploy/env/`. Secrets are NEVER stored in policy YAML files — those contain only env var names via `value_env` references.

### Provider API Keys

| Env Var | File | Service | Consumed By | Rotation Method |
|---------|------|---------|-------------|-----------------|
| `ANTHROPIC_API_KEY` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |
| `OPENAI_API_KEY` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |
| `OPENROUTER_API_KEY` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |
| `CLEWDR_1_PASSWORD` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |
| `CLEWDR_2_PASSWORD` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |
| `CLEWDR_3_PASSWORD` | `bifrost.env` | Bifrost | Sync engine → Bifrost HTTP API | `starapihub sync --target provider` |

### Infrastructure Credentials

| Env Var | File | Service | Consumed By | Rotation Method |
|---------|------|---------|-------------|-----------------|
| `POSTGRES_PASSWORD` | `common.env` | PostgreSQL, New-API | Docker Compose + container env | DB ALTER USER + restart New-API |
| `REDIS_PASSWORD` | `common.env` | Redis, New-API | Docker Compose + container env | Update env + restart Redis + New-API |
| `SESSION_SECRET` | `new-api.env` | New-API | Container env at startup | Update env + restart New-API |
| `DASHBOARD_TOKEN` | `dashboard.env` | Dashboard | Container env at startup | Update env + restart dashboard |
| `BIFROST_API_KEY` | `new-api.env` | New-API | New-API channels (auth to Bifrost) | Update env + restart New-API |

### ClewdR Instance Credentials

| Env Var | File | Service | Consumed By | Rotation Method |
|---------|------|---------|-------------|-----------------|
| `CLEWDR_ADMIN_TOKENS` | `dashboard.env` | Dashboard + CLI | Dashboard poller and CLI (`cookie-status`, `health`) — CSV, one per instance. CLI also accepts singular `CLEWDR_ADMIN_TOKEN` as fallback. | Update env + restart dashboard |

### Alert Delivery Credentials

| Env Var | File | Service | Consumed By | Rotation Method |
|---------|------|---------|-------------|-----------------|
| `ALERT_WEBHOOK_URL` | `dashboard.env` | Dashboard | `poller/alerts.go` → `sendWebhook()` | Update env + restart dashboard |
| `WEBHOOK_URL` | Operator shell / cron env | Cron | `scripts/starapihub-cron.sh` → `send_alert()` | Update cron env or wrapper script |

## How Provider Keys Are Consumed

Provider API keys follow a specific flow that is different from infrastructure credentials:

1. **Operator** sets `PROVIDER_API_KEY=value` in `deploy/env/bifrost.env`
2. **Sync CLI** reads the env var via `os.Getenv()` at runtime
3. **Sync CLI** pushes the resolved value to Bifrost via `PUT /api/providers/{id}`
4. **Bifrost** stores the key in its internal state and uses it for outbound requests

**Key points:**
- Bifrost does NOT read provider keys from its own environment variables at startup
- The sync engine is the bridge between env files and Bifrost's live state
- After a key rotation, `starapihub sync --target provider` is required
- No Bifrost restart is needed for key changes

## Rotation Procedures

### All Provider Keys (Anthropic, OpenAI, OpenRouter)

```bash
# 1. Update the env var in bifrost.env
#    e.g., ANTHROPIC_API_KEY=sk-ant-new-key

# 2. Export the new value for the sync CLI
export ANTHROPIC_API_KEY=sk-ant-new-key

# 3. Sync to Bifrost
cd control-plane
./dashboard/starapihub sync --target provider

# 4. Smoke test
curl -s -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
  http://localhost:3000/v1/chat/completions

# 5. Revoke the old key in the provider dashboard
```

### Dashboard Token

```bash
# 1. Generate new token
NEW_TOKEN=$(openssl rand -hex 32)

# 2. Update dashboard.env
# DASHBOARD_TOKEN=$NEW_TOKEN

# 3. Restart dashboard
cd control-plane/deploy
docker compose restart dashboard

# 4. Update CI secrets if applicable (DASHBOARD_TOKEN in GitHub Actions)

# 5. Verify
curl -sf -H "Authorization: Bearer $NEW_TOKEN" \
  http://localhost:8090/api/health && echo "OK"
```

### Database Password

```bash
# 1. Change password in PostgreSQL
docker exec -it cp-postgres psql -U newapi -c \
  "ALTER USER newapi WITH PASSWORD 'new-password';"

# 2. Update common.env and new-api.env with new password

# 3. Restart New-API
cd control-plane/deploy
docker compose restart new-api

# 4. Verify
curl -sf http://localhost:3000/api/status && echo "New-API OK"
```

## Backup: What Provider State Exists

### Bifrost Provider State

Bifrost stores provider configuration (including resolved API keys) in its internal state. This state is populated by the sync engine.

| State | Location | Backup Method |
|-------|----------|---------------|
| Provider config (keys, models, network) | Bifrost internal (in-memory or config.db) | Re-sync from `policies/providers.yaml` + env vars |
| Routing rules | Bifrost internal | Re-sync from `policies/routing-rules.yaml` |

**Bifrost provider state is fully reconstructable** from the policy YAML files + env vars via `starapihub sync`. There is no Bifrost-specific backup required for provider config.

### New-API Channel State

New-API stores channel configuration in its PostgreSQL database, pushed there by the sync engine.

| State | Location | Backup Method |
|-------|----------|---------------|
| Channels (model mappings, URLs) | PostgreSQL `channels` table | `pg_dump` or re-sync from `policies/channels.yaml` |
| Models (logical model definitions) | PostgreSQL | `pg_dump` or re-sync from `policies/models.yaml` |
| Pricing (token ratios) | PostgreSQL `abilities` table | `pg_dump` or re-sync from `policies/pricing.yaml` |
| User tokens, billing, quotas | PostgreSQL | `pg_dump` (NOT reconstructable from sync) |

**New-API provider-related state is reconstructable** via sync. User/billing state is NOT — it requires database backup.

### Env Files

| File | Contains | Backup Method |
|------|----------|---------------|
| `deploy/env/bifrost.env` | Provider API keys, ClewdR passwords | Copy to secure backup location |
| `deploy/env/common.env` | DB password, Redis password | Copy to secure backup location |
| `deploy/env/new-api.env` | Session secret, Bifrost API key | Copy to secure backup location |
| `deploy/env/dashboard.env` | Dashboard token, ClewdR admin tokens | Copy to secure backup location |
| `deploy/env/clewdr-*.env` | ClewdR instance config | Copy to secure backup location |

**Env files are the only non-reconstructable secret storage.** If these are lost, all credentials must be regenerated.

## Recovery Procedures

### Scenario: Bifrost Lost All Provider Config

Cause: Bifrost container replaced, data volume lost, or config.db corrupted.

```bash
# Re-sync all providers from registry + env vars
cd control-plane
# Export all provider API keys (set -a auto-exports plain KEY=value)
set -a; source deploy/env/bifrost.env; set +a
./dashboard/starapihub sync

# Verify
./dashboard/starapihub sync --dry-run   # 0 pending
./dashboard/starapihub diff             # no blocking drift
bash scripts/smoke/run-all.sh           # all pass
```

### Scenario: New-API Lost Channel/Model Config

Cause: Database reset, migration failure, or manual deletion.

```bash
# Re-sync channels, models, and pricing
cd control-plane
./dashboard/starapihub sync --target channels
./dashboard/starapihub sync --target models
./dashboard/starapihub sync --target pricing

# Verify
./dashboard/starapihub sync --dry-run
```

### Scenario: Env Files Lost

Cause: Host failure, accidental deletion.

1. Regenerate all credentials:
   - New provider API keys from each provider dashboard
   - New database password (requires DB ALTER USER)
   - New dashboard token (`openssl rand -hex 32`)
   - ClewdR admin passwords from ClewdR instance logs
2. Populate new env files from `.env.example` templates
3. Restart all services
4. Re-sync all config: `starapihub sync`

### Scenario: Full Stack Recovery from Scratch

```bash
# 1. Restore env files from backup (or regenerate)
cp backup/env/* control-plane/deploy/env/

# 2. Start the stack
cd control-plane/deploy
docker compose up -d

# 3. Wait for services to be healthy
sleep 30

# 4. Restore database from backup (if available)
docker exec -i cp-postgres psql -U newapi newapi < backup.sql

# 5. Re-sync all provider config
cd control-plane
set -a; source deploy/env/bifrost.env; set +a
./dashboard/starapihub sync

# 6. Verify
./dashboard/starapihub health
./dashboard/starapihub diff
bash scripts/smoke/run-all.sh
```

## Security Notes

- Env files (`deploy/env/*.env`) should NOT be committed to version control
- The `.env.example` files ARE committed — they contain only placeholder values
- `policies/providers.yaml` contains env var NAMES, not secrets — it IS safe to commit
- Bifrost's internal state stores resolved key values — ensure Bifrost's data volume has appropriate access controls
- Back up env files to a secure, encrypted location separate from the codebase

## Related Docs

- [Runbook](runbook.md) — rotation procedures by component
- [OpenRouter Operations](openrouter-operations.md) — OpenRouter-specific key rotation
- [Provider Onboarding](provider-onboarding.md) — how to add new provider credentials
- [CI Guide](ci-guide.md) — GitHub Actions secrets for CI/release workflows
