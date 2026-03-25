# Rollback Runbook — StarAPIHub

## When to Roll Back

Roll back when ANY of the following occur after promotion:

- Nightly health check fails (service down)
- Nightly smoke inference fails (request path broken)
- Nightly drift check shows unexpected divergence
- Operator observes production inference failures not explained by upstream provider issues

## Before Rolling Back

1. Identify the **rollback target** — the last known-good version
2. Find its evidence: `release-manifest.json` from that version's release workflow run
3. Confirm the target's images are still in the registry: `docker pull <digest>`

## Upstream Mode Rollback

Upstream mode only manages the dashboard image. New-API, Bifrost, and ClewdR are operator-managed upstream images.

### Steps

```bash
# 1. Identify rollback target version
TARGET_VERSION="0.2.0"  # last known-good

# 2. Pin dashboard image to target version in docker-compose
# Edit control-plane/deploy/docker-compose.yml:
#   image: starapihub/dashboard:${TARGET_VERSION}
# OR pin by digest from release-manifest.json:
#   image: starapihub/dashboard@sha256:<digest>

# 3. Pull and restart
cd control-plane/deploy
docker compose pull dashboard
docker compose up -d dashboard

# 4. Verify dashboard is healthy
curl -sf http://localhost:8090/api/health && echo "Dashboard OK"

# 5. Verify CLI matches (optional — download from GitHub Release)
gh release download "v${TARGET_VERSION}" --pattern 'starapihub-*' --dir /tmp
# Verify sha256 matches release-manifest.json
shasum -a 256 /tmp/starapihub-*
```

### What Upstream Mode Does NOT Roll Back

- New-API — operator manages upstream images independently
- Bifrost — operator manages upstream images independently
- ClewdR — operator manages upstream images independently
- nginx config — managed by control-plane sync, not by image version

## Appliance Mode Rollback

Appliance mode manages both dashboard and patched New-API images.

### Steps

```bash
# 1. Identify rollback target version
TARGET_VERSION="0.2.0"  # last known-good

# 2. Pin both images in docker-compose
# Edit control-plane/deploy/docker-compose.yml:
#   dashboard:
#     image: starapihub/dashboard:${TARGET_VERSION}
#   new-api:
#     image: starapihub/new-api:patched-${TARGET_VERSION}
# OR pin by digest from release-manifest.json

# 3. Pull and restart both services
cd control-plane/deploy
docker compose pull dashboard new-api
docker compose up -d dashboard new-api

# 4. Verify services are healthy
curl -sf http://localhost:8090/api/health && echo "Dashboard OK"
curl -sf http://localhost:3000/api/status && echo "New-API OK"

# 5. Verify patched behavior (Patch 001 — X-Request-ID propagation)
RID=$(uuidgen)
curl -s -H "X-Request-ID: $RID" -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"cheap-chat","messages":[{"role":"user","content":"ping"}],"max_tokens":5}' \
  http://localhost:3000/v1/chat/completions -D - 2>/dev/null | grep -i x-request-id
# Should echo back the same $RID

# 6. Run smoke tests
cd control-plane
bash scripts/smoke/run-all.sh
```

## Post-Rollback Verification

After ANY rollback (either mode), verify:

| Check | Command | Expected |
|-------|---------|----------|
| Dashboard health | `curl -sf http://localhost:8090/api/health` | 200 OK |
| New-API health | `curl -sf http://localhost:3000/api/status` | 200 OK |
| Bifrost health | `curl -sf http://localhost:8080/health` | 200 OK |
| Smoke inference | `bash scripts/smoke/run-all.sh` | All checks pass |
| Drift check | `./dashboard/starapihub diff` | No unexpected drift |
| Config sync | `./dashboard/starapihub sync --dry-run` | No pending changes |

## Post-Rollback Actions

1. **Update `release-status.md`** — mark the bad version as failed, record rollback target
2. **Investigate** — determine root cause before attempting a new release
3. **Do not re-promote the failed version** — fix forward with a new release

## Git Tag Rollback

If the git tag needs to match the deployed state:

```bash
# Check out the rollback target
git checkout "v${TARGET_VERSION}"

# Verify the control-plane code matches
cat control-plane/VERSION
```

Do NOT delete or move git tags. Tags are permanent release evidence. If a version was bad, document it in `release-status.md` rather than rewriting history.

## Emergency Contacts

This runbook assumes a single operator. If multiple operators are involved, coordinate via the project's communication channel before executing rollback steps.
