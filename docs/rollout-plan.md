# Phased Rollout Plan

## Overview

This document describes how to roll out the control plane stack from initial development through production. Each phase has clear entry criteria, deliverables, and exit criteria. The operator should not advance to the next phase until the current phase's exit criteria are met.

The guiding principle at every phase is the same: external integration comes first. Integration should happen through configuration, environment variables, APIs, and deployment orchestration unless a commercial-appliance requirement cannot be met that way.

For the stricter commercial-appliance target, a tiny upstream patch set is allowed only after the patch-audit workflow is completed. See [patch-audit-workflow.md](/Users/mac/projects/OpenRouterAround/starapihub/control-plane/docs/patch-audit-workflow.md).

## Phase 0: Local Development

**Goal**: Get all three upstream systems running on a single developer machine, connected to each other, with one official provider.

### Entry Criteria
- Docker and Docker Compose installed
- At least one official provider API key available (e.g., Anthropic or OpenAI)
- The control-plane repo is cloned

### Steps

1. Copy environment templates:
   ```bash
   cd control-plane/deploy/env
   cp common.env.example common.env
   cp new-api.env.example new-api.env
   cp bifrost.env.example bifrost.env
   cp clewdr.env.example clewdr.env
   ```

2. Edit `common.env` — set image versions and the official provider API key.

3. Start the stack:
   ```bash
   cd control-plane/deploy
   docker-compose --env-file env/common.env up -d
   ```

4. Wait for all containers to become healthy (30-60 seconds):
   ```bash
   docker-compose ps
   ```

5. Create an admin account in New-API (first user is auto-admin).

6. Create one Bifrost-facing channel (`bifrost-premium`) in New-API admin UI:
   - Type: OpenAI-compatible
   - Base URL: `http://bifrost:8080`
   - Models: `claude-sonnet`
   - Model mapping: `claude-sonnet` -> `claude-sonnet-4-20250514`

7. Verify Bifrost has the official provider key configured (auto-detected from env, or via config.json mount).

8. Create a test user and API token in New-API.

9. Send a test request:
   ```bash
   curl https://localhost/v1/chat/completions \
     -H "Authorization: Bearer sk-your-test-token" \
     -H "Content-Type: application/json" \
     -d '{"model":"claude-sonnet","messages":[{"role":"user","content":"Hello"}],"max_tokens":10}'
   ```

### Exit Criteria
- [ ] All containers healthy
- [ ] Test request returns a valid completion
- [ ] New-API billing record shows the request
- [ ] Bifrost logs show routing to the official provider

## Phase 1: Multi-Channel with ClewdR (Staging)

**Goal**: Add the three-channel model (premium/standard/risky), bring up ClewdR instances, and verify provider isolation.

### Entry Criteria
- Phase 0 complete
- At least one Claude.ai cookie available for ClewdR
- A staging domain or port forwarding for HTTPS access

### Steps

1. Configure ClewdR instances:
   - Access each ClewdR admin UI via port forwarding
   - Add Claude.ai cookies (one cookie per instance)
   - Verify each instance responds to inference requests

2. Add ClewdR instances to Bifrost:
   - Via Bifrost Web UI or config.json, register each ClewdR as an OpenAI-compatible provider
   - Set appropriate model lists and weights

3. Create routing rules in Bifrost:
   - Premium models route to official providers only
   - Risky models route to ClewdR with official fallback
   - Standard models route to official with ClewdR as last-resort fallback

4. Create all three channels in New-API:
   - `bifrost-premium` — premium model list
   - `bifrost-standard` — standard model list
   - `bifrost-risky` — lab/risky model list (e.g., `lab-claude`)

5. Set model pricing in New-API:
   - Premium models: market rate
   - Standard models: 70% of market rate
   - Risky models: 30% of market rate

6. Update `policies/logical-models.example.yaml` and `policies/route-policies.example.yaml` to reflect the actual configuration.

7. Run the full smoke test suite:
   ```bash
   bash scripts/smoke/run-all.sh
   ```

8. Run failure drills 1, 2, and 4 (provider outage, ClewdR down, ClewdR isolation) from `docs/failure-drills.md`.

### Exit Criteria
- [ ] All 10 smoke tests pass (see `docs/verification.md`)
- [ ] Premium requests never touch ClewdR (verified in Bifrost logs)
- [ ] Risky requests use ClewdR when available
- [ ] ClewdR is unreachable from outside Docker
- [ ] Failure drills 1, 2, and 4 produce expected behavior
- [ ] Billing records reflect correct pricing tiers

## Phase 2: Policy Registry and Config Sync (Staging)

**Goal**: Shift from manual configuration to registry-driven config generation and sync scripts.

### Entry Criteria
- Phase 1 complete
- All policy registry files populated with real values (not example placeholders)

### Steps

1. Populate production-ready policy registries:
   - `policies/logical-models.yaml` (copy from example, fill real values)
   - `policies/route-policies.yaml`
   - `policies/provider-pools.yaml`

2. Validate the registries:
   ```bash
   starapihub validate
   ```

3. Preview sync changes:
   ```bash
   starapihub sync --dry-run
   ```
   Review proposed changes, then apply:
   ```bash
   starapihub sync
   ```

4. Verify that the synced configuration matches what was manually set in Phase 1.

5. Destroy and recreate the stack from scratch using bootstrap:
   ```bash
   docker-compose down -v
   docker-compose --env-file env/common.env up -d
   # Bootstrap: validates prereqs, waits for services, seeds admin, syncs all config, verifies health
   starapihub bootstrap
   # Verify no drift
   starapihub diff
   ```

### Exit Criteria
- [ ] Stack can be fully deployed with `docker-compose up -d` + `starapihub bootstrap`
- [ ] `starapihub diff` shows no blocking drift after bootstrap
- [ ] Full smoke test suite passes after CLI-driven deploy
- [ ] Policy registries are the single source of truth — no undocumented manual config

## Phase 3: Observability and Operations (Pre-Production)

**Goal**: Set up monitoring, alerting, log correlation, and operational workflows before production traffic.

### Entry Criteria
- Phase 2 complete
- Ops zone containers available (Prometheus, Grafana, or equivalent)

### Steps

1. Deploy ops stack (if using the optional ops zone):
   ```bash
   docker-compose --profile ops up -d
   ```

2. Configure Prometheus to scrape:
   - Nginx metrics endpoint
   - Bifrost metrics endpoint (if telemetry plugin enabled)
   - Container health via cAdvisor or Docker metrics

3. Import Grafana dashboards:
   - Request rate and latency by channel tier
   - Provider error rates
   - ClewdR cookie health (manual or scraped from admin API)

4. Verify log correlation:
   - Send a request with a known `X-Request-ID`
   - Trace it through Nginx logs, New-API logs, and Bifrost logs
   - Document any gaps (see `docs/observability.md` for known limitations)

5. Practice the full runbook (`docs/runbook.md`):
   - Normal startup/shutdown
   - Secret rotation for each component
   - Adding and removing a provider

6. Run all failure drills from `docs/failure-drills.md` and record results.

7. Document any deviations from expected behavior and update runbook/drill docs.

### Exit Criteria
- [ ] Monitoring dashboards show real-time metrics
- [ ] Log correlation works for `X-Request-ID` at minimum through Nginx
- [ ] All failure drills completed with documented results
- [ ] Runbook procedures verified by a second operator
- [ ] Alerting rules configured for: service down, high error rate, ClewdR all-unhealthy

## Phase 4: Production Deployment

**Goal**: Deploy the stack for real user traffic with proper security, backups, and operational readiness.

### Entry Criteria
- Phase 3 complete
- Production domain and TLS certificates ready
- Production provider API keys provisioned
- Database backup strategy defined
- On-call rotation established (if applicable)

### Steps

1. Provision production infrastructure:
   - Dedicated host or VM with sufficient resources
   - Production-grade PostgreSQL (consider managed DB if available)
   - Production TLS certificates (Let's Encrypt or organizational CA)

2. Deploy with production environment files:
   - All passwords and secrets are unique, strong, and not reused from staging
   - Image versions are pinned (never `latest`)
   - ClewdR instances use separate Claude.ai accounts

3. Configure production Nginx:
   - Real domain name
   - TLS with certificate auto-renewal
   - Rate limiting at the Nginx level
   - Access logging with `X-Request-ID`

4. Bootstrap the production stack:
   ```bash
   starapihub bootstrap
   ```
   Or if admin is already seeded:
   ```bash
   starapihub sync
   ```

5. Add ClewdR cookies for production instances.

6. Verify and run smoke tests:
   ```bash
   starapihub health
   starapihub diff
   ```

7. Onboard a small group of internal users first (canary):
   - Issue tokens for the pilot group
   - Monitor for 24-48 hours
   - Check billing accuracy, routing correctness, error rates

8. Gradually widen access:
   - Enable more models and user groups

> For detailed canary deployment procedure with per-step rollback, see the "Canary Deployment Procedure" section below.

## Canary Deployment Procedure

For production deployments, use a canary approach: run a parallel Docker Compose environment on different ports, verify it, then cut over. This section covers the full-stack canary process. For single-service upgrades, see the simpler in-place procedure below.

### Prerequisites

- Current production stack running and healthy (`starapihub health` exits 0)
- Desired state YAML validated (`starapihub validate` exits 0)
- No blocking drift (`starapihub diff` exits 0)
- Database backup taken within the last hour

### Step 1: Pre-flight Checks

```bash
# Validate desired state
starapihub validate

# Check current drift
starapihub diff

# Record current image versions
docker inspect cp-new-api --format '{{.Config.Image}}' > /tmp/rollout-previous-versions.txt
docker inspect cp-bifrost --format '{{.Config.Image}}' >> /tmp/rollout-previous-versions.txt
docker inspect cp-clewdr-1 --format '{{.Config.Image}}' >> /tmp/rollout-previous-versions.txt

# Backup PostgreSQL
docker exec cp-postgres pg_dump -U newapi newapi > backup-$(date +%Y%m%d-%H%M).sql
```

**Rollback:** No changes made yet. If pre-flight fails, fix issues before proceeding.

### Step 2: Prepare Canary Environment

```bash
# Create canary env files with alternate ports
cp deploy/env/common.env deploy/env/canary.env

# Edit canary.env to set:
# - NGINX_HTTP_PORT=8080 (instead of 80)
# - NGINX_HTTPS_PORT=8443 (instead of 443)
# - COMPOSE_PROJECT_NAME=starapihub-canary
# - Update image versions to new target versions
# - Keep same database credentials (canary uses same DB or separate test DB)

# For isolated canary (recommended): use a separate database
# POSTGRES_PORT=5433
# DATABASE_URL=postgresql://newapi:password@postgres-canary:5432/newapi
```

**Rollback:** Delete canary env files: `rm deploy/env/canary.env`

### Step 3: Deploy Canary Stack

```bash
cd control-plane/deploy

# Bring up canary stack on alternate ports
COMPOSE_PROJECT_NAME=starapihub-canary \
  docker-compose --env-file env/canary.env up -d

# Wait for all containers to become healthy
COMPOSE_PROJECT_NAME=starapihub-canary \
  docker-compose --env-file env/canary.env ps

# Should show all services as "Up (healthy)" within 60 seconds
```

**Rollback:** Tear down canary: `COMPOSE_PROJECT_NAME=starapihub-canary docker-compose --env-file env/canary.env down -v`

### Step 4: Verify Canary

```bash
# Point CLI at canary endpoints
export NEWAPI_URL=http://localhost:8080
export BIFROST_URL=http://localhost:8080/bifrost
export CLEWDR_URLS=http://localhost:8484

# Run health checks against canary
starapihub health

# Run sync against canary (applies desired state)
starapihub sync

# Verify no drift
starapihub diff

# Run smoke tests against canary URL
SMOKE_BASE_URL=https://localhost:8443 bash scripts/smoke/run-all.sh

# Check cookie status
starapihub cookie-status
```

**Rollback:** Tear down canary: `COMPOSE_PROJECT_NAME=starapihub-canary docker-compose --env-file env/canary.env down -v`

### Step 5: Cut Over

```bash
# Update production env with new image versions
# Edit deploy/env/common.env with the canary-validated versions

# Stop old production stack
cd control-plane/deploy
docker-compose --env-file env/common.env down

# Start new production stack
docker-compose --env-file env/common.env up -d

# Wait for healthy
sleep 30
docker-compose --env-file env/common.env ps

# Point CLI back at production
export NEWAPI_URL=http://localhost  # (production ports)
export BIFROST_URL=http://localhost/bifrost

# Run health and sync
starapihub health
starapihub sync
```

**Rollback:** Revert image versions in common.env to values from `/tmp/rollout-previous-versions.txt`, then `docker-compose --env-file env/common.env up -d`. If DB migration occurred, restore from backup: `docker exec -i cp-postgres psql -U newapi newapi < backup-YYYYMMDD-HHMM.sql`

### Step 6: Cleanup and Soak

```bash
# Tear down canary stack
COMPOSE_PROJECT_NAME=starapihub-canary \
  docker-compose --env-file env/canary.env down -v

# Remove canary env files
rm deploy/env/canary.env

# Monitor for 24 hours (soak period):
# - Watch health: starapihub health
# - Watch drift: starapihub diff
# - Watch cookies: starapihub cookie-status
# - Check audit log: tail -5 ~/.starapihub/audit.log | jq .
# - Review error logs: docker logs cp-new-api --since 1h | grep -i error
```

**Rollback during soak:** If issues appear within the soak period, revert using Step 5 rollback procedure.

### Single-Service In-Place Upgrade

For upgrading a single service (e.g., just Bifrost) without a full canary:

```bash
# 1. Pre-flight
starapihub health
docker inspect cp-bifrost --format '{{.Config.Image}}'  # Record current version

# 2. Backup
docker cp cp-bifrost:/app/data ./bifrost-backup-$(date +%Y%m%d)

# 3. Update version in common.env and recreate
cd control-plane/deploy
docker-compose --env-file env/common.env pull bifrost
docker-compose --env-file env/common.env up -d bifrost

# 4. Verify
starapihub health
starapihub diff
bash scripts/smoke/run-all.sh

# 5. Rollback (if needed): revert version in common.env, pull, recreate
```

See `docs/upgrade-strategy.md` for per-service upgrade details and watch-for notes. For monitoring setup to observe the canary and soak periods, see `docs/monitoring.md`.

## Commercial Appliance Overlay

If you are shipping the commercial-appliance variant, add these gates before Phase 4 is considered complete:

- [ ] One-click bootstrap flow succeeds from a fresh environment
- [ ] Drift detection runs clean against the desired-state registries
- [ ] Request correlation works according to `docs/observability.md`
- [ ] `docs/version-matrix.md` is updated with validated upstream versions
- [ ] Any upstream patches are documented and approved through `docs/patch-audit-workflow.md`
   - Monitor each expansion for 24 hours before the next

### Exit Criteria
- [ ] Production stack healthy for 48+ hours
- [ ] Canary users report no issues
- [ ] Billing records are accurate
- [ ] Premium traffic confirmed official-only in Bifrost logs
- [ ] ClewdR confirmed isolated from public access
- [ ] Backup and restore tested (PostgreSQL dump/restore cycle)
- [ ] Runbook procedures executed at least once in production context

## Phase 5: Ongoing Operations

**Goal**: Maintain the stack, handle upgrades, and evolve policies over time.

### Recurring Tasks

| Task | Frequency | Reference Doc |
|------|-----------|---------------|
| Check ClewdR cookie health | Daily | `docs/clewdr-operations.md` |
| Review error logs | Daily | `docs/runbook.md` |
| Rotate ClewdR cookies | As needed (weekly typical) | `docs/clewdr-operations.md` |
| Database backup verification | Weekly | `docs/runbook.md` |
| Disk usage check | Weekly | `docs/runbook.md` |
| Failure drills | Quarterly | `docs/failure-drills.md` |
| Upstream version review | Monthly | `docs/upgrade-strategy.md` |
| Policy registry review | Monthly | `docs/policies.md` |

### Adding New Models

When a new model is released by a provider:

1. Update `policies/logical-models.yaml` with the new logical model entry
2. Update `policies/provider-pools.yaml` if the model requires a new pool
3. Run the sync scripts to push config to New-API and Bifrost
4. Run targeted smoke tests (Tests 3, 4, 5 from `docs/verification.md`)
5. Announce the new model to users

### Upstream Upgrades

Follow `docs/upgrade-strategy.md` for the full procedure. Key points:
- Always upgrade in staging first
- Pin versions, never use `latest`
- Run the full smoke test suite after each upgrade
- Keep the previous version noted for rollback

## Risk Mitigation Across Phases

| Risk | Mitigation | Phase |
|------|-----------|-------|
| ClewdR leaks to premium traffic | Verify pool isolation in every phase | 1-5 |
| Config drift between registries and running systems | Re-run sync scripts periodically, compare output | 2-5 |
| Upstream upgrade breaks config | Always read release notes, test in staging | 4-5 |
| Secret exposure | Never commit real secrets, use env files outside version control | 0-5 |
| Single point of failure | Multiple ClewdR instances, Bifrost fallback pools, PostgreSQL backups | 1-5 |

## Summary

| Phase | Focus | Key Deliverable |
|-------|-------|----------------|
| 0 | Local dev | Single working request through the full chain |
| 1 | Multi-channel + ClewdR | Three-tier routing with provider isolation |
| 2 | Config sync | Registry-driven deployment, no manual config except cookies |
| 3 | Observability | Monitoring, log correlation, drills completed |
| 4 | Production | Real users, real traffic, operational readiness |
| 5 | Ongoing | Maintenance, upgrades, policy evolution |
