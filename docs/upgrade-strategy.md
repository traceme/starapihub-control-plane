# Upstream Upgrade Strategy

This document describes the **base** upgrade process for the control-plane stack.

If you are operating in the stricter commercial-appliance mode, also read:

- [upgrade-strategy-commercial-appliance.md](/Users/mac/projects/OpenRouterAround/starapihub/docs/upgrade-strategy-commercial-appliance.md)
- [upstream-patches.md](/Users/mac/projects/OpenRouterAround/starapihub/docs/upstream-patches.md)
- [version-matrix.md](/Users/mac/projects/OpenRouterAround/starapihub/control-plane/docs/version-matrix.md)
- [patch-audit-workflow.md](/Users/mac/projects/OpenRouterAround/starapihub/control-plane/docs/patch-audit-workflow.md)

## Core Principle

In the pure external-integration mode, upgrades are straightforward:

1. Change the image tag
2. Check for config schema changes
3. Recreate the container
4. Verify

No patch rebasing, no merge conflicts, no source diffing.

In the commercial-appliance mode, this remains the default path. If a minimal upstream patch set exists, follow the additional patch-aware upgrade process in the root-level commercial appliance docs.

## Pre-Upgrade Checklist

- [ ] Read upstream release notes for breaking changes
- [ ] Check if config file format changed (config.json schema, env vars, etc.)
- [ ] Backup current database volumes
- [ ] Plan a maintenance window if needed
- [ ] Ensure rollback plan is ready (keep old image tag noted)

## Upgrade Procedure

### Upgrading New-API

```bash
# 1. Note current version
docker inspect cp-new-api --format '{{.Config.Image}}'

# 2. Backup PostgreSQL
docker exec cp-postgres pg_dump -U newapi newapi > backup-$(date +%Y%m%d).sql

# 3. Update version in common.env
# NEWAPI_VERSION=v1.2.3

# 4. Pull and recreate
cd control-plane/deploy
docker-compose --env-file env/common.env pull new-api
docker-compose --env-file env/common.env up -d new-api

# 5. Verify
docker-compose ps new-api
docker logs cp-new-api --tail 20
bash ../scripts/smoke/check-newapi.sh
```

**Watch for**: Database migration changes (New-API runs auto-migrations via GORM). Check logs for migration errors.

### Upgrading Bifrost

```bash
# 1. Note current version
docker inspect cp-bifrost --format '{{.Config.Image}}'

# 2. Backup Bifrost data directory (contains config.db)
docker cp cp-bifrost:/app/data ./bifrost-backup-$(date +%Y%m%d)

# 3. Update version
# BIFROST_VERSION=v2.3.4

# 4. Pull and recreate
docker-compose --env-file env/common.env pull bifrost
docker-compose --env-file env/common.env up -d bifrost

# 5. Verify
docker-compose ps bifrost
docker logs cp-bifrost --tail 20
bash ../scripts/smoke/check-bifrost.sh
```

**Watch for**: config.json schema changes (new fields, renamed fields). Check Bifrost release notes for provider config changes.

### Upgrading ClewdR

```bash
# Upgrade one instance at a time — keep others running for availability

# 1. Upgrade instance 1
docker-compose --env-file env/common.env pull clewdr-1
docker-compose --env-file env/common.env up -d clewdr-1

# 2. Verify instance 1 is healthy
docker logs cp-clewdr-1 --tail 10
# Check cookies still work via admin UI

# 3. Repeat for instances 2 and 3
```

**Watch for**: Cookie format changes, admin UI changes, config file format changes.

## Rollback

If an upgrade fails:

```bash
# 1. Revert the version in common.env to the old version
# NEWAPI_VERSION=v1.2.2  (previous version)

# 2. Pull the old image and recreate
docker-compose --env-file env/common.env pull <service>
docker-compose --env-file env/common.env up -d <service>

# 3. For database rollback (if migration broke things):
# Restore from backup
docker exec -i cp-postgres psql -U newapi newapi < backup-YYYYMMDD.sql
# Then restart New-API
```

## Version Pinning

**Always pin versions in production.** Never use `latest` in `common.env`:

```bash
# Good
NEWAPI_VERSION=v1.2.3
BIFROST_VERSION=v2.3.4
CLEWDR_VERSION=v0.12.23

# Bad (for production)
NEWAPI_VERSION=latest
BIFROST_VERSION=latest
CLEWDR_VERSION=latest
```

## Change Impact Checklist

When upgrading any component, check:

| Change Type | Impact | Action |
|------------|--------|--------|
| New env vars added | Low | Add to env template, set defaults |
| Env vars renamed/removed | Medium | Update env files before upgrade |
| Config file schema change | Medium | Update config templates, regenerate config |
| API endpoint changes | High | Update sync scripts, channel base URLs |
| Database schema migration | High | Backup first, verify post-upgrade |
| Default behavior change | Medium | Read release notes, adjust config if needed |
| Port changes | High | Update compose, nginx config |
| New required dependencies | Medium | Update compose (add services) |

## Commercial Appliance Addendum

If the appliance carries approved upstream patches:

1. update the upstream version pin
2. verify whether each patch is still needed
3. reapply or remove only the documented minimal patch set
4. run the fixed verification suite
5. update the patch inventory and version matrix

If an upgrade would require broad new divergence, reject or defer that upgrade.
