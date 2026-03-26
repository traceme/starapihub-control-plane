# Secrets Bootstrap Guide

## Purpose

This document lists every secret required before first run, where each secret is consumed, and what fails if it's missing.

Bootstrap is a two-phase operation because New-API's `/api/setup` creates the admin user but does NOT return an auth token. Phase 1 seeds admin (`--skip-sync`), the operator obtains a token from the admin UI, then Phase 2 syncs config (`--skip-seed`). See [install.md](install.md) Steps 5-7.

For rotation of existing secrets, see [provider-secrets.md](provider-secrets.md).
For the full install path, see [install.md](install.md).

## Required Secrets by Phase

### Before `docker compose up` (Step 4)

These must be set in env files before any container starts.

| Secret | File | Why | Generate |
|--------|------|-----|----------|
| `POSTGRES_PASSWORD` | `common.env` | PostgreSQL won't start without a password | `openssl rand -hex 16` |
| `REDIS_PASSWORD` | `common.env` | Redis authentication | `openssl rand -hex 16` |
| `SESSION_SECRET` | `new-api.env` | New-API session signing (JWT) | `openssl rand -hex 32` |
| `DASHBOARD_TOKEN` | `dashboard.env` | Dashboard API authentication | `openssl rand -hex 32` |

**If missing:** Containers may start but services will crash or reject connections. Check `docker logs cp-<service>` for auth errors.

### Before `starapihub bootstrap --skip-sync` (Step 5 — seed admin)

| Secret | Source | Why |
|--------|--------|-----|
| `NEWAPI_ADMIN_PASSWORD` | Operator-chosen, ≥8 chars | Bootstrap seeds admin account via `POST /api/setup` |

Service URL env vars (`NEWAPI_URL`, `BIFROST_URL`) are also required. These come from `dashboard.env` (see env file reference below).

**If missing:** Bootstrap fails at the seed-admin step. The admin account is not created.

### After seed, before sync (Step 6 — obtain tokens)

These are obtained from the New-API admin UI after the admin account exists.

| Secret | Source | Why |
|--------|--------|-----|
| `NEWAPI_ADMIN_TOKEN` | New-API admin UI → Tokens → Add Token | Sync engine calls admin API endpoints (`/api/channel/`, etc.) |
| `API_KEY` | New-API admin UI → Tokens → Add Token | Client inference token for smoke tests and `/v1/models` |

**Critical:** `NEWAPI_ADMIN_TOKEN` cannot exist before the admin is seeded. This is why bootstrap has a two-phase flow. `/api/setup` does NOT return a token.

### Before `starapihub bootstrap --skip-seed` (Step 7 — sync)

These must be exported in the CLI's shell environment before the sync phase.

| Secret | File | Why | Source |
|--------|------|-----|--------|
| `NEWAPI_ADMIN_TOKEN` | Shell env | Sync engine authenticates to New-API admin API | Step 6 |
| At least one provider API key | `bifrost.env` | Sync pushes provider keys to Bifrost | Provider dashboard (Anthropic, OpenAI, OpenRouter) |

Provider API key env var names (set whichever providers you use):

| Env Var | Provider |
|---------|----------|
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENAI_API_KEY` | OpenAI |
| `OPENROUTER_API_KEY` | OpenRouter |

**If missing:** Bootstrap reports "NEWAPI_ADMIN_TOKEN env var not set" at the prereq-validation step and stops.

### After sync, before smoke tests (Step 8 — verification)

| Secret | Source | Why |
|--------|--------|-----|
| `API_KEY` | Step 6 | Smoke tests and `/v1/models` require a client API token |

**If missing:** Smoke tests skip or fail with 401. The system itself is functional — only verification is blocked.

## Complete Env File Reference

### `common.env` — Docker Compose interpolation

| Variable | Required | Default | Consumed By |
|----------|----------|---------|-------------|
| `POSTGRES_PASSWORD` | Yes | — | PostgreSQL container, New-API `SQL_DSN` |
| `POSTGRES_USER` | No | `newapi` | PostgreSQL container |
| `POSTGRES_DB` | No | `newapi` | PostgreSQL container |
| `REDIS_PASSWORD` | Yes | — | Redis container, New-API `REDIS_CONN_STRING` |
| `NEWAPI_VERSION` | No | `latest` | Docker Compose image tag |
| `BIFROST_VERSION` | No | `latest` | Docker Compose image tag |
| `STARAPIHUB_MODE` | Yes | `upstream` | Dashboard version reporting |
| `CLEWDR_IMAGE` | No | `clewdr:local` | Docker Compose image |
| `PUBLIC_HTTPS_PORT` | No | `443` | Nginx published port |
| `PUBLIC_HTTP_PORT` | No | `80` | Nginx published port |
| `TLS_CERT_DIR` | No | `./certs` | Nginx TLS mount |

### `new-api.env` — New-API container

| Variable | Required | Default | Consumed By |
|----------|----------|---------|-------------|
| `SESSION_SECRET` | Yes | — | New-API JWT signing |
| `SQL_DSN` | Yes | — | New-API database connection (must match `POSTGRES_*` in common.env) |
| `REDIS_CONN_STRING` | Yes | — | New-API Redis connection (must match `REDIS_PASSWORD` in common.env) |

### `bifrost.env` — Bifrost container + sync CLI

| Variable | Required | Default | Consumed By |
|----------|----------|---------|-------------|
| `PORT` | No | `8080` | Bifrost listener |
| `HOST` | No | `0.0.0.0` | Bifrost listener |
| `LOG_STYLE` | No | `json` | Bifrost logging |
| `LOG_LEVEL` | No | `info` | Bifrost logging |
| `ANTHROPIC_API_KEY` | Conditional | — | Sync CLI → Bifrost HTTP API |
| `OPENAI_API_KEY` | Conditional | — | Sync CLI → Bifrost HTTP API |
| `OPENROUTER_API_KEY` | Conditional | — | Sync CLI → Bifrost HTTP API |
| `CLEWDR_*_PASSWORD` | Conditional | — | Sync CLI → Bifrost HTTP API |

"Conditional" means at least one provider key is required for the system to route traffic.

### `dashboard.env` — Dashboard container

| Variable | Required | Default | Consumed By |
|----------|----------|---------|-------------|
| `DASHBOARD_TOKEN` | Yes | — | Dashboard API auth |
| `NEWAPI_URL` | No | `http://new-api:3000` | Dashboard → New-API |
| `BIFROST_URL` | No | `http://bifrost:8080` | Dashboard → Bifrost |
| `CLEWDR_URLS` | No | — | Dashboard → ClewdR instances |
| `DASHBOARD_PORT` | No | `8090` | Dashboard listener |
| `STARAPIHUB_MODE` | Yes | `upstream` | Version reporting |

### `clewdr-*.env` — ClewdR instances (optional)

| Variable | Required | Default | Consumed By |
|----------|----------|---------|-------------|
| `CLEWDR_PASSWORD` | No | auto-generated | ClewdR instance auth |
| `CLEWDR_ADMIN_PASSWORD` | No | auto-generated | ClewdR admin UI auth |

## Validation Before Bootstrap

Run this checklist before `starapihub bootstrap`:

```bash
# 1. Check all env files exist
ls deploy/env/common.env deploy/env/new-api.env deploy/env/bifrost.env deploy/env/dashboard.env

# 2. Check for unfilled placeholder values
grep -n 'CHANGE_ME' deploy/env/*.env
# Should return no matches — all placeholders should be replaced with real values

# 3. Check critical secrets have values (not just blank lines)
for var in POSTGRES_PASSWORD REDIS_PASSWORD; do
  grep -q "^${var}=.\+" deploy/env/common.env && echo "$var: OK" || echo "$var: MISSING"
done
grep -q "^SESSION_SECRET=.\+" deploy/env/new-api.env && echo "SESSION_SECRET: OK" || echo "SESSION_SECRET: MISSING"
grep -q "^DASHBOARD_TOKEN=.\+" deploy/env/dashboard.env && echo "DASHBOARD_TOKEN: OK" || echo "DASHBOARD_TOKEN: MISSING"

# 4. Check at least one provider key is set in bifrost.env
grep -cE '^(ANTHROPIC|OPENAI|OPENROUTER)_API_KEY=.+' deploy/env/bifrost.env
# Should return at least 1
```

## Missing-Secret Failure Modes

| Missing Secret | When It Fails | Symptom | Fix |
|---------------|---------------|---------|-----|
| `POSTGRES_PASSWORD` | Container start | `cp-new-api` crashes with "password authentication failed" | Set in `common.env`, restart |
| `REDIS_PASSWORD` | Container start | New-API logs "NOAUTH Authentication required" | Set in `common.env`, restart |
| `SESSION_SECRET` | First login | New-API rejects all sessions | Set in `new-api.env`, restart New-API |
| `DASHBOARD_TOKEN` | Dashboard access | 401 on all `/api/*` endpoints | Set in `dashboard.env`, restart dashboard |
| `NEWAPI_ADMIN_PASSWORD` | Bootstrap seed step | Admin seed fails or creates user with empty password | Export before `bootstrap --skip-sync` |
| `NEWAPI_ADMIN_TOKEN` | Bootstrap sync step | Prereq validation fails: "NEWAPI_ADMIN_TOKEN env var not set" | Log in to admin UI, create token, export before `bootstrap --skip-seed` |
| Provider API key | Bootstrap sync step | "env var not set" error during provider sync | Set in `bifrost.env`, re-export, re-sync |
| `API_KEY` (client token) | Smoke tests | 401 on `/v1/models` and chat completions | Create token in New-API admin UI |

## Related Docs

- [Install Guide](install.md) — full first-install walkthrough
- [Provider Secrets](provider-secrets.md) — credential map, rotation, recovery
- [Provider Onboarding](provider-onboarding.md) — adding new provider keys
- [Runbook](runbook.md) — secret rotation procedures
