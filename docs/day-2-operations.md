# Day-2 Operations

## Purpose

This is the operator's daily entry point. It defines what to check, how often, and what to do when something is wrong. No planning history or chat context required.

For first-time installation, see [install.md](install.md).
For backup and restore, see [backup-restore.md](backup-restore.md).
For secret rotation and provider changes, see [runbook.md](runbook.md).

## Environment Setup

Most CLI commands need env vars. Run this once per shell session:

```bash
cd control-plane
set -a; source deploy/env/dashboard.env; set +a
set -a; source deploy/env/bifrost.env; set +a
# Only needed for diff/sync commands:
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token
```

## Daily Checklist

Run these every day. Total time: ~2 minutes.

### 1. Container health

```bash
cd control-plane/deploy
docker compose --env-file env/common.env ps
```

**Expected:** All services show `Up (healthy)`.
**If not:** See [Service Down](#service-down) below.

### 2. CLI health check

```bash
cd control-plane
./dashboard/starapihub health
```

**Requires:** `NEWAPI_URL`, `BIFROST_URL` (from `dashboard.env`).
**Expected:** Exit code 0, all services green.
**If not:** Check which service is unreachable — likely a container is down or restarting.

### 3. ClewdR cookie status (if using ClewdR)

```bash
./dashboard/starapihub cookie-status
```

**Expected:** At least 2 valid cookies per instance.
**If low:** See [Cookie Exhaustion](runbook.md#cookie-exhaustion) in the runbook.

### 4. Check nightly result (if nightly workflow is enabled)

```bash
gh run list --workflow=nightly.yml --limit=1
```

**Expected:** Latest run shows `completed` / `success`.
**If failed:** Download nightly artifacts and check which step broke:

```bash
# Get the failed run ID
RUN_ID=$(gh run list --workflow=nightly.yml --limit=1 --json databaseId -q '.[0].databaseId')

# Download logs
gh run download "$RUN_ID" --dir /tmp/nightly-evidence

# Check what failed
ls /tmp/nightly-evidence/nightly/
cat /tmp/nightly-evidence/nightly/*.log
```

See [ci-guide.md](ci-guide.md) → Nightly Failures for the full failure table.

### 5. Review error logs (quick scan)

```bash
docker logs cp-new-api --since 24h 2>&1 | grep -i error | tail -20
docker logs cp-bifrost --since 24h 2>&1 | grep -i error | tail -20
```

**Expected:** No unexpected errors. Occasional provider timeouts are normal.
**If errors are persistent:** See [Failure Triage](#failure-triage) below.

## Weekly Checklist

Run these once per week. Total time: ~5 minutes.

### 1. Configuration drift

```bash
cd control-plane
set -a; source deploy/env/dashboard.env; set +a
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token
./dashboard/starapihub diff
```

**Requires:** `NEWAPI_URL`, `BIFROST_URL`, `NEWAPI_ADMIN_TOKEN`.
**Expected:** Exit code 0, no blocking drift.
**If drift found:** See [Drift Detected](runbook.md#drift-detected) in the runbook.

### 2. Sync state

```bash
./dashboard/starapihub sync --dry-run
```

**Requires:** Same as drift check.
**Expected:** 0 pending changes.
**If changes pending:** Run `starapihub sync` to apply, or investigate why state diverged.

### 3. Disk usage

```bash
docker system df
```

**Expected:** No volumes consuming unexpectedly large space.
**If disk is filling:** Prune old images: `docker image prune -f`. For log growth, rotate with `docker logs --since` or truncate log files.

### 4. Backup status

```bash
# Check when the most recent PostgreSQL backup was created
ls -lt control-plane/deploy/backups/*/postgres.dump 2>/dev/null | head -1
```

**Expected:** A backup from within the last 24 hours (or your configured schedule).
**If stale or missing:** Run a backup now. See [backup-restore.md](backup-restore.md) → Full Backup Script.

### 5. Release and nightly status

```bash
# Nightly green streak (last 7 runs)
gh run list --workflow=nightly.yml --limit=7 --json conclusion \
  | jq '[.[] | select(.conclusion == "success")] | length'
```

**Expected:** 7 (all green). Fewer than 3 means low confidence — investigate failures.

Review [release-status.md](release-status.md) for current promoted/validated versions if you are planning an upgrade.

### 6. Provider key expiry review

Check each provider's dashboard for approaching key expiry dates. Most providers don't expire API keys automatically, but OpenRouter and some enterprise accounts do.

If a key is approaching expiry, rotate it now: see [runbook.md](runbook.md) → Secret Rotation.

### 7. Webhook delivery check (if alerting is configured)

Verify your alert receiver is still working:

```bash
# Source the installed cron wrapper and send a test alert
export WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
source /usr/local/bin/starapihub-cron.sh
send_alert INFO test-alert weekly-check "Weekly webhook delivery test"
```

**Expected:** Your receiver (Slack, etc.) shows the test message.
**If delivery fails:** Check the webhook URL hasn't changed and the receiver is up. See [monitoring.md → Test Alert Procedure](monitoring.md#test-alert-procedure).

## Failure Triage

When something breaks, use this section to identify the category and find the right fix. You do not need to read planning docs or chat history.

### Step 1: Identify scope

| Scope | Meaning |
|-------|---------|
| All traffic fails | Core service down (New-API, PostgreSQL, nginx) |
| One model fails | Config issue (channel mapping) or provider issue (key/rate limit) |
| One tier fails (e.g., risky only) | ClewdR cookies expired or ClewdR instances down |
| One user fails | Token issue — check user's API key in New-API admin UI |

### Step 2: Check the signal hierarchy

Work through these in order. Stop at the first failure — that's your starting point.

```bash
# 1. Are containers running?
cd control-plane/deploy
docker compose --env-file env/common.env ps

# 2. Are services healthy?
cd control-plane
set -a; source deploy/env/dashboard.env; set +a
./dashboard/starapihub health

# 3. Is config in sync?
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token
./dashboard/starapihub diff

# 4. Are cookies valid? (if using ClewdR)
./dashboard/starapihub cookie-status
```

### Step 3: Classify and fix

| Category | Signal | First Command | Fix Doc |
|----------|--------|---------------|---------|
| **Service down** | Container not `Up (healthy)` | `docker logs cp-<service> --tail 50` | [Service Down](#service-down) below |
| **Config drift** | `starapihub diff` exits non-zero | `starapihub sync --dry-run` | [Drift Detected](runbook.md#drift-detected) |
| **Provider failure** | 401/403/429/5xx from provider | Curl provider directly (bypass StarAPIHub) | [Provider failure](#provider-failure) below |
| **Cookie exhaustion** | `cookie-status` shows 0 valid | See runbook | [Cookie Exhaustion](runbook.md#cookie-exhaustion) |
| **Product defect** | Sync succeeds but behavior wrong | Check audit log: `tail -1 ~/.starapihub/audit.log \| jq .` | [Product defect](#product-defect) below |

## Failure Responses

### Service Down

A container is not running or not healthy.

```bash
# Check which service is down
docker compose --env-file env/common.env ps

# Read its logs
docker logs cp-<service> --tail 50

# Restart it
docker compose --env-file env/common.env restart <service>

# If it keeps crashing, check for:
# - Auth errors (password mismatch between env files)
# - Port conflicts
# - OOM (check docker stats)
docker stats --no-stream
```

**After restart:** Run `starapihub health` to confirm recovery.

**Service-specific notes:**

| Service | Impact if down | Common cause | Extra check |
|---------|---------------|-------------|-------------|
| `new-api` | All traffic blocked | DB password mismatch, migration failure | `docker exec cp-postgres pg_isready -U newapi` |
| `bifrost` | All inference fails (502) | Config corruption, OOM | Check memory with `docker stats` |
| `postgres` | New-API crashes | Disk full, OOM | `docker exec cp-postgres pg_isready` |
| `redis` | Sessions lost, rate limits reset | OOM | Usually recovers on restart |
| `nginx` | No external access | Config syntax error | `docker exec cp-nginx nginx -t` |
| `clewdr-*` | Risky/standard tier only | Cookie expiry, OOM | `starapihub cookie-status` |

### Provider Failure

A provider's API is rejecting requests. StarAPIHub config is correct.

**Confirm it's the provider, not us:**

```bash
# Test provider directly (example: Anthropic)
curl -s https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":5,"messages":[{"role":"user","content":"ping"}]}'
```

| Provider response | Meaning | Action |
|------------------|---------|--------|
| 401/403 | Key expired or revoked | Rotate key: [runbook.md → Secret Rotation](runbook.md#rotate-official-provider-api-keys) |
| 429 | Rate limited | Wait, or reduce traffic |
| 500/502/503 | Provider outage | Check provider status page, wait |
| Timeout | Provider overloaded | Increase timeout or wait |

### Product Defect

Sync succeeds, diff is clean, but the system behaves incorrectly.

```bash
# Check the last sync result
tail -1 ~/.starapihub/audit.log | jq .

# Check for reconciler failures
tail -1 ~/.starapihub/audit.log | jq '.changes[] | select(.status == "failed")'

# If X-Request-ID is not propagating (appliance mode only):
# Verify the patched New-API image is running
docker inspect cp-new-api --format '{{.Config.Image}}'
# Should show starapihub/new-api:patched, not calciumion/new-api
```

**If you confirm a product defect:**
1. Note the exact symptom, the sync audit log entry, and the container versions
2. Check upstream issue trackers (New-API, Bifrost, ClewdR GitHub)
3. If the issue is in StarAPIHub's own code, file it against this repo

## Escalation Ladder

When to escalate beyond operator self-service:

| Level | Condition | Action |
|-------|-----------|--------|
| **L1: Operator** | Service restart, config sync, key rotation | Follow this doc and the runbook |
| **L2: Investigate** | Issue persists after restart + sync | Check logs, audit trail, provider status pages |
| **L3: Upstream** | Bug in New-API, Bifrost, or ClewdR | File issue on upstream GitHub repo, document workaround |
| **L4: Code change** | StarAPIHub sync/reconciler bug | Requires code fix in `control-plane/dashboard/` |

## Signal Reference

Quick lookup for what each signal tells you and where to find it. Severity levels are defined in [alert-model.md](alert-model.md). The dashboard classifies severity from live state; cron alerts are always WARNING (see [monitoring.md → Crontab Setup](monitoring.md#crontab-setup)).

| Signal | Command | Dashboard severity | Good | Bad | Doc |
|--------|---------|----------|------|-----|-----|
| Container health | `docker compose ps` | CRITICAL (core) / INFO (ClewdR) | All `Up (healthy)` | Any `Exit` or `unhealthy` | [runbook.md](runbook.md) |
| Service health | `starapihub health` | CRITICAL (core) / INFO (ClewdR) | Exit 0 | Exit 1 + error details | [runbook.md](runbook.md) |
| Config drift | `starapihub diff` | WARNING | Exit 0, no blocking | Exit 1, blocking items listed | [runbook.md](runbook.md#drift-detected) |
| Cookie status | `starapihub cookie-status` | CRITICAL (0 valid) / WARNING (low) | ≥2 valid per instance | 0 valid | [runbook.md](runbook.md#cookie-exhaustion) |
| Nightly | `gh run list --workflow=nightly.yml` | WARNING | `success` | `failure` | [ci-guide.md](ci-guide.md) |
| Release status | [release-status.md](release-status.md) | INFO | Promoted version is current | No promoted version | [promotion-criteria.md](promotion-criteria.md) |
| Backup freshness | `ls -lt backups/*/postgres.dump` | INFO | Within 24h | Older than 24h | [backup-restore.md](backup-restore.md) |

## Related Docs

- [Alert Model](alert-model.md) — severity definitions and signal catalog (single source of truth)
- [Monitoring and Alerting](monitoring.md) — crontab setup, webhook delivery, receiver examples
- [Runbook](runbook.md) — startup, shutdown, secret rotation, emergency procedures
- [Backup and Restore](backup-restore.md) — backup scope, restore procedures, drill checklist
- [CI Guide](ci-guide.md) — workflow details, reading CI failures
- [Provider Verification](provider-verification.md) — provider coverage matrix, failure classification
- [Provider Secrets](provider-secrets.md) — credential map and rotation
- [Release Status](release-status.md) — current promoted/validated versions
