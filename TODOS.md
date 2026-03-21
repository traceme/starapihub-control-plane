# TODOs

## Completed

### ~~Unified health check endpoint~~
**Implemented:** `scripts/health-check.sh` — aggregates health of New-API, Bifrost, Postgres, Redis, and optionally ClewdR into a single JSON response. Supports `--with-clewdr`, `--quiet` modes.
**Completed:** 2026-03-21

### ~~ClewdR cookie rotation alerts~~
**Implemented:** `scripts/check-clewdr-cookies.sh` — calls `/api/cookies` on each ClewdR instance, reports valid/exhausted/invalid cookie counts, flags CRITICAL when zero valid cookies remain. Auto-reads admin tokens from docker logs. Supports `--json`, `--quiet` modes for cron integration.
**Completed:** 2026-03-21

## Phase 2 Candidates

### Rate limiting on dashboard API
**What:** Add per-token rate limiting middleware to all `/api/*` endpoints.
**Why:** Prevents abuse and protects SQLite from write storms under concurrent access.
**Context:** Single-operator dashboard behind Docker network makes this low priority now. Becomes important when multi-user auth is added. Use `golang.org/x/time/rate` or in-memory token bucket. Token-based limiter is simplest since auth is already required.
**Depends on:** Multi-user auth (for per-user limits) — but a global limit can be added independently.

### Persist wizard state across page refreshes
**What:** Save wizard progress to `sessionStorage` (frontend) and backend wizard status endpoint.
**Why:** Currently the setup wizard resets to step 0 on browser refresh, even if earlier steps completed. Frustrating during onboarding if user needs to look something up mid-wizard.
**Context:** Backend already has `/api/wizard/status` endpoint that returns step completion state. Frontend needs to read it on mount and resume from the last completed step.
**Depends on:** Nothing.
