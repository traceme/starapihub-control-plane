# Backup and Restore

## Purpose

This document defines exactly what state must be preserved, how to back it up, and how to restore from backup. All procedures include post-restore validation commands.

For secrets and env file details, see [secrets-bootstrap.md](secrets-bootstrap.md).
For the full install path (including two-phase bootstrap), see [install.md](install.md).

## State Classification

Every piece of persistent state falls into one of three categories.

### Must Be Backed Up (non-reconstructable)

These cannot be regenerated from the repo or sync engine. If lost without backup, the data is gone.

| State | Location | Why It's Non-Reconstructable |
|-------|----------|------------------------------|
| User accounts, API tokens, quotas | PostgreSQL (`cp-pg-data` volume) | Created by operators/users via admin UI |
| Billing and usage records | PostgreSQL (`cp-pg-data` volume) | Accumulated over time from inference traffic |
| Env files (all secrets) | `deploy/env/*.env` on host | Operator-generated passwords, provider API keys |
| TLS certificates | `deploy/certs/` on host | CA-signed certs or self-signed — must match domain |
| ClewdR session cookies | ClewdR data volumes (`cp-clewdr-*-data`) | Manually added via admin UI, expire over time |

### Reconstructable by Sync (backup optional but recommended)

These can be regenerated from `policies/*.yaml` + env vars via `starapihub sync`. Backing them up avoids a re-sync step during restore.

| State | Location | Reconstruction Method |
|-------|----------|-----------------------|
| Channels, models, pricing | PostgreSQL (New-API tables) | `starapihub sync` from policy YAML files |
| Provider config (keys, routing) | Bifrost internal state (`cp-bifrost-data`) | `starapihub sync` from policy YAML + env vars |
| Routing rules | Bifrost internal state | `starapihub sync` from `policies/routing-rules.yaml` |
| Dashboard cache | Dashboard volume (`cp-dashboard-data`) | Regenerated automatically on startup |

### Not Required for Recovery

These are ephemeral or can be regenerated with no operator action.

| State | Location | Why Not Needed |
|-------|----------|---------------|
| Redis cache/sessions | `cp-redis-data` volume | Rate limit counters and session cache rebuild automatically |
| New-API application data | `cp-newapi-data` volume | File uploads and temp data — not critical for service recovery |
| Nginx logs | `cp-nginx-logs` volume | Useful for audit but not for recovery |
| New-API logs | `cp-newapi-logs` volume | Useful for debugging but not for recovery |

**Archive logs separately if you need them for audit or compliance.** They are not part of the recovery path.

## Backup Procedures

### 1. PostgreSQL Database (Critical)

This is the single most important backup. It contains all user accounts, API tokens, billing records, and channel config.

```bash
# One-liner backup with timestamp
DUMP="backup-pg-$(date +%Y%m%dT%H%M%S).dump"
docker exec cp-postgres pg_dump -U newapi -Fc newapi > "$DUMP"

# Verify the dump is valid (should print table of contents, not errors)
pg_restore --list "$DUMP" | head -20
```

**Custom format (`-Fc`)** is recommended over plain SQL because it supports selective restore, parallel restore, and compression.

**Schedule:** Daily, or before any upgrade or destructive operation.

**Plain SQL alternative** (if you prefer human-readable dumps):

```bash
docker exec cp-postgres pg_dump -U newapi newapi \
  > "backup-pg-$(date +%Y%m%dT%H%M%S).sql"
```

### 2. Env Files (Critical)

```bash
# Copy all env files to a timestamped backup directory
BACKUP_DIR="backup-env-$(date +%Y%m%dT%H%M%S)"
mkdir -p "$BACKUP_DIR"
cp deploy/env/*.env "$BACKUP_DIR/"

# Verify
ls -la "$BACKUP_DIR/"
```

**Schedule:** After any secret rotation or env file change. These change infrequently.

**Security:** Store env backups in an encrypted location separate from the codebase. They contain all infrastructure and provider secrets.

### 3. TLS Certificates

```bash
# Copy certs
cp -r deploy/certs/ "backup-certs-$(date +%Y%m%dT%H%M%S)/"
```

**Schedule:** After certificate issuance or renewal. For Let's Encrypt, this is every 60-90 days.

### 4. ClewdR Session Data (If Using ClewdR)

ClewdR stores cookie/session state in its data volumes. These are manually added by the operator and expire over time.

```bash
# Back up each ClewdR instance's data volume
for i in 1 2 3; do
  docker run --rm \
    -v cp-clewdr-${i}-data:/data:ro \
    -v "$(pwd)":/backup \
    alpine tar czf "/backup/backup-clewdr-${i}-$(date +%Y%m%dT%H%M%S).tar.gz" -C /data .
done
```

**Schedule:** After adding or rotating cookies. ClewdR cookies expire — a backup of expired cookies has no value.

### 5. Full Backup Script

Run all critical backups in one pass:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd control-plane/deploy

TS=$(date +%Y%m%dT%H%M%S)
DEST="backups/${TS}"
mkdir -p "$DEST"

# 1. PostgreSQL
docker exec cp-postgres pg_dump -U newapi -Fc newapi > "$DEST/postgres.dump"
echo "PostgreSQL: OK"

# 2. Env files
cp env/*.env "$DEST/"
echo "Env files: OK"

# 3. TLS certs
cp -r certs/ "$DEST/certs/" 2>/dev/null && echo "TLS certs: OK" || echo "TLS certs: skipped (no certs dir)"

# 4. ClewdR data (optional)
for i in 1 2 3; do
  docker run --rm \
    -v cp-clewdr-${i}-data:/data:ro \
    -v "$(pwd)/$DEST":/backup \
    alpine tar czf "/backup/clewdr-${i}.tar.gz" -C /data . 2>/dev/null \
    && echo "ClewdR-${i}: OK" || echo "ClewdR-${i}: skipped (not running)"
done

# 5. Record version info for restore context
docker inspect cp-new-api --format '{{.Config.Image}}' > "$DEST/image-newapi.txt" 2>/dev/null || true
docker inspect cp-bifrost --format '{{.Config.Image}}' > "$DEST/image-bifrost.txt" 2>/dev/null || true
cat env/common.env | grep -E '^(NEWAPI_VERSION|BIFROST_VERSION|STARAPIHUB_MODE)=' > "$DEST/versions.txt" 2>/dev/null || true

echo ""
echo "Backup complete: $DEST"
ls -lh "$DEST/"
```

## Restore Procedures

### Prerequisites

Before restoring:

1. The Docker stack is running (or you can start it)
2. You have a backup directory with at least `postgres.dump` and env files
3. You know which mode was in use (check `versions.txt` in the backup, or `STARAPIHUB_MODE` in `common.env`)

### Full Restore from Backup

This procedure restores all state to a clean stack. Use after host failure, accidental data loss, or migration to a new machine.

```bash
cd control-plane

# ── Step 1: Restore env files ──
BACKUP="/path/to/backups/YYYYMMDDTHHMMSS"
cp "$BACKUP"/*.env deploy/env/
echo "Env files restored"

# ── Step 2: Restore TLS certs (if backed up) ──
if [ -d "$BACKUP/certs" ]; then
  cp -r "$BACKUP/certs/" deploy/certs/
  chmod 600 deploy/certs/server.key
  echo "TLS certs restored"
fi

# ── Step 3: Start the stack ──
cd deploy
docker compose --env-file env/common.env up -d
echo "Waiting for services to start..."
sleep 30

# ── Step 4: Restore PostgreSQL ──
# Drop existing data and restore from backup
docker exec -i cp-postgres pg_restore \
  -U newapi -d newapi --clean --if-exists \
  < "$BACKUP/postgres.dump"
echo "PostgreSQL restored"

# ── Step 5: Restart New-API to pick up restored DB state ──
docker compose --env-file env/common.env restart new-api
sleep 10

# ── Step 6: Obtain admin token and re-sync provider config to Bifrost ──
#
# The restored database contains the admin account and any previously created
# tokens. If you backed up your NEWAPI_ADMIN_TOKEN value, export it now.
# If you don't have the old token, create a new one:
#   1. Log into the New-API admin UI at http://localhost:3000
#      (username: root, password: the NEWAPI_ADMIN_PASSWORD you used at install)
#   2. Navigate to Tokens → Add Token
#   3. Copy the generated sk-... token
#
cd ..
set -a; source deploy/env/bifrost.env; set +a
set -a; source deploy/env/dashboard.env; set +a
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token
./dashboard/starapihub sync

# ── Step 7: Restore ClewdR data (if applicable) ──
for i in 1 2 3; do
  if [ -f "$BACKUP/clewdr-${i}.tar.gz" ]; then
    docker run --rm \
      -v cp-clewdr-${i}-data:/data \
      -v "$(cd "$BACKUP" && pwd)":/backup:ro \
      alpine sh -c "rm -rf /data/* && tar xzf /backup/clewdr-${i}.tar.gz -C /data"
    echo "ClewdR-${i} data restored"
  fi
done
# Restart ClewdR instances to pick up restored data
docker compose --env-file env/common.env restart clewdr-1 clewdr-2 clewdr-3 2>/dev/null || true
```

### Database-Only Restore

Use when only PostgreSQL data was lost (e.g., volume deletion) but env files and config are intact.

```bash
cd control-plane/deploy

# Restore from custom-format dump
docker exec -i cp-postgres pg_restore \
  -U newapi -d newapi --clean --if-exists \
  < /path/to/postgres.dump

# Or from plain SQL dump
docker exec -i cp-postgres psql -U newapi newapi \
  < /path/to/backup.sql

# Restart New-API to pick up restored state
docker compose --env-file env/common.env restart new-api

# Verify (health requires NEWAPI_URL and BIFROST_URL from dashboard.env)
cd ..
set -a; source deploy/env/dashboard.env; set +a
./dashboard/starapihub health
```

### Env-File-Only Restore

Use when env files were deleted but the running stack is intact (containers still have the old values in memory).

```bash
cd control-plane/deploy

# Restore from backup
cp /path/to/backup/*.env env/

# No restart needed if containers are still running with the same values.
# If you need to restart (e.g., after host reboot):
docker compose --env-file env/common.env up -d
```

### Sync-Only Recovery (No Database Backup Available)

If PostgreSQL data is lost and no database backup exists, you can reconstruct **provider/channel/model config** from the policy registries. **User accounts, tokens, and billing data are permanently lost.**

```bash
cd control-plane

# Start fresh stack
cd deploy && docker compose --env-file env/common.env up -d && cd ..
sleep 30

# Re-seed admin account (two-phase bootstrap)
set -a; source deploy/env/dashboard.env; set +a
NEWAPI_ADMIN_PASSWORD=your-chosen-password \
  ./dashboard/starapihub bootstrap --skip-sync

# >> Log into New-API admin UI, create admin token
export NEWAPI_ADMIN_TOKEN=sk-new-admin-token

# Sync all config from policy registries
set -a; source deploy/env/bifrost.env; set +a
./dashboard/starapihub bootstrap --skip-seed

# WARNING: All user accounts, API tokens, quotas, and billing records are lost.
# Users must re-register or be re-created manually.
```

## Upstream vs Appliance: What Differs

| Aspect | Upstream | Appliance |
|--------|----------|-----------|
| New-API image | `calciumion/new-api` (public registry) | `starapihub/new-api:patched` (locally built) |
| Image backup | Not needed — pull from registry | **Recommended** — save patched image: `docker save starapihub/new-api:patched > newapi-patched.tar` |
| Restore image | `docker compose pull` | `docker load < newapi-patched.tar` before starting |
| `STARAPIHUB_MODE` | `upstream` in `common.env` | `appliance` in `common.env` |
| Patch 001 | Not active | Active (X-Request-ID propagation) — verify after restore |

**Appliance operators** should also back up the patched New-API image since it's built locally and not available in a public registry:

```bash
# Save patched image to file
docker save starapihub/new-api:patched | gzip > backup-newapi-patched.tar.gz

# Restore on new host
docker load < backup-newapi-patched.tar.gz
```

## Post-Restore Validation

Run these checks after any restore to confirm the system is healthy. Every check includes the expected output.

### Environment setup for validation

Before running validation, export the env vars that the CLI and curl commands need. If you already have these exported from the restore steps, skip this block.

```bash
cd control-plane

# dashboard.env provides DASHBOARD_TOKEN, NEWAPI_URL, BIFROST_URL
set -a; source deploy/env/dashboard.env; set +a

# bifrost.env provides provider API keys (needed only if you re-sync)
set -a; source deploy/env/bifrost.env; set +a

# NEWAPI_ADMIN_TOKEN: required by diff and sync commands.
# Use the token you exported during restore Step 6.
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token

# API_KEY: a client inference token from New-API (created in admin UI → Tokens).
# If the restored DB has your old tokens, use the same value.
# If not, log into http://localhost:3000, create a new token, and export it.
export API_KEY=sk-your-client-token
```

### Checks

```bash
# 1. All services running and healthy
# Requires: nothing (docker command only)
docker compose -f deploy/docker-compose.yml --env-file deploy/env/common.env ps
# Expected: all services show "Up (healthy)"

# 2. Dashboard health
# Requires: DASHBOARD_TOKEN (from dashboard.env)
curl -sf -H "Authorization: Bearer $DASHBOARD_TOKEN" \
  http://localhost:8090/api/health && echo " Dashboard OK"
# Expected: JSON health response + "Dashboard OK"

# 3. CLI health check
# Requires: NEWAPI_URL, BIFROST_URL (from dashboard.env). No admin token needed.
./dashboard/starapihub health
# Expected: exit code 0, all services green

# 4. No configuration drift
# Requires: NEWAPI_URL, BIFROST_URL (from dashboard.env), NEWAPI_ADMIN_TOKEN
./dashboard/starapihub diff
# Expected: exit code 0, no blocking drift

# 5. Sync is clean
# Requires: NEWAPI_URL, BIFROST_URL (from dashboard.env), NEWAPI_ADMIN_TOKEN
./dashboard/starapihub sync --dry-run
# Expected: 0 pending changes

# 6. Models are visible
# Requires: API_KEY (client inference token from New-API admin UI)
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/v1/models | jq '.data | length'
# Expected: number > 0

# 7. Inference works (optional — consumes provider quota)
# Requires: API_KEY
curl -s -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
  http://localhost:3000/v1/chat/completions | jq '.choices[0].message.content'
# Expected: a short response (not an error)
```

**If any check fails:**

| Check | Failure | Fix |
|-------|---------|-----|
| Services not healthy | Container crashing | Check `docker logs cp-<service>` for auth or config errors |
| Dashboard 401 | `DASHBOARD_TOKEN` mismatch | Verify `deploy/env/dashboard.env` matches what the container loaded |
| `starapihub health` fails | Service unreachable | Check container networking, env URLs |
| Drift detected | Sync state stale | Run `starapihub sync` to push policy state |
| Models not visible | DB restore incomplete or admin token invalid | Re-run `pg_restore`, verify API_KEY |
| Inference fails | Provider key not synced | Run `starapihub sync --target provider` |

## Restore Drill Checklist

Use this checklist to practice a full restore. Run it periodically (recommended: quarterly) to verify that your backup and restore procedures work.

### Preparation

- [ ] Choose a test environment (NOT production)
- [ ] Verify you have a recent backup: `ls -la backups/` or your backup storage location
- [ ] Export required env vars (see "Environment setup for validation" above)
- [ ] Record current state for comparison:
  ```bash
  ./dashboard/starapihub health > /tmp/pre-drill-health.txt
  curl -s -H "Authorization: Bearer $API_KEY" \
    http://localhost:3000/v1/models | jq '.data[].id' > /tmp/pre-drill-models.txt
  ```

### Execute

1. [ ] **Simulate data loss**: Stop stack and delete PostgreSQL volume
   ```bash
   cd control-plane/deploy
   docker compose --env-file env/common.env down
   docker volume rm cp-pg-data
   ```

2. [ ] **Restore from backup**: Follow "Full Restore from Backup" procedure above

3. [ ] **Run post-restore validation**: All 7 checks must pass

### Verify

4. [ ] **Compare state**: Models and health should match pre-drill state
   ```bash
   ./dashboard/starapihub health > /tmp/post-drill-health.txt
   curl -s -H "Authorization: Bearer $API_KEY" \
     http://localhost:3000/v1/models | jq '.data[].id' > /tmp/post-drill-models.txt
   diff /tmp/pre-drill-models.txt /tmp/post-drill-models.txt
   # Expected: no differences (or expected differences if model config changed)
   ```

5. [ ] **Record results**: Note date, duration, any issues encountered
   ```
   Date: YYYY-MM-DD
   Duration: ____ minutes
   Backup used: backups/YYYYMMDDTHHMMSS/
   Result: PASS / FAIL
   Issues: (none / describe)
   ```

### Drill Scope Options

| Drill Type | What's Destroyed | What to Verify |
|-----------|-----------------|----------------|
| **Database only** | Delete `cp-pg-data` volume | Users, tokens, channels restored |
| **Env files only** | Delete `deploy/env/*.env` | Services restart with restored secrets |
| **Full stack** | Delete all volumes + env files | Complete system recovery |
| **ClewdR only** | Delete `cp-clewdr-*-data` volumes | Cookie state restored (if not expired) |

## Backup Retention

Suggested retention policy (adjust to your needs):

| Backup Type | Frequency | Retain |
|-------------|-----------|--------|
| PostgreSQL dump | Daily | 7 daily + 4 weekly + 3 monthly |
| Env files | On change | Last 3 versions |
| TLS certificates | On renewal | Current + previous |
| ClewdR data | On cookie change | Last version only (cookies expire) |
| Patched images (appliance) | On build | Current + previous |

## Related Docs

- [Install Guide](install.md) — full first-install path including two-phase bootstrap
- [Secrets Bootstrap](secrets-bootstrap.md) — complete secret prerequisites
- [Provider Secrets](provider-secrets.md) — credential map and recovery
- [Runbook](runbook.md) — operational procedures including secret rotation
- [Upgrade Strategy](upgrade-strategy.md) — pre-upgrade backup requirements
