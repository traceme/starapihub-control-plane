# Monitoring and Alerting

## Overview

Monitoring is built on existing CLI commands (`starapihub health`, `starapihub diff`, `starapihub cookie-status`) run as cron jobs, with the dashboard webhook system providing real-time alerts. No external monitoring stack is needed for v1 — all checks are self-contained in the CLI binary and dashboard process.

**Alert severity and signal definitions live in [alert-model.md](alert-model.md).** This doc covers setup, delivery, and operational checklists — not severity semantics.

The monitoring system covers four critical paths:
1. **Service health** — are New-API, Bifrost, and ClewdR reachable?
2. **Drift detection** — has live state diverged from desired state?
3. **Sync failures** — did the last sync operation fail?
4. **Cookie depletion** — are there enough valid ClewdR cookies?

## Critical Path Checks

| Critical Path | Check Command | Frequency | Alert Condition |
|---|---|---|---|
| Service health | `starapihub health` | Every 60s | Any service unhealthy (exit code 1) |
| Drift detection | `starapihub diff --output json` | Every 5min | Blocking drift detected (exit code 1) |
| Sync failures | `jq 'select(.failed > 0)' ~/.starapihub/audit.log` | Every 5min | Any entry with failed > 0 |
| Cookie depletion | `starapihub cookie-status --min-valid 2` | Every 60s | Valid cookies below threshold (exit code 1) |

Each check uses process exit codes as the signal: exit 0 means healthy, exit 1 means action needed.

## Crontab Setup

Copy these entries into your crontab (`crontab -e`). Adjust `WEBHOOK_URL` and binary path as needed.

**Cron severity limitation:** The cron path only knows whether a command exited 0 (healthy) or non-zero (something wrong). It cannot distinguish which service failed or how severe the failure is. All cron alerts use `WARNING` — the operator must run the failing command interactively to determine actual severity. For granular severity (CRITICAL vs WARNING vs INFO), use the dashboard webhook path instead; it has full state awareness and classifies severity per the [alert model](alert-model.md).

The cron wrapper script lives at [`control-plane/scripts/starapihub-cron.sh`](../scripts/starapihub-cron.sh). It runs all four CLI checks (health, drift, sync audit, cookie status) and calls `send_alert` on failure.

**Install:**

```bash
sudo cp control-plane/scripts/starapihub-cron.sh /usr/local/bin/starapihub-cron.sh
sudo chmod +x /usr/local/bin/starapihub-cron.sh
```

**Add to crontab** (`crontab -e`):

```crontab
WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
NEWAPI_URL=http://localhost:3000
BIFROST_URL=http://localhost:8080
CLEWDR_URLS=http://clewdr-1:8484,http://clewdr-2:8484,http://clewdr-3:8484
CLEWDR_ADMIN_TOKENS=token1,token2,token3

* * * * * /usr/local/bin/starapihub-cron.sh
```

**Configurable env vars** (set in crontab or source an env file in the script):

| Var | Default | Purpose |
|-----|---------|---------|
| `WEBHOOK_URL` | (required) | HTTP endpoint for alert POST |
| `STARAPIHUB_BIN` | `/usr/local/bin/starapihub` | Path to CLI binary |
| `LOG_DIR` | `/var/log/starapihub` | Where check logs are written |
| `MIN_VALID_COOKIES` | `2` | Per-instance cookie threshold |
| `DRY_RUN` | (empty) | Set to `1` to print alerts to stderr instead of sending |

**Verify the script works before enabling cron** (see [Verify cron wrapper](#verify-cron-wrapper) below).

**Note:** Set `WEBHOOK_URL`, `NEWAPI_URL`, `BIFROST_URL`, `CLEWDR_URLS`, and `CLEWDR_ADMIN_TOKENS` as environment variables in the crontab header or via a wrapper script that sources an env file before running the check. The CLI also accepts the singular `CLEWDR_ADMIN_TOKEN` for backward compatibility (applied to all instances).

## Webhook Alerting

### Payload Format

Both alert sources send the same field set. See [alert-model.md → Webhook Payload Contract](alert-model.md#webhook-payload-contract) for the canonical definition.

**Dashboard payload** (`json.Marshal` of the full `store.Alert` struct):

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

**Cron payload** (from the `send_alert` helper in `starapihub-cron.sh`):

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

**Differences between the two sources:**

| Field | Dashboard | Cron |
|-------|-----------|------|
| `id` | Present (DB row ID) | Absent |
| `acknowledged` | Present (always `false` at fire time) | Absent |
| `severity` | Granular (CRITICAL/WARNING/INFO from live state) | Always `WARNING` (exit code only) |
| `type` | Same as `severity` (backward compat) | Same as `severity` |
| `signal` | From alert condition | From check type (static) |
| `timestamp` | From Go `time.Now()` | From shell `date -u` |

Receivers should treat `severity` as the primary field. `type` exists for v1.7 backward compatibility.

### Configuring Dashboard Webhooks

The dashboard process (`poller/alerts.go`) fires webhooks automatically when running. Unlike cron, it has full state awareness and classifies severity per the [alert model](alert-model.md). It checks these conditions every 15 seconds:

- **Cookie depletion** (0 valid cookies on an instance) — fires `CRITICAL` / `cookie-exhaustion`
- **High cookie utilization** (>80% of cookies used) — fires `WARNING` / `cookie-exhaustion`
- **Core service unhealthy >30s** (new-api, bifrost, nginx, postgres) — fires `CRITICAL` / `service-down`
- **Single ClewdR instance unhealthy >30s** — fires `INFO` / `service-down`

Configure the webhook URL via the dashboard config's `AlertWebhookURL` field. The dashboard deduplicates alerts: it will not fire the same severity + service combination within a 5-minute window.

These dashboard alerts run automatically alongside the crontab checks. The crontab provides a fallback in case the dashboard itself is down.

### Example Slack Webhook Receiver

A simple script that receives an alert payload on stdin (piped from curl) and forwards to Slack:

```bash
#!/bin/bash
# slack-forwarder.sh -- receives alert JSON on stdin, posts to Slack
# Usage: curl ... | bash slack-forwarder.sh
# Or: use WEBHOOK_URL pointing directly at Slack (simpler — no forwarder needed)

read -r PAYLOAD
SEVERITY=$(echo "$PAYLOAD" | jq -r '.severity // .type // "UNKNOWN"')
SERVICE=$(echo "$PAYLOAD" | jq -r '.service // "unknown"')
MESSAGE=$(echo "$PAYLOAD" | jq -r '.message // "Check failed"')

curl -s -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
  -H "Content-Type: application/json" \
  -d "{\"text\": \"[$SEVERITY] $SERVICE: $MESSAGE\"}"
```

### Example Generic HTTP Receiver

A minimal Python webhook receiver for logging and forwarding. Reads `severity` with fallback to `type` for backward compatibility:

```python
# webhook_receiver.py -- simple alert aggregator
from flask import Flask, request
import datetime

app = Flask(__name__)

@app.route("/webhook", methods=["POST"])
def webhook():
    data = request.json
    ts = data.get("timestamp", datetime.datetime.utcnow().isoformat())
    severity = data.get("severity") or data.get("type", "UNKNOWN")
    signal = data.get("signal", "")
    prefix = f"[{severity}]"
    if signal:
        prefix += f" [{signal}]"
    print(f"[{ts}] {prefix} {data['service']}: {data['message']}")
    # Forward to email, PagerDuty, OpsGenie, etc.
    return "ok", 200

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=9090)
```

## Alert Thresholds

| Check | Default | Adjust By |
|---|---|---|
| Health check interval | 60s | Crontab schedule |
| Drift check interval | 5min | Crontab schedule |
| Cookie minimum valid | 2 | `--min-valid` flag on `starapihub cookie-status` |
| Webhook dedup window | 5min | Dashboard config (`poller/alerts.go`) |
| Unhealthy grace period | 30s | Dashboard config (`poller/alerts.go`) |

## Log Locations

| Log | Path | Format |
|---|---|---|
| Health checks | `/var/log/starapihub/health.log` | JSON (one object per check) |
| Drift detection | `/var/log/starapihub/drift.log` | JSON (drift report) |
| Cookie status | `/var/log/starapihub/cookies.log` | JSON (cookie counts) |
| Sync audit | `~/.starapihub/audit.log` | JSONL (one entry per sync operation) |

Create the log directory before enabling cron jobs:

```bash
sudo mkdir -p /var/log/starapihub
sudo chown $(whoami) /var/log/starapihub
```

## Alert Delivery Setup

Follow these steps in order to enable alert delivery. Both paths (dashboard and cron) are independent — configure either or both.

### Step 1: Choose a receiver

You need an HTTP endpoint that accepts POST requests with JSON bodies. Common options:

| Receiver | Payload handling | Setup |
|----------|-----------------|-------|
| **Slack Incoming Webhook** | Slack accepts arbitrary JSON; use the text field for display | Create at api.slack.com → Your Apps → Incoming Webhooks |
| **PagerDuty Events API v2** | Forward via a small adapter that maps `severity` → PagerDuty severity | PagerDuty → Services → Integrations → Events API v2 |
| **Custom HTTP endpoint** | Your receiver parses the JSON fields directly | See [Example Generic HTTP Receiver](#example-generic-http-receiver) above |

### Step 2: Configure the dashboard webhook

Set `ALERT_WEBHOOK_URL` in your dashboard env file:

```bash
cd control-plane/deploy

# Edit the dashboard env file
echo 'ALERT_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL' >> env/dashboard.env

# Restart the dashboard to pick up the new env var
docker compose --env-file env/common.env restart dashboard
```

**Verify the env var was loaded:**

```bash
docker exec cp-dashboard env | grep ALERT_WEBHOOK_URL
```

The dashboard will now POST alerts to this URL whenever a condition fires (see [Configuring Dashboard Webhooks](#configuring-dashboard-webhooks) above).

### Step 3: Configure the cron webhook (optional fallback)

If the dashboard goes down, cron alerts are the fallback. Install the wrapper script from the repo:

```bash
cd control-plane
sudo cp scripts/starapihub-cron.sh /usr/local/bin/starapihub-cron.sh
sudo chmod +x /usr/local/bin/starapihub-cron.sh
```

Add to crontab (`crontab -e`):

```crontab
WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
NEWAPI_URL=http://localhost:3000
BIFROST_URL=http://localhost:8080
* * * * * /usr/local/bin/starapihub-cron.sh
```

### Step 4: Validate delivery end to end

Run the test-alert procedure below to prove alerts actually arrive at your receiver.

## Test Alert Procedure

This procedure validates that alert delivery works end to end. Run it after initial setup and after any webhook URL change.

### Test 1: Dashboard webhook delivery

Send a test alert through the dashboard's alert path. This requires the dashboard to be running with `ALERT_WEBHOOK_URL` set.

```bash
# Temporarily stop a ClewdR instance to trigger a real alert
# (The dashboard fires an alert after 30s of unhealthy state)
cd control-plane/deploy
docker compose --env-file env/common.env stop clewdr-1

# Wait 45 seconds for the alert checker to fire
sleep 45

# Check the dashboard saw the unhealthy state
cd control-plane
set -a; source deploy/env/dashboard.env; set +a
curl -s -H "Authorization: Bearer $DASHBOARD_TOKEN" http://localhost:8090/api/alerts?limit=5 | jq '.alerts[0]'
```

**Expected:** The alert JSON shows `"signal": "service-down"`, `"service": "clewdr-1"`. Your receiver (Slack, etc.) should have received the webhook.

**Cleanup:**

```bash
cd control-plane/deploy
docker compose --env-file env/common.env start clewdr-1
```

### Test 2: Cron webhook delivery

Send a test alert through the cron path by sourcing the wrapper and calling `send_alert`:

```bash
export WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
source /usr/local/bin/starapihub-cron.sh
send_alert WARNING test-alert test "This is a test alert from starapihub-cron.sh"
```

**Expected:** Your receiver shows a WARNING alert with service `test` and the test message.

### Verify cron wrapper

Before enabling the cron schedule, verify the wrapper script itself:

```bash
# 1. Syntax check
bash -n /usr/local/bin/starapihub-cron.sh && echo "Syntax OK"

# 2. Dry-run — prints what would be sent, does not POST
DRY_RUN=1 /usr/local/bin/starapihub-cron.sh
```

**Expected:** Dry-run prints `[DRY_RUN] Would send: {...}` lines for each check that would fire. No HTTP requests are made.

### Test 3: Dedup verification

Fire the same alert twice within 5 minutes and confirm only one webhook is sent:

```bash
# From the dashboard API, check alert count
curl -s -H "Authorization: Bearer $DASHBOARD_TOKEN" http://localhost:8090/api/alerts?limit=10 \
  | jq '[.alerts[] | select(.service == "clewdr-1" and .signal == "service-down")] | length'
```

**Expected:** 1 (not 2), because the dashboard deduplicates alerts with the same severity + service within a 5-minute window.

## Monitoring Checklist

### Initial Setup

- [ ] Install `starapihub` binary to `/usr/local/bin/`
- [ ] Set environment variables (`NEWAPI_URL`, `BIFROST_URL`, `CLEWDR_URLS`, `CLEWDR_ADMIN_TOKENS`)
- [ ] Create `/var/log/starapihub/` directory
- [ ] Set `ALERT_WEBHOOK_URL` in `deploy/env/dashboard.env` and restart dashboard
- [ ] Install `scripts/starapihub-cron.sh` to `/usr/local/bin/` and add crontab entry
- [ ] Run [Test Alert Procedure](#test-alert-procedure) to validate delivery

### Daily Verification

- [ ] Verify cron jobs are running: `crontab -l | grep starapihub`
- [ ] Check for recent alerts: `tail -20 /var/log/starapihub/health.log`
- [ ] Verify cookie counts: `starapihub cookie-status`
- [ ] Review sync audit for failures: `grep '"failed":[1-9]' ~/.starapihub/audit.log`

### Weekly Maintenance

- [ ] Check log rotation is configured for `/var/log/starapihub/`
- [ ] Review drift trends: `starapihub diff --output json | jq '.summary'`
- [ ] Verify webhook delivery: re-run [Test 2](#test-2-cron-webhook-delivery) of the test alert procedure
- [ ] Review and adjust `--min-valid` threshold based on cookie consumption rate

## See Also

- [Alert Model](alert-model.md) — Severity definitions and signal catalog (single source of truth)
- [Runbook](runbook.md) — Operational procedures for startup, shutdown, and recovery
- [Observability](observability.md) — Request correlation and tracing with X-Request-ID
- [Failure Drills](failure-drills.md) — Failure scenario testing procedures
- [Day-2 Operations](day-2-operations.md) — Daily/weekly checklists, failure triage
