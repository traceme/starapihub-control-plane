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
