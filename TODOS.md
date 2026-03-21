# TODOs

## Completed

### ~~Unified health check endpoint~~
**Implemented:** `scripts/health-check.sh` — aggregates health of New-API, Bifrost, Postgres, Redis, and optionally ClewdR into a single JSON response. Supports `--with-clewdr`, `--quiet` modes.
**Completed:** 2026-03-21

### ~~ClewdR cookie rotation alerts~~
**Implemented:** `scripts/check-clewdr-cookies.sh` — calls `/api/cookies` on each ClewdR instance, reports valid/exhausted/invalid cookie counts, flags CRITICAL when zero valid cookies remain. Auto-reads admin tokens from docker logs. Supports `--json`, `--quiet` modes for cron integration.
**Completed:** 2026-03-21

## Phase 2 Candidates

(None currently — observe design partner's usage patterns to identify next priorities.)
