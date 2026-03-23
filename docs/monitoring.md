# Monitoring and Alerting

## Overview

Monitoring is built on existing CLI commands (`starapihub health`, `starapihub diff`, `starapihub cookie-status`) run as cron jobs, with the dashboard webhook system providing real-time alerts. No external monitoring stack is needed for v1 -- all checks are self-contained in the CLI binary and dashboard process.

The monitoring system covers four critical paths:
1. **Service health** -- are New-API, Bifrost, and ClewdR reachable?
2. **Drift detection** -- has live state diverged from desired state?
3. **Sync failures** -- did the last sync operation fail?
4. **Cookie depletion** -- are there enough valid ClewdR cookies?

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

```crontab
# Environment variables (set these or use a wrapper script)
# NEWAPI_URL=http://newapi:3000
# BIFROST_URL=http://bifrost:8080
# CLEWDR_URLS=http://clewdr-1:8484,http://clewdr-2:8484
# CLEWDR_ADMIN_TOKEN=your-admin-token
# WEBHOOK_URL=https://your-webhook-endpoint.example.com/alerts

# Service health check - every 60 seconds
* * * * * /usr/local/bin/starapihub health --output json >> /var/log/starapihub/health.log 2>&1 || curl -s -X POST "$WEBHOOK_URL" -H "Content-Type: application/json" -d '{"type":"CRITICAL","service":"health","message":"One or more services unhealthy"}'

# Drift detection - every 5 minutes
*/5 * * * * /usr/local/bin/starapihub diff --output json >> /var/log/starapihub/drift.log 2>&1 || curl -s -X POST "$WEBHOOK_URL" -H "Content-Type: application/json" -d '{"type":"WARNING","service":"drift","message":"Blocking drift detected"}'

# Sync failure audit - every 5 minutes
*/5 * * * * jq -c 'select(.failed > 0)' ~/.starapihub/audit.log | tail -1 | grep -q . && curl -s -X POST "$WEBHOOK_URL" -H "Content-Type: application/json" -d '{"type":"WARNING","service":"sync","message":"Recent sync operation had failures"}'

# Cookie depletion check - every 60 seconds
* * * * * /usr/local/bin/starapihub cookie-status --min-valid 2 --output json >> /var/log/starapihub/cookies.log 2>&1 || curl -s -X POST "$WEBHOOK_URL" -H "Content-Type: application/json" -d '{"type":"CRITICAL","service":"cookies","message":"Valid cookie count below threshold"}'
```

**Note:** Set `WEBHOOK_URL`, `NEWAPI_URL`, `BIFROST_URL`, `CLEWDR_URLS`, and `CLEWDR_ADMIN_TOKEN` as environment variables in the crontab header or via a wrapper script that sources an env file before running the check.

## Webhook Alerting

### Payload Format

All alerts (both from crontab `curl` commands and from the dashboard's built-in alert system) use this JSON format:

```json
{
  "type": "CRITICAL",
  "service": "clewdr-1",
  "message": "clewdr-1 has 0 valid cookies out of 4 total",
  "timestamp": "2026-03-22T10:30:00Z"
}
```

Fields:

| Field | Type | Description |
|---|---|---|
| `type` | string | `CRITICAL` (action needed now) or `WARNING` (investigate soon) |
| `service` | string | Service identifier: `new-api`, `bifrost`, `clewdr-1`, `cookies`, `drift`, `sync`, `health` |
| `message` | string | Human-readable description of the alert condition |
| `timestamp` | string | RFC 3339 timestamp of when the alert was generated |

### Configuring Dashboard Webhooks

The dashboard process (`poller/alerts.go`) fires webhooks automatically when running. It checks for these conditions every 15 seconds:

- **Cookie depletion** (0 valid cookies) -- fires `CRITICAL` alert
- **High cookie utilization** (>80% of cookies used) -- fires `WARNING` alert
- **Service unhealthy for >30 seconds** -- fires `WARNING` alert

Configure the webhook URL via the dashboard config's `AlertWebhookURL` field. The dashboard deduplicates alerts: it will not fire the same alert type + service combination within a 5-minute window.

These dashboard alerts run automatically alongside the crontab checks. The crontab provides a fallback in case the dashboard itself is down.

### Example Slack Webhook Receiver

Forward alerts to a Slack incoming webhook:

```bash
#!/bin/bash
# slack-alert.sh -- called by crontab on check failure
TYPE="${1:-CRITICAL}"
SERVICE="${2:-unknown}"
MESSAGE="${3:-Check failed}"

curl -s -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
  -H "Content-Type: application/json" \
  -d "{\"text\": \"[$TYPE] $SERVICE: $MESSAGE\"}"
```

### Example Generic HTTP Receiver

A minimal Python webhook receiver for logging and forwarding:

```python
# webhook_receiver.py -- simple alert aggregator
from flask import Flask, request
import datetime

app = Flask(__name__)

@app.route("/webhook", methods=["POST"])
def webhook():
    data = request.json
    ts = data.get("timestamp", datetime.datetime.utcnow().isoformat())
    print(f"[{ts}] [{data['type']}] {data['service']}: {data['message']}")
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

## Monitoring Checklist

### Initial Setup

- [ ] Install `starapihub` binary to `/usr/local/bin/`
- [ ] Set environment variables (`NEWAPI_URL`, `BIFROST_URL`, `CLEWDR_URLS`, `CLEWDR_ADMIN_TOKEN`)
- [ ] Create `/var/log/starapihub/` directory
- [ ] Add crontab entries from the Crontab Setup section
- [ ] Configure webhook URL (Slack, PagerDuty, or custom receiver)
- [ ] Send a test alert to verify webhook delivery

### Daily Verification

- [ ] Verify cron jobs are running: `crontab -l | grep starapihub`
- [ ] Check for recent alerts: `tail -20 /var/log/starapihub/health.log`
- [ ] Verify cookie counts: `starapihub cookie-status`
- [ ] Review sync audit for failures: `grep '"failed":[1-9]' ~/.starapihub/audit.log`

### Weekly Maintenance

- [ ] Check log rotation is configured for `/var/log/starapihub/`
- [ ] Review drift trends: `starapihub diff --output json | jq '.summary'`
- [ ] Verify webhook delivery: send a test alert manually
- [ ] Review and adjust `--min-valid` threshold based on cookie consumption rate

## See Also

- `docs/runbook.md` -- Operational procedures for startup, shutdown, and recovery
- `docs/observability.md` -- Request correlation and tracing with X-Request-ID
- `docs/failure-drills.md` -- Failure scenario testing procedures
