# TODOs

## Phase 2 Candidates

### Unified health dashboard endpoint
**What:** Add a `/health` endpoint to nginx that aggregates health status of all upstream services (new-api, bifrost, postgres, redis, optionally clewdr) into a single JSON response.
**Why:** Operators need to check stack health without inspecting 8 containers individually. Each upstream has its own healthcheck but there's no unified view.
**Pros:** One curl command shows full stack health. Prevents "which container is broken?" debugging.
**Cons:** Nginx doesn't natively aggregate upstream health — needs Lua module or a lightweight sidecar.
**Context:** Current healthchecks: New-API `/api/status`, Bifrost `/health`, ClewdR port check. No aggregation exists.
**Depends on:** Phase 1 completion. Observe design partner's debugging patterns to determine priority.
**Added:** 2026-03-20 (eng review)

### ClewdR cookie rotation alerts
**What:** A script or cron job that checks ClewdR instances for cookie expiry and alerts the operator before cookies expire.
**Why:** ClewdR cookies expire silently. When they do, inference fails with "No cookie available" — the operator has no warning until a user complains.
**Pros:** Prevents silent degradation of the unofficial Claude path.
**Cons:** Requires understanding ClewdR's cookie expiry behavior and admin API. May need ClewdR source inspection.
**Context:** Current ClewdR healthcheck only verifies port is open, not cookie validity. The "No cookie available" response is the only signal today.
**Depends on:** ClewdR cookie setup (Phase 1 manual step). Becomes relevant only after ClewdR is actively used.
**Added:** 2026-03-20 (eng review)
