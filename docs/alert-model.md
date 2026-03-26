# Alert Model

## Purpose

This is the single source of truth for alert severity definitions and signal classification. All other docs (monitoring, day-2, CI, failure drills) reference this model — they do not define their own severity meanings.

## Severity Definitions

| Severity | Meaning | Expected Response | Example |
|----------|---------|-------------------|---------|
| **CRITICAL** | Traffic is dropping or will drop imminently. Operator must act now. | Respond within 5 minutes. | New-API down, all ClewdR cookies exhausted |
| **WARNING** | System is degraded but still serving traffic. Investigation needed. | Respond within 1 hour. | One ClewdR instance unhealthy, config drift detected |
| **INFO** | No action needed. Useful for trend analysis and audit. | Review in daily/weekly check. | Nightly passed, backup completed, provider key approaching expiry |

### Severity Rules

- A signal is **CRITICAL** only if traffic loss is happening or imminent.
- A signal is **WARNING** if the system is degraded but traffic is still flowing.
- Everything else is **INFO** — useful to know, not urgent to act on.
- If a WARNING persists for more than one check cycle without resolution, consider escalating to CRITICAL.

## Signal Catalog

Every monitored signal, its severity, and the operator response.

| Signal | Check Command | Severity | Alert Condition | Operator Action |
|--------|---------------|----------|-----------------|-----------------|
| **Service down** (New-API) | `starapihub health` | CRITICAL | New-API unreachable (exit 1) | All traffic blocked. Restart container, check PostgreSQL. See [day-2 → Service Down](day-2-operations.md#service-down). |
| **Service down** (Bifrost) | `starapihub health` | CRITICAL | Bifrost unreachable (exit 1) | All inference fails (502). Restart container, check memory. See [day-2 → Service Down](day-2-operations.md#service-down). |
| **Service down** (nginx) | `docker compose ps` | CRITICAL | nginx not `Up (healthy)` | No external access. Check config syntax: `docker exec cp-nginx nginx -t`. |
| **Service down** (PostgreSQL) | `docker exec cp-postgres pg_isready` | CRITICAL | PostgreSQL not accepting connections | New-API depends on it. Check disk, memory. |
| **Service down** (ClewdR, all) | `starapihub health` | WARNING | All ClewdR instances unreachable | Risky/standard tier degraded; premium unaffected. Restart ClewdR containers. |
| **Service down** (ClewdR, one) | `starapihub health` | INFO | One ClewdR instance unreachable, others healthy | Bifrost routes around it. Investigate and restart when convenient. |
| **Cookie exhaustion** (all instances) | `starapihub cookie-status --min-valid 2` | CRITICAL | 0 valid cookies across all instances (exit 1) | ClewdR tier non-functional. Replenish cookies. See [runbook → Cookie Exhaustion](runbook.md#cookie-exhaustion). |
| **Cookie exhaustion** (per-instance) | `starapihub cookie-status --min-valid 2` | WARNING | Any instance below threshold but others still valid | Reduced capacity. Replenish soon. |
| **Config drift** (blocking) | `starapihub diff` | WARNING | Blocking drift detected (exit 1) | Live state diverged from desired. Run `starapihub sync --dry-run` then `starapihub sync`. See [runbook → Drift Detected](runbook.md#drift-detected). |
| **Sync failure** | Audit log: `jq 'select(.failed > 0)' ~/.starapihub/audit.log` | WARNING | Any sync operation had failures | Reconciliation incomplete. Re-run `starapihub sync` and check audit log. |
| **Nightly failure** | `gh run list --workflow=nightly.yml --limit=1` | WARNING | Latest nightly concluded as `failure` | Environment breakage detected. Download artifacts and triage. See [ci-guide.md](ci-guide.md). |
| **Nightly green streak < 3** | `gh run list --workflow=nightly.yml --limit=7` | INFO | Fewer than 3 consecutive green runs | Low promotion confidence. Investigate recent failures. |
| **Backup stale** | `ls -lt backups/*/postgres.dump` | INFO | Most recent backup older than 24h | Run a backup. See [backup-restore.md](backup-restore.md). |
| **Provider error** (401/403) | Provider-specific curl | WARNING | Provider rejecting requests with auth error | Key expired or revoked. Rotate key. See [provider-secrets.md](provider-secrets.md). |
| **Provider error** (429) | Provider-specific curl | INFO | Rate limited by provider | Transient. Wait or reduce traffic. |
| **Provider error** (5xx) | Provider-specific curl | INFO | Provider outage | External issue. Check provider status page. |
| **Redis down** | `docker compose ps` | INFO | Redis not running | Sessions lost, rate limits reset. Usually recovers on restart. No traffic loss. |

## Severity by Failure Class

Summary view for quick triage — maps to the [day-2 failure triage](day-2-operations.md#failure-triage) categories.

| Failure Class | CRITICAL | WARNING | INFO |
|---------------|----------|---------|------|
| **Service down** | New-API, Bifrost, nginx, PostgreSQL | All ClewdR instances | Single ClewdR instance, Redis |
| **Config drift** | — | Blocking drift, sync failure | — |
| **Provider failure** | — | Auth errors (401/403) | Rate limits (429), outages (5xx) |
| **Cookie exhaustion** | All instances at 0 valid | Per-instance below threshold | — |
| **Product defect** | — | Sync succeeds but behavior wrong | — |

## Webhook Payload Contract

There are two alert sources with different payload shapes. Receivers should handle both.

### Dashboard payload

Emitted by `poller/alerts.go` via `json.Marshal(store.Alert)`. Severity is classified from live state.

```json
{
  "id": 42,
  "severity": "CRITICAL",
  "type": "CRITICAL",
  "signal": "service-down",
  "service": "new-api",
  "message": "new-api has been unhealthy for 45s",
  "timestamp": "2026-03-26T10:30:00Z",
  "acknowledged": false
}
```

### Cron payload

Emitted by [`scripts/starapihub-cron.sh`](../scripts/starapihub-cron.sh) (see [monitoring.md](monitoring.md#crontab-setup)). Severity is always `WARNING` because exit codes cannot distinguish failure severity.

```json
{
  "severity": "WARNING",
  "type": "WARNING",
  "signal": "service-down",
  "service": "health",
  "message": "One or more services unhealthy — run starapihub health to identify which",
  "timestamp": "2026-03-26T10:30:00Z"
}
```

### Shared fields

Both sources include these fields. Receivers should key on these:

| Field | Type | Values | Description |
|-------|------|--------|-------------|
| `severity` | string | `CRITICAL`, `WARNING`, `INFO` | From the severity definitions above |
| `signal` | string | `service-down`, `cookie-exhaustion`, `config-drift`, `sync-failure`, `nightly-failure`, `provider-error` | Signal category |
| `service` | string | `new-api`, `bifrost`, `nginx`, `postgres`, `clewdr-1`, `clewdr-2`, `clewdr-3`, `cookies`, `drift`, `sync`, `nightly`, `health` | Affected service or subsystem |
| `message` | string | — | Human-readable description |
| `timestamp` | string | RFC 3339 | When the alert was generated |
| `type` | string | Same as `severity` | **Backward compat.** Receivers should prefer `severity`. |

### Dashboard-only fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Database row ID |
| `acknowledged` | boolean | Whether the alert was acknowledged in the dashboard UI |

### Source comparison

| Property | Dashboard | Cron |
|----------|-----------|------|
| Severity accuracy | Granular — per service and condition | Always `WARNING` — exit code only |
| Runs when | Dashboard process is up | Cron daemon is up |
| Primary use | Main alert path | Fallback when dashboard is down |

The dashboard is the primary alert source. Since cron always emits `WARNING`, operators who receive a cron alert should run the failing command interactively to determine actual severity.

## How Other Docs Reference This Model

- **monitoring.md** — uses this model for webhook payloads; documents cron WARNING limitation
- **day-2-operations.md** — signal reference table includes severity column referencing this model
- **failure-drills.md** — each drill states which alerts should fire and at what severity
- **ci-guide.md** — nightly failure severity defined here, not in CI docs

## Related Docs

- [Monitoring and Alerting](monitoring.md) — crontab setup, webhook delivery, receiver examples
- [Day-2 Operations](day-2-operations.md) — daily/weekly checklists, failure triage
- [Failure Drills](failure-drills.md) — component failure scenarios
- [CI Guide](ci-guide.md) — workflow details, nightly interpretation
