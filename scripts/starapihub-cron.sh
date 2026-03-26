#!/usr/bin/env bash
# starapihub-cron.sh — Cron alert wrapper for StarAPIHub.
#
# Runs CLI health checks and sends webhook alerts on failure.
# This is the fallback alert path — the dashboard webhook is primary.
# See docs/monitoring.md for setup and docs/alert-model.md for payload contract.
#
# Installation:
#   sudo cp control-plane/scripts/starapihub-cron.sh /usr/local/bin/starapihub-cron.sh
#   sudo chmod +x /usr/local/bin/starapihub-cron.sh
#
# Crontab entry:
#   * * * * * /usr/local/bin/starapihub-cron.sh
#
# Required env vars (set in crontab header, wrapper, or sourced env file):
#   WEBHOOK_URL       — HTTP endpoint that receives POST JSON alerts
#   NEWAPI_URL        — New-API base URL (e.g. http://localhost:3000)
#   BIFROST_URL       — Bifrost base URL (e.g. http://localhost:8080)
#
# Optional env vars:
#   CLEWDR_URLS           — Comma-separated ClewdR URLs
#   CLEWDR_ADMIN_TOKENS   — Comma-separated admin tokens (one per instance)
#   CLEWDR_ADMIN_TOKEN    — Single admin token (fallback, applied to all instances)
#   STARAPIHUB_BIN        — Path to starapihub binary (default: /usr/local/bin/starapihub)
#   LOG_DIR               — Log directory (default: /var/log/starapihub)
#   AUDIT_LOG             — Audit log path (default: ~/.starapihub/audit.log)
#   MIN_VALID_COOKIES     — Cookie threshold per instance (default: 2)
#
# Dry-run mode (test without sending webhooks):
#   DRY_RUN=1 /usr/local/bin/starapihub-cron.sh
#
# Test a single alert:
#   source /usr/local/bin/starapihub-cron.sh
#   send_alert WARNING test-alert test "This is a test alert"

set -euo pipefail

STARAPIHUB_BIN="${STARAPIHUB_BIN:-/usr/local/bin/starapihub}"
LOG_DIR="${LOG_DIR:-/var/log/starapihub}"
AUDIT_LOG="${AUDIT_LOG:-${HOME}/.starapihub/audit.log}"
MIN_VALID_COOKIES="${MIN_VALID_COOKIES:-2}"
DRY_RUN="${DRY_RUN:-}"

# send_alert severity signal service message
# Sends a JSON alert to WEBHOOK_URL. In DRY_RUN mode, prints to stderr instead.
send_alert() {
  local severity="$1" signal="$2" service="$3" message="$4"
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  local payload
  payload="{\"severity\":\"${severity}\",\"type\":\"${severity}\",\"signal\":\"${signal}\",\"service\":\"${service}\",\"message\":\"${message}\",\"timestamp\":\"${ts}\"}"

  if [ -n "$DRY_RUN" ]; then
    echo "[DRY_RUN] Would send: $payload" >&2
    return 0
  fi

  if [ -z "${WEBHOOK_URL:-}" ]; then
    echo "[starapihub-cron] WEBHOOK_URL not set, skipping alert: $payload" >&2
    return 0
  fi

  curl -s -X POST "$WEBHOOK_URL" \
    -H "Content-Type: application/json" \
    -d "$payload" >/dev/null 2>&1 || \
    echo "[starapihub-cron] Failed to send alert to $WEBHOOK_URL" >&2
}

# When sourced (not executed), export send_alert and stop.
# This allows: source starapihub-cron.sh && send_alert WARNING test test "msg"
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
  return 0 2>/dev/null || true
fi

# --- Checks (only run when executed, not sourced) ---

# 1. Service health — exit 1 means at least one service unreachable
if ! "$STARAPIHUB_BIN" health --output json >> "$LOG_DIR/health.log" 2>&1; then
  send_alert WARNING service-down health \
    "One or more services unhealthy — run starapihub health to identify which"
fi

# 2. Drift detection — exit 1 means blocking drift found
if ! "$STARAPIHUB_BIN" diff --output json >> "$LOG_DIR/drift.log" 2>&1; then
  send_alert WARNING config-drift drift "Blocking drift detected"
fi

# 3. Sync failure audit — failed > 0 in audit log
if [ -f "$AUDIT_LOG" ]; then
  if jq -ce 'select(.failed > 0)' "$AUDIT_LOG" 2>/dev/null | tail -1 | grep -q .; then
    send_alert WARNING sync-failure sync "Recent sync operation had failures"
  fi
fi

# 4. Cookie depletion — exit 1 means at least one instance below threshold
if [ -n "${CLEWDR_URLS:-}" ]; then
  if ! "$STARAPIHUB_BIN" cookie-status --min-valid "$MIN_VALID_COOKIES" --output json \
       >> "$LOG_DIR/cookies.log" 2>&1; then
    send_alert WARNING cookie-exhaustion cookies \
      "Cookie count below per-instance threshold — run starapihub cookie-status to identify which"
  fi
fi
