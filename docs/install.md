# Installation Guide

## Purpose

This is the canonical first-install path for StarAPIHub. Follow this document from top to bottom to go from a clean machine to a verified healthy system.

For day-2 operations after installation, see [day-2-operations.md](day-2-operations.md) (when available) or [runbook.md](runbook.md).

## Prerequisites

Before starting, you need:

| Requirement | Why | How to check |
|-------------|-----|--------------|
| Docker Engine 24+ | Runs all services | `docker --version` |
| Docker Compose v2 | Orchestration | `docker compose version` |
| Go 1.22+ | Build the CLI binary | `go version` |
| 4 GB RAM, 2 CPU | Minimum for full stack | `free -h` / `sysctl hw.memsize` |
| At least one provider API key | Bifrost needs a provider to route to | Anthropic, OpenAI, or OpenRouter dashboard |

**Optional:**
- A domain name + TLS certificate (for production — self-signed works for dev/staging)
- Claude.ai cookies (only if using ClewdR for unofficial provider access)

## Choose Your Mode

StarAPIHub supports two operating modes. Choose before you start.

| Mode | What it means | When to use |
|------|-------------|-------------|
| **Upstream** | All upstream images are unmodified vendor releases | Default. Use unless you need Patch 001 (X-Request-ID propagation) |
| **Appliance** | Includes a patched New-API image (`starapihub/new-api:patched`) | Use when end-to-end request correlation is required |

The mode is set in `deploy/env/common.env` as `STARAPIHUB_MODE=upstream` or `STARAPIHUB_MODE=appliance`.

## Step 1: Clone and Build the CLI

```bash
git clone <repo-url> starapihub
cd starapihub/control-plane

# Build the CLI binary (includes version stamping via ldflags)
make build

# Verify
./dashboard/starapihub --help
```

The CLI binary is written to `dashboard/starapihub`. If you prefer building without Make:

```bash
cd dashboard && go build -o starapihub ./cmd/starapihub/ && cd ..
```

## Step 2: Generate Environment Files

**Option A — Automated** (recommended for first install):

```bash
cd control-plane
bash scripts/setup.sh --no-start
# Generates .env files from templates with random passwords
```

**Option B — Manual:**

```bash
cd control-plane/deploy/env
cp common.env.example common.env
cp new-api.env.example new-api.env
cp bifrost.env.example bifrost.env
cp dashboard.env.example dashboard.env
# Only if using ClewdR:
cp clewdr-1.env.example clewdr-1.env
cp clewdr-2.env.example clewdr-2.env
cp clewdr-3.env.example clewdr-3.env
```

Then edit each file. See [secrets-bootstrap.md](secrets-bootstrap.md) for the complete list of required secrets.

### Minimum required edits

| File | Variable | What to set |
|------|----------|-------------|
| `common.env` | `POSTGRES_PASSWORD` | Strong random password |
| `common.env` | `REDIS_PASSWORD` | Strong random password |
| `common.env` | `STARAPIHUB_MODE` | `upstream` or `appliance` |
| `new-api.env` | `SESSION_SECRET` | `openssl rand -hex 32` |
| `bifrost.env` | Provider keys | At least one of `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY` |
| `dashboard.env` | `DASHBOARD_TOKEN` | `openssl rand -hex 32` |

## Step 3: Prepare TLS

**For development** (self-signed):

```bash
cd control-plane
bash scripts/gen-certs.sh
```

**For production** (CA-signed):

```bash
mkdir -p deploy/certs
cp /path/to/fullchain.pem deploy/certs/server.crt
cp /path/to/privkey.pem deploy/certs/server.key
chmod 600 deploy/certs/server.key
```

If using a production domain, edit `config/nginx/nginx.conf` and replace `api.example.com` with your domain.

## Step 4: Start the Stack

```bash
cd control-plane/deploy

# Without ClewdR (official API providers only):
docker compose --env-file env/common.env up -d

# With ClewdR:
docker compose --env-file env/common.env --profile clewdr up -d
```

Wait 30-60 seconds for all services to become healthy:

```bash
docker compose --env-file env/common.env ps
# All services should show "Up (healthy)"
```

## Step 5: Seed Admin Account

Bootstrap has a two-phase flow because New-API's `/api/setup` endpoint creates the admin user but does **not** return an auth token. The sync engine needs an admin token to push configuration. So:

**Phase 1 — Seed admin (no sync):**

```bash
cd control-plane

# Export service URLs from dashboard env
set -a; source deploy/env/dashboard.env; set +a

# Seed the admin account only (skip sync — we don't have an admin token yet)
NEWAPI_ADMIN_PASSWORD=your-chosen-password \
  ./dashboard/starapihub bootstrap --skip-sync
```

This creates the admin user in New-API via `POST /api/setup`.

## Step 6: Obtain Admin Token

New-API's setup endpoint does **not** return a token. You must log in and create one.

1. **Log into the New-API admin UI** at `http://localhost:3000`:
   - Username: `root` (or the value of `NEWAPI_ADMIN_USERNAME` if set)
   - Password: the value you set for `NEWAPI_ADMIN_PASSWORD` in Step 5

2. **Create an admin API token** for the sync engine:
   - Navigate to **Tokens** in the admin sidebar
   - Click **Add Token**
   - Give it a name (e.g., `sync-admin`)
   - Copy the generated `sk-...` token

3. **Create a client API token** for smoke tests:
   - Add another token (e.g., `operator-test`)
   - Copy the generated `sk-...` token

4. **Export both tokens:**

   ```bash
   # Admin token for sync engine (used by starapihub sync/bootstrap)
   export NEWAPI_ADMIN_TOKEN=sk-your-admin-token

   # Client token for inference and smoke tests
   export API_KEY=sk-your-client-token
   ```

**Token distinction:** `NEWAPI_ADMIN_TOKEN` is used by the sync engine to call New-API admin endpoints (`/api/channel/`, `/api/option/`, etc.). `API_KEY` is used by clients for inference (`/v1/chat/completions`, `/v1/models`). They can be the same token from a root user, but separating them is cleaner.

## Step 7: Sync Configuration

**Phase 2 — Sync with admin token:**

```bash
cd control-plane

# Export provider API keys
set -a; source deploy/env/bifrost.env; set +a

# Export service URLs and dashboard token
set -a; source deploy/env/dashboard.env; set +a

# Set the admin token obtained in Step 6
export NEWAPI_ADMIN_TOKEN=sk-your-admin-token

# Run bootstrap with sync (skip seed since admin already exists)
./dashboard/starapihub bootstrap --skip-seed
```

This syncs all policy registries (providers, channels, models, routing rules, pricing) to the live system and verifies health.

### If bootstrap fails

| Symptom | Fix |
|---------|-----|
| "service not healthy" | Check `docker compose ps` — is the failing service running? Check `docker logs cp-<service>` |
| "NEWAPI_ADMIN_TOKEN env var not set" | Obtain the token from Step 6 and export it |
| "sync failed" | Check service health first (`starapihub health`), then retry |

## Step 8: Verify Installation

Run these checks in order:

```bash
cd control-plane

# 1. All services healthy
./dashboard/starapihub health

# 2. No configuration drift
./dashboard/starapihub diff

# 3. Sync is clean
./dashboard/starapihub sync --dry-run
# Should show 0 pending changes

# 4. Models are visible (requires API_KEY from Step 6)
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/v1/models | jq '.data[].id'

# 5. Smoke test (full suite — requires API_KEY)
API_KEY=$API_KEY bash scripts/smoke/run-all.sh

# 6. Dashboard accessible
curl -sf -H "Authorization: Bearer $DASHBOARD_TOKEN" \
  http://localhost:8090/api/health && echo "Dashboard OK"
```

If all six checks pass, installation is complete.

## Step 9: ClewdR Setup (Optional)

If you started with the `--profile clewdr` flag:

1. Get each instance's admin password:
   ```bash
   docker logs cp-clewdr-1 2>&1 | head -20 | grep -i password
   ```

2. Add cookies via each instance's admin UI (port 8484 inside Docker — use SSH tunnel or temporary port exposure)

3. Verify cookie status:
   ```bash
   ./dashboard/starapihub cookie-status
   ```

See [clewdr-operations.md](clewdr-operations.md) for detailed cookie management.

## Post-Install Checklist

After a successful installation, verify these one-time items:

- [ ] All services healthy: `starapihub health` exits 0
- [ ] Sync clean: `starapihub sync --dry-run` shows 0 pending
- [ ] No blocking drift: `starapihub diff` exits 0
- [ ] At least one model responds to inference
- [ ] Dashboard accessible with `DASHBOARD_TOKEN`
- [ ] Env files backed up to a secure location (they contain secrets)
- [ ] If appliance mode: patched New-API image built and X-Request-ID verified

## What to Read Next

| Topic | Doc |
|-------|-----|
| Adding providers | [provider-onboarding.md](provider-onboarding.md) |
| OpenRouter setup | [openrouter-operations.md](openrouter-operations.md) |
| Alert delivery setup | [monitoring.md → Alert Delivery Setup](monitoring.md#alert-delivery-setup) |
| Secret rotation | [provider-secrets.md](provider-secrets.md) |
| Backup and restore | [backup-restore.md](backup-restore.md) |
| Daily operations | [day-2-operations.md](day-2-operations.md) |
| Upgrade strategy | [upgrade-strategy.md](upgrade-strategy.md) |
| Release process | [promotion-criteria.md](promotion-criteria.md) |

## Upstream vs Appliance: What Differs

| Step | Upstream | Appliance |
|------|----------|-----------|
| Mode setting | `STARAPIHUB_MODE=upstream` | `STARAPIHUB_MODE=appliance` |
| New-API image | `calciumion/new-api` (vendor) | `starapihub/new-api:patched` (built locally) |
| Patch 001 | Not active | Active (X-Request-ID propagation) |
| RC validation | 17/17 applicable gates | 20/20 gates |
| Image to build | Dashboard only | Dashboard + patched New-API |

For appliance mode, build the patched image before Step 4:

```bash
cd control-plane
make build-patched-newapi
```

## Related Docs

- [Deploy README](../deploy/README.md) — Docker Compose details, TLS setup, architecture diagram
- [Secrets Bootstrap](secrets-bootstrap.md) — complete secret prerequisites (when available)
- [Provider Secrets](provider-secrets.md) — credential map, rotation, recovery
- [Runbook](runbook.md) — startup, shutdown, rotation, incident response
- [Rollout Plan](rollout-plan.md) — phased deployment from dev to production
