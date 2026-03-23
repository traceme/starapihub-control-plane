# Operational Runbook

## Normal Startup

```bash
cd control-plane/deploy

# 1. Verify env files exist
ls env/*.env

# 2. Start the full stack
docker-compose --env-file env/common.env up -d

# 3. Wait for health checks (30-60 seconds)
sleep 30

# 4. Verify all services
docker-compose ps
# All should show "Up (healthy)"

# 5. Run smoke tests
cd .. && bash scripts/smoke/run-all.sh
```

## Normal Shutdown

```bash
cd control-plane/deploy

# Graceful stop (preserves all data)
docker-compose --env-file env/common.env down

# Verify all stopped
docker-compose ps
```

## Service Restart

```bash
# Restart a single service
docker-compose --env-file env/common.env restart bifrost

# Restart with fresh container (pulls same image)
docker-compose --env-file env/common.env up -d --force-recreate bifrost
```

## Secret Rotation

### Rotate New-API Database Password

1. Update password in PostgreSQL:
   ```bash
   docker exec -it cp-postgres psql -U newapi -c "ALTER USER newapi WITH PASSWORD 'new-password';"
   ```
2. Update `deploy/env/common.env` and `deploy/env/new-api.env` with new password
3. Restart New-API:
   ```bash
   docker-compose --env-file env/common.env restart new-api
   ```
4. Verify health

### Rotate New-API Session Secret

1. Update `SESSION_SECRET` in `deploy/env/new-api.env`
2. Restart New-API (all active sessions will be invalidated)
3. Users will need to re-authenticate

### Rotate Official Provider API Keys

1. Generate a new API key in the provider's dashboard (OpenAI, Anthropic, etc.)
2. Update the key in Bifrost via Web UI or config.json
3. If using config.json, remount and restart Bifrost
4. Verify with a smoke test through the affected provider

### Rotate ClewdR Cookies

See `docs/clewdr-operations.md` → Credential Rotation section.

## Adding/Removing Providers

### Add a New Official Provider

1. Add provider entry to `policies/provider-pools.example.yaml`
2. Add key to Bifrost (via Web UI or config.json)
3. Add relevant models to Bifrost key's `models` list
4. If new models are exposed: update `policies/logical-models.example.yaml`
5. If new models need New-API channels: create/update channels
6. Smoke test the new provider

### Remove a Provider

1. Remove provider key from Bifrost config
2. Ensure other providers in the same pool can handle the load
3. Remove from `policies/provider-pools.example.yaml`
4. Restart Bifrost if using file-based config

### Add a ClewdR Instance

See `docs/clewdr-operations.md` → Scaling section.

## Monitoring Checklist

Daily:
- [ ] Check `docker-compose ps` — all services healthy
- [ ] Check ClewdR cookie status in each instance's admin UI
- [ ] Review New-API error logs: `docker logs cp-new-api --since 24h | grep -i error`

Weekly:
- [ ] Review Bifrost routing logs for unusual error rates
- [ ] Check disk usage on volumes: `docker system df`
- [ ] Verify backup of PostgreSQL data

## Log Access

```bash
# Nginx
docker logs cp-nginx --tail 100

# New-API
docker logs cp-new-api --tail 100
# Or: ls the mounted log volume

# Bifrost
docker logs cp-bifrost --tail 100

# ClewdR instances
docker logs cp-clewdr-1 --tail 100
docker logs cp-clewdr-2 --tail 100
docker logs cp-clewdr-3 --tail 100

# PostgreSQL
docker logs cp-postgres --tail 100
```

## Emergency Procedures

> For failure drills to practice these scenarios, see `docs/failure-drills.md`.

### Everything is Down

```bash
# Check Docker daemon
systemctl status docker

# Bring up with fresh containers
cd control-plane/deploy
docker-compose --env-file env/common.env up -d

# Check logs for the failing service
docker-compose logs --tail 50 <service-name>
```

### Bifrost is Down but New-API is Up

Clients will get 502 errors. New-API is returning errors because it can't reach Bifrost.

```bash
# Check Bifrost logs
docker logs cp-bifrost --tail 50

# Restart Bifrost
docker-compose --env-file env/common.env restart bifrost

# If config is corrupted, mount a known-good config.json
```

### New-API Down but Bifrost and ClewdR Up

All client-facing traffic is blocked. No inference requests will work. Bifrost and ClewdR remain idle since they only receive traffic from New-API.

```bash
# Check New-API logs for crash cause
docker logs cp-new-api --tail 50

# Check if it's a DB connectivity issue
docker exec cp-postgres pg_isready -U newapi

# Restart New-API
docker-compose --env-file env/common.env restart new-api

# If DB migration failed after upgrade, restore from backup:
# docker exec -i cp-postgres psql -U newapi newapi < backup-YYYYMMDD.sql
# docker-compose --env-file env/common.env restart new-api
```

### All ClewdR Instances Down

Only affects `standard` (fallback) and `risky` (primary) traffic. Premium traffic is unaffected.

```bash
# Check each instance
for i in 1 2 3; do echo "=== ClewdR $i ===" && docker logs cp-clewdr-$i --tail 10; done

# Restart all
docker-compose --env-file env/common.env restart clewdr-1 clewdr-2 clewdr-3

# If cookies are all expired, see ClewdR operations doc for rotation
```

## Sync Failure Recovery

### Symptoms

- `starapihub sync` exits with non-zero code
- Audit log (`~/.starapihub/audit.log`) shows entries with `"failed" > 0`
- Partial state: some resources updated, others not

### Diagnosis

```bash
# Check the last sync audit entry
tail -1 ~/.starapihub/audit.log | jq .

# Look for failed changes
tail -1 ~/.starapihub/audit.log | jq '.changes[] | select(.status == "failed")'

# Check which targets had issues
tail -1 ~/.starapihub/audit.log | jq '{targets: .targets, failed: .failed, succeeded: .succeeded}'
```

### Resolution

```bash
# 1. Check service health first (sync may have failed due to a down service)
starapihub health

# 2. If a service is down, fix it first (see Emergency Procedures above)

# 3. Run diff to see current drift state
starapihub diff

# 4. Re-run sync (idempotent -- safe to retry)
starapihub sync

# 5. If sync fails on a specific target, run with that target only
starapihub sync --target providers
starapihub sync --target channels
starapihub sync --target routing-rule
```

### Verification

```bash
# Verify sync is clean (no changes needed)
starapihub sync --dry-run
# Should show 0 changes

# Verify no blocking drift
starapihub diff
```

### Prevention

- Run `starapihub health` before sync operations
- Monitor audit log for failed entries (see `docs/monitoring.md`)
- Sync is idempotent -- running it twice is always safe

## Cookie Exhaustion

### Symptoms

- `starapihub cookie-status` shows 0 valid cookies
- Risky tier requests returning 502 errors
- Standard tier ClewdR fallback unavailable
- Dashboard alerts: "CRITICAL: clewdr-X has 0 valid cookies"

### Diagnosis

```bash
# Check cookie inventory
starapihub cookie-status

# Check each ClewdR instance directly
for i in 1 2 3; do
  echo "=== ClewdR $i ==="
  curl -s http://clewdr-$i:8484/api/status | jq '.cookies'
done
```

### Resolution

```bash
# 1. Obtain fresh Claude.ai cookies (manual browser step)
#    - Log into claude.ai in a browser
#    - Extract session cookies

# 2. Push new cookies via API
starapihub sync --target cookie

# 3. Or push directly to a specific instance
curl -X POST http://clewdr-1:8484/api/cookies \
  -H "Authorization: Bearer $CLEWDR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cookie": "sessionKey=sk-ant-..."}'

# 4. Verify cookies are valid
starapihub cookie-status
```

### Verification

```bash
# Confirm valid cookie count is above threshold
starapihub cookie-status --min-valid 2
# Should exit 0

# Send a test request through risky tier
curl -s https://localhost/v1/chat/completions \
  -H "Authorization: Bearer $TEST_TOKEN" \
  -d '{"model":"lab-claude","messages":[{"role":"user","content":"test"}],"max_tokens":5}'
```

### Prevention

- Monitor cookie status on cron (see `docs/monitoring.md`)
- Rotate cookies proactively before they expire
- Keep spare cookies ready
- See `docs/clewdr-operations.md` for rotation procedure

## Drift Detected

### Symptoms

- `starapihub diff` exits non-zero
- Monitoring alert: "Blocking drift detected"
- Configuration has diverged from desired state YAML

### Diagnosis

```bash
# See full drift report
starapihub diff

# Get structured report for analysis
starapihub diff --output json | jq '.items[] | select(.severity == "blocking")'

# Common drift causes:
# - Someone changed config via upstream UI
# - An upstream restart reset config to defaults
# - Desired state YAML was updated but sync not run
```

### Resolution

Operator must decide: re-sync (overwrite live with desired) or investigate first.

```bash
# Option A: Re-sync to enforce desired state
starapihub sync --dry-run    # Preview changes first
starapihub sync              # Apply

# Option B: If live state is intentionally different, update desired state
# Edit the relevant YAML file in policies/
vim policies/provider-pools.yaml
starapihub validate          # Verify YAML is valid
starapihub diff              # Confirm drift resolved
```

### Verification

```bash
# Confirm no blocking drift remains
starapihub diff
# Should exit 0 (no blocking drift)
```

### Prevention

- Never make changes via upstream UIs -- always use desired state YAML + sync
- Run drift checks on cron (see `docs/monitoring.md`)
- After any manual intervention, re-run sync to restore desired state

## Upgrade Rollback

### Symptoms

- Service unhealthy after version upgrade
- New errors appearing in logs after image tag change
- Smoke tests failing after upgrade

### Diagnosis

```bash
# Check which version is running
docker inspect cp-new-api --format '{{.Config.Image}}'
docker inspect cp-bifrost --format '{{.Config.Image}}'

# Check logs for errors
docker logs cp-new-api --tail 50
docker logs cp-bifrost --tail 50

# Run health check
starapihub health
```

### Resolution

```bash
# 1. Revert the version in deploy/env/common.env to the previous version
#    e.g., NEWAPI_VERSION=v1.2.2 (was v1.2.3)

# 2. Pull old image and recreate
cd control-plane/deploy
docker-compose --env-file env/common.env pull <service>
docker-compose --env-file env/common.env up -d <service>

# 3. For New-API database rollback (if migration broke things):
docker exec -i cp-postgres psql -U newapi newapi < backup-YYYYMMDD.sql
docker-compose --env-file env/common.env restart new-api

# 4. Re-run sync to ensure config matches
starapihub sync
```

### Verification

```bash
starapihub health          # All services healthy
starapihub diff            # No blocking drift
bash scripts/smoke/run-all.sh  # Smoke tests pass
```

### Prevention

- Always backup before upgrading (see `docs/upgrade-strategy.md`)
- Upgrade in staging first
- Pin versions, never use `latest`
- Follow the full upgrade workflow in `docs/upgrade-strategy.md`
- For commercial appliance mode, also follow `docs/upgrade-strategy-commercial-appliance.md`

## Incident Response

### Triage Checklist

When users report errors, work through this checklist:

1. **Identify the scope**: Is it all traffic, one model, one tier, or one user?
2. **Check container health**: `docker-compose ps` — are all containers up?
3. **Check the request path bottom-up**:
   - Can Bifrost reach providers? (`docker exec cp-bifrost wget -q -O - http://clewdr-1:8484/api/version`)
   - Can New-API reach Bifrost? (`docker exec cp-new-api wget -q -O - http://bifrost:8080/health`)
   - Can Nginx reach New-API? (check Nginx error log for upstream connection errors)
4. **Check error logs**: `docker logs cp-new-api --tail 50`, `docker logs cp-bifrost --tail 50`
5. **Check correlation**: If you have an `X-Request-ID`, trace it through Nginx, New-API, and Bifrost logs.

### Common Incident Patterns

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| All requests return 502 | Bifrost is down | Restart Bifrost |
| All requests return 401 | New-API DB is down (can't verify tokens) | Restart PostgreSQL, then New-API |
| Premium model returns 502, others work | Official provider API key expired | Rotate key in Bifrost |
| Risky model returns 502, premium works | All ClewdR cookies expired | Rotate cookies |
| Requests hang (no response) | Nginx buffering or provider timeout | Check `proxy_buffering`, Bifrost timeouts |
| Billing not recording | New-API internal error post-relay | Check New-API logs for GORM/DB errors |
| Model not in /v1/models list | Channel misconfigured or disabled | Check New-API channel status in admin UI |

### Escalation

If the issue cannot be resolved through container restarts, config changes, or credential rotation:

1. Check upstream issue trackers (New-API GitHub, Bifrost GitHub, ClewdR GitHub) for known bugs.
2. Check if the issue appeared after an upstream version upgrade — roll back if needed (see `docs/upgrade-strategy.md`).
3. If the issue requires a source code fix in an upstream project, file an issue upstream and document the workaround in this runbook.

### Post-Incident

After resolving an incident:

1. Run the full smoke test suite to confirm recovery.
2. Document what happened, root cause, and resolution.
3. If the incident revealed a gap in monitoring, add an alert or a new smoke test.
4. If a new failure pattern was discovered, add it to `docs/failure-drills.md`.
