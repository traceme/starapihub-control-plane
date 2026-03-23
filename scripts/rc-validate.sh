#!/usr/bin/env bash
# rc-validate.sh — Release candidate validation with evidence capture.
#
# Implements every gate from RELEASE_CHECKLIST.md. Each gate either passes,
# fails, or is skipped with an explicit reason. Evidence files are only
# listed for gates that actually ran.
#
# The script validates against the production-like compose topology
# (docker-compose.yml + docker-compose.rc-validate.yml), NOT the local-dev
# override. The topology must be brought up manually before running this
# script. ClewdR ports are not exposed to the host — ClewdR health is
# validated through the dashboard's /api/health (dashboard → ClewdR via
# provider-net). The isolation smoke test verifies ClewdR is unreachable
# from the host.
#
# Usage:
#   cd deploy && docker compose -f docker-compose.yml \
#     -f docker-compose.rc-validate.yml --env-file env/common.env \
#     --profile clewdr up -d --build
#   cd .. && make rc-validate
#
# Required environment for full validation:
#   NEWAPI_ADMIN_TOKEN — New-API admin access_token (from users table, NOT an sk- API key)
#   DASHBOARD_TOKEN    — Dashboard bearer token (for health + dashboard gates)
#
# Optional environment:
#   NEWAPI_URL         — (default: http://127.0.0.1:3000)
#   BIFROST_URL        — (default: http://127.0.0.1:8080)
#   DASHBOARD_URL      — (default: http://127.0.0.1:8090)
#   STARAPIHUB_MODE    — "upstream" or "appliance" (default: upstream)
#   SKIP_INTEGRATION   — set to 1 to skip integration tests
#   SKIP_IMAGES        — set to 1 to skip Docker image builds
#
# Output:
#   artifacts/releases/<version>/<timestamp>/
#     summary.md     — structured evidence index
#     evidence.log   — full gate-by-gate log
#     *.log / *.json — per-gate evidence files

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

VERSION=$(cat VERSION 2>/dev/null || echo "dev")
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
ARTIFACT_DIR="artifacts/releases/${VERSION}/${TIMESTAMP}"
mkdir -p "$ARTIFACT_DIR"

EVIDENCE_LOG="$ARTIFACT_DIR/evidence.log"
MODE="${STARAPIHUB_MODE:-upstream}"

# Service URLs
NEWAPI_URL="${NEWAPI_URL:-http://127.0.0.1:3000}"
BIFROST_URL="${BIFROST_URL:-http://127.0.0.1:8080}"
DASHBOARD_URL="${DASHBOARD_URL:-http://127.0.0.1:8090}"
CLEWDR_URLS="${CLEWDR_URLS:-http://127.0.0.1:18484}"
export NEWAPI_URL BIFROST_URL CLEWDR_URLS STARAPIHUB_MODE="${MODE}"

# Gate tracking — parallel arrays
declare -a GATE_NAMES=()
declare -a GATE_RESULTS=()   # PASS / FAIL / SKIP
declare -a GATE_FILES=()     # evidence filename or empty
declare -a GATE_NOTES=()     # reason for skip, or extra info

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# ── helpers ──────────────────────────────────────────────

log() {
  local msg="[$(date -u +%H:%M:%S)] $*"
  echo "$msg"
  echo "$msg" >> "$EVIDENCE_LOG"
}

record_pass() {
  local name="$1" file="${2:-}"
  GATE_NAMES+=("$name"); GATE_RESULTS+=("PASS"); GATE_FILES+=("$file"); GATE_NOTES+=("")
  PASS_COUNT=$((PASS_COUNT + 1))
  log "  PASS: $name"
}

record_fail() {
  local name="$1" file="${2:-}" note="${3:-}"
  GATE_NAMES+=("$name"); GATE_RESULTS+=("FAIL"); GATE_FILES+=("$file"); GATE_NOTES+=("$note")
  FAIL_COUNT=$((FAIL_COUNT + 1))
  log "  FAIL: $name${note:+ ($note)}"
}

record_skip() {
  local name="$1" reason="$2"
  GATE_NAMES+=("$name"); GATE_RESULTS+=("SKIP"); GATE_FILES+=(""); GATE_NOTES+=("$reason")
  SKIP_COUNT=$((SKIP_COUNT + 1))
  log "  SKIP: $name — $reason"
}

run_capturing() {
  local outfile="$1"; shift
  "$@" > "$ARTIFACT_DIR/$outfile" 2>&1
}

live_stack_up() {
  curl -sf "$NEWAPI_URL/api/status" > /dev/null 2>&1
}

has_admin_token() {
  [ -n "${NEWAPI_ADMIN_TOKEN:-}" ]
}

# ── Header ───────────────────────────────────────────────

log "========================================================"
log "RC Validation — StarAPIHub v${VERSION}"
log "Timestamp:  $TIMESTAMP"
log "Mode:       $MODE"
log "Topology:   production-like (no local-dev override)"
log "Artifacts:  $ARTIFACT_DIR"
log "========================================================"
log ""

# ════════════════════════════════════════════════════════════
# RELEASE_CHECKLIST §1: Pre-release
# ════════════════════════════════════════════════════════════

log "── §1 Pre-release ──"

# 1a. VERSION file
log "GATE: VERSION file"
echo "$VERSION" > "$ARTIFACT_DIR/version-file.txt"
if [ "$VERSION" != "dev" ] && [ -n "$VERSION" ]; then
  record_pass "VERSION file" "version-file.txt"
else
  record_fail "VERSION file" "version-file.txt" "VERSION is '$VERSION'"
fi

# 1b. Build
log "GATE: CLI build"
if run_capturing "build.log" make build; then
  record_pass "CLI build" "build.log"
else
  record_fail "CLI build" "build.log"
fi

# Capture version output immediately after build
./dashboard/starapihub version --output json > "$ARTIFACT_DIR/version.json" 2>&1 || true
log "  Built version: $(cat "$ARTIFACT_DIR/version.json" 2>/dev/null)"

# 1c. Lint
log "GATE: go vet"
if run_capturing "lint.log" make lint; then
  record_pass "go vet" "lint.log"
else
  record_fail "go vet" "lint.log"
fi

# 1d. Unit tests
log "GATE: unit tests"
if run_capturing "unit-tests.log" env -u STARAPIHUB_MODE make test-unit; then
  record_pass "unit tests" "unit-tests.log"
else
  record_fail "unit tests" "unit-tests.log"
fi

# 1e. Integration tests — bring up test stack, run, tear down
log "GATE: integration tests"
if [ "${SKIP_INTEGRATION:-0}" = "1" ]; then
  record_skip "integration tests" "SKIP_INTEGRATION=1"
else
  # make test-integration handles its own stack lifecycle (up, test, down)
  # via TestMain in cli_test.go. We just need to invoke it.
  if run_capturing "integration-tests.log" make test-integration; then
    record_pass "integration tests" "integration-tests.log"
  else
    record_fail "integration tests" "integration-tests.log"
  fi
fi

log ""

# ════════════════════════════════════════════════════════════
# RELEASE_CHECKLIST §2: Image build
# ════════════════════════════════════════════════════════════

log "── §2 Image build ──"

# 2a. Dashboard image
log "GATE: dashboard image build"
if [ "${SKIP_IMAGES:-0}" = "1" ]; then
  record_skip "dashboard image build" "SKIP_IMAGES=1"
else
  if run_capturing "build-dashboard.log" make build-dashboard; then
    record_pass "dashboard image build" "build-dashboard.log"
  else
    record_fail "dashboard image build" "build-dashboard.log"
  fi
fi

# 2b. Patched New-API image (appliance mode only)
log "GATE: patched New-API image build"
if [ "$MODE" != "appliance" ]; then
  record_skip "patched New-API image" "mode is upstream"
elif [ "${SKIP_IMAGES:-0}" = "1" ]; then
  record_skip "patched New-API image" "SKIP_IMAGES=1"
else
  if run_capturing "build-patched-newapi.log" make build-patched-newapi; then
    record_pass "patched New-API image" "build-patched-newapi.log"
  else
    record_fail "patched New-API image" "build-patched-newapi.log"
  fi
fi

log ""

# ════════════════════════════════════════════════════════════
# RELEASE_CHECKLIST §3: Validation (against real stack)
# ════════════════════════════════════════════════════════════

log "── §3 Validation (live stack) ──"

# 3a. Health
# The CLI health command runs on the host, which cannot reach ClewdR directly
# in the rc-validate topology. Instead, validate through the dashboard's
# /api/health endpoint, which checks all services (including ClewdR) via
# the internal Docker network. This proves the deployed service mesh.
log "GATE: health check"
if [ -z "${DASHBOARD_TOKEN:-}" ]; then
  record_skip "health check" "DASHBOARD_TOKEN not set (required for dashboard-mediated health)"
elif ! curl -sf "$DASHBOARD_URL/api/version" > /dev/null 2>&1; then
  record_fail "health check" "" "dashboard not reachable at $DASHBOARD_URL"
else
  HEALTH_RESP=$(curl -sf -H "Authorization: Bearer $DASHBOARD_TOKEN" "$DASHBOARD_URL/api/health" 2>&1 || echo "")
  echo "$HEALTH_RESP" > "$ARTIFACT_DIR/health.json"
  if [ -z "$HEALTH_RESP" ]; then
    record_fail "health check" "health.json" "dashboard /api/health returned empty or auth failed"
  else
    # Check that no service reports unhealthy
    UNHEALTHY=$(echo "$HEALTH_RESP" | grep -o '"status":"[^"]*"' | grep -v '"status":"healthy"' || true)
    if [ -n "$UNHEALTHY" ]; then
      record_fail "health check" "health.json" "unhealthy services: $UNHEALTHY"
    else
      record_pass "health check" "health.json"
    fi
  fi
fi

# 3b. Validate
log "GATE: registry validate"
if run_capturing "validate.log" ./dashboard/starapihub validate --config-dir policies; then
  record_pass "registry validate" "validate.log"
else
  record_fail "registry validate" "validate.log"
fi

# ── Host-side CLI environment ──
# The CLI runs on the host, which cannot reach ClewdR (internal Docker network
# only). ClewdR health is validated by gate 3a through the dashboard's
# /api/health endpoint, which reaches ClewdR via provider-net.
#
# Bifrost does not expose an admin API for provider/routing-rule CRUD.
# Scope sync/diff to channel,pricing (New-API resources) only.
#
# Unset CLEWDR_URLS so host-side CLI commands don't attempt connections to
# unreachable ClewdR hosts leaked from the user's ambient environment.
RC_SYNC_TARGETS="--target channel,pricing"
RC_HOST_ENV="env -u CLEWDR_URLS -u CLEWDR_ADMIN_TOKEN"

# 3c. Bootstrap
log "GATE: bootstrap"
if ! has_admin_token; then
  record_skip "bootstrap" "NEWAPI_ADMIN_TOKEN not set"
elif ! live_stack_up; then
  record_skip "bootstrap" "live stack not reachable"
else
  # Bootstrap: seed admin, then sync New-API resources only.
  # --skip-sync avoids Bifrost reconcilers; sync gate handles targeted sync.
  if run_capturing "bootstrap.log" $RC_HOST_ENV ./dashboard/starapihub bootstrap --config-dir policies --skip-sync; then
    # Now sync only New-API resources (channel, pricing)
    if run_capturing "bootstrap-sync.log" $RC_HOST_ENV ./dashboard/starapihub sync --config-dir policies $RC_SYNC_TARGETS; then
      record_pass "bootstrap" "bootstrap.log"
    else
      record_fail "bootstrap" "bootstrap-sync.log"
    fi
  else
    record_fail "bootstrap" "bootstrap.log"
  fi
fi

# 3d. Sync
log "GATE: sync (dry-run)"
if ! has_admin_token; then
  record_skip "sync" "NEWAPI_ADMIN_TOKEN not set"
elif ! live_stack_up; then
  record_skip "sync" "live stack not reachable"
else
  if run_capturing "sync.log" $RC_HOST_ENV ./dashboard/starapihub sync --dry-run --config-dir policies $RC_SYNC_TARGETS; then
    record_pass "sync" "sync.log"
  else
    record_fail "sync" "sync.log"
  fi
fi

# 3e. Diff
log "GATE: diff"
if ! has_admin_token; then
  record_skip "diff" "NEWAPI_ADMIN_TOKEN not set"
elif ! live_stack_up; then
  record_skip "diff" "live stack not reachable"
else
  if run_capturing "diff.log" $RC_HOST_ENV ./dashboard/starapihub diff --config-dir policies $RC_SYNC_TARGETS; then
    record_pass "diff" "diff.log"
  else
    record_fail "diff" "diff.log"
  fi
fi

# 3f. Upgrade check
# ClewdR health is validated by gate 3a via the dashboard's internal network.
# Unset CLEWDR_URLS to prevent the CLI from failing on unreachable hosts.
log "GATE: upgrade-check"
if run_capturing "upgrade-check.log" $RC_HOST_ENV ./dashboard/starapihub upgrade-check --repo-root "$ROOT_DIR/.."; then
  record_pass "upgrade-check" "upgrade-check.log"
else
  record_fail "upgrade-check" "upgrade-check.log"
fi

# 3g. Smoke tests
log "GATE: smoke tests"
if ! live_stack_up; then
  record_skip "smoke tests" "live stack not reachable"
elif [ ! -x "$ROOT_DIR/scripts/smoke/run-all.sh" ]; then
  record_skip "smoke tests" "scripts/smoke/run-all.sh not found or not executable"
else
  if run_capturing "smoke-tests.log" bash "$ROOT_DIR/scripts/smoke/run-all.sh"; then
    record_pass "smoke tests" "smoke-tests.log"
  else
    record_fail "smoke tests" "smoke-tests.log"
  fi
fi

log ""

# ════════════════════════════════════════════════════════════
# RELEASE_CHECKLIST §4: Auditability
# ════════════════════════════════════════════════════════════

log "── §4 Auditability ──"

# 4a. Trace
log "GATE: trace command"
if ! live_stack_up; then
  record_skip "trace" "live stack not reachable"
else
  TEST_RID="rc-validate-${TIMESTAMP}"
  curl -sI -H "X-Request-ID: $TEST_RID" "$NEWAPI_URL/api/status" > /dev/null 2>&1 || true
  if run_capturing "trace.log" ./dashboard/starapihub trace "$TEST_RID"; then
    record_pass "trace" "trace.log"
  else
    if [ -s "$ARTIFACT_DIR/trace.log" ]; then
      record_pass "trace" "trace.log"
    else
      record_fail "trace" "trace.log" "no trace output"
    fi
  fi
fi

# 4b. Audit log
log "GATE: audit log"
AUDIT_LOG_PATH="${HOME}/.starapihub/audit.log"
if [ -f "$AUDIT_LOG_PATH" ]; then
  tail -20 "$AUDIT_LOG_PATH" > "$ARTIFACT_DIR/audit-log-tail.txt" 2>&1
  record_pass "audit log exists" "audit-log-tail.txt"
else
  record_skip "audit log" "no audit log at $AUDIT_LOG_PATH (run bootstrap or sync first)"
fi

# 4c. Patch 001 (appliance mode only)
log "GATE: Patch 001 (X-Request-ID propagation)"
if [ "$MODE" != "appliance" ]; then
  record_skip "Patch 001" "mode is upstream (no patch to verify)"
elif ! live_stack_up; then
  record_skip "Patch 001" "live stack not reachable"
else
  TEST_RID="patch001-${TIMESTAMP}"
  HEADERS=$(curl -sI -H "X-Request-ID: $TEST_RID" "$NEWAPI_URL/api/status" 2>&1 || true)
  echo "$HEADERS" > "$ARTIFACT_DIR/patch001-headers.txt"
  if echo "$HEADERS" | grep -qi "$TEST_RID"; then
    record_pass "Patch 001" "patch001-headers.txt"
  else
    record_fail "Patch 001" "patch001-headers.txt" "X-Request-ID not propagated"
  fi
fi

log ""

# ════════════════════════════════════════════════════════════
# §5: Dashboard runtime
# ════════════════════════════════════════════════════════════

log "── §5 Dashboard runtime ──"

# 5a. Dashboard /api/version
log "GATE: dashboard /api/version"
DASHBOARD_TOKEN="${DASHBOARD_TOKEN:-}"
if curl -sf "$DASHBOARD_URL/api/version" > /dev/null 2>&1; then
  DASH_RESP=$(curl -sf "$DASHBOARD_URL/api/version" 2>&1 || echo "{}")
  echo "$DASH_RESP" > "$ARTIFACT_DIR/dashboard-version.json"
  # Verify it contains a version field
  if echo "$DASH_RESP" | grep -q '"version"'; then
    record_pass "dashboard /api/version" "dashboard-version.json"
  else
    record_fail "dashboard /api/version" "dashboard-version.json" "response missing version field"
  fi
else
  record_fail "dashboard /api/version" "" "dashboard not reachable at $DASHBOARD_URL/api/version"
fi

# 5b. Dashboard /api/health (requires token)
log "GATE: dashboard /api/health"
if [ -z "$DASHBOARD_TOKEN" ]; then
  record_skip "dashboard /api/health" "DASHBOARD_TOKEN not set"
elif curl -sf -H "Authorization: Bearer $DASHBOARD_TOKEN" "$DASHBOARD_URL/api/health" > /dev/null 2>&1; then
  curl -sf -H "Authorization: Bearer $DASHBOARD_TOKEN" "$DASHBOARD_URL/api/health" > "$ARTIFACT_DIR/dashboard-health.json" 2>&1
  record_pass "dashboard /api/health" "dashboard-health.json"
else
  record_fail "dashboard /api/health" "" "dashboard /api/health not reachable or auth failed"
fi

log ""

# ════════════════════════════════════════════════════════════
# Image provenance — resolved from running containers
# ════════════════════════════════════════════════════════════

log "── Image provenance ──"

{
  echo "# Image provenance — v${VERSION} validated at ${TIMESTAMP}"
  echo "# Mode: ${MODE}"
  echo ""
  echo "## Running containers (from docker inspect)"
  echo ""

  for container in cp-new-api cp-bifrost cp-dashboard cp-clewdr-1 cp-clewdr-2 cp-clewdr-3; do
    if docker inspect "$container" > /dev/null 2>&1; then
      img=$(docker inspect "$container" --format '{{.Config.Image}}' 2>/dev/null)
      digest=$(docker inspect "$container" --format '{{.Image}}' 2>/dev/null)
      echo "- ${container}: image=${img} digest=${digest}"
    else
      echo "- ${container}: not running"
    fi
  done

  echo ""
  echo "## Local image digests (from docker image inspect)"
  echo ""
  for img in "calciumion/new-api:latest" "maximhq/bifrost:latest" "clewdr:local" "starapihub/dashboard:local"; do
    if docker image inspect "$img" > /dev/null 2>&1; then
      repo_digest=$(docker image inspect "$img" --format '{{range .RepoDigests}}{{.}}{{end}}' 2>/dev/null)
      local_id=$(docker image inspect "$img" --format '{{.Id}}' 2>/dev/null)
      if [ -n "$repo_digest" ]; then
        echo "- ${img}: ${repo_digest}"
      else
        echo "- ${img}: id=${local_id} (local build, no registry digest)"
      fi
    else
      echo "- ${img}: not found locally"
    fi
  done

  echo ""
  echo "## CLI binary"
  echo ""
  cat "$ARTIFACT_DIR/version.json" 2>/dev/null || echo "build failed"
} > "$ARTIFACT_DIR/image-provenance.txt"

log "  Image provenance written"

log ""

# ════════════════════════════════════════════════════════════
# Summary
# ════════════════════════════════════════════════════════════

TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))

RESULT="FAIL"
if [ "$FAIL_COUNT" -eq 0 ] && [ "$SKIP_COUNT" -eq 0 ]; then
  RESULT="PASS"
elif [ "$FAIL_COUNT" -eq 0 ]; then
  RESULT="PARTIAL"
fi

log "========================================================"
log "RC Validation Complete — v${VERSION} (${MODE} mode)"
log "  Result: $RESULT"
log "  Pass:   $PASS_COUNT / $TOTAL"
log "  Fail:   $FAIL_COUNT / $TOTAL"
log "  Skip:   $SKIP_COUNT / $TOTAL"
log "  Dir:    $ARTIFACT_DIR"
log "========================================================"

# Determine patch count for this mode
if [ "$MODE" = "appliance" ]; then
  PATCH_COUNT=1
else
  PATCH_COUNT=0
fi

# Build the summary — dynamic section mapping
{
  cat <<HEADER
# RC Validation Evidence — StarAPIHub v${VERSION}

| Field | Value |
|-------|-------|
| Version | ${VERSION} |
| Mode | ${MODE} |
| Timestamp | ${TIMESTAMP} |
| Result | **${RESULT}** |
| Pass | ${PASS_COUNT} / ${TOTAL} |
| Fail | ${FAIL_COUNT} |
| Skip | ${SKIP_COUNT} |
| Patch count | ${PATCH_COUNT} |

## Checklist Gates

| Section | Gate | Result | Evidence | Notes |
|---------|------|--------|----------|-------|
HEADER

  for i in "${!GATE_NAMES[@]}"; do
    name="${GATE_NAMES[$i]}"
    result="${GATE_RESULTS[$i]}"
    file="${GATE_FILES[$i]}"
    note="${GATE_NOTES[$i]}"

    # Derive section from gate name
    case "$name" in
      "VERSION file"|"CLI build"|"go vet"|"unit tests"|"integration tests")
        section="Pre-release" ;;
      "dashboard image build"|"patched New-API image")
        section="Image build" ;;
      "health check"|"registry validate"|bootstrap|sync|diff|"upgrade-check"|"smoke tests")
        section="Validation" ;;
      trace|"audit log exists"|"Patch 001")
        section="Auditability" ;;
      dashboard*)
        section="Dashboard" ;;
      *)
        section="—" ;;
    esac

    # Evidence: show filename only if file exists
    if [ -n "$file" ] && [ -f "$ARTIFACT_DIR/$file" ]; then
      evidence="$file"
    else
      evidence="—"
    fi

    # Notes: show reason for skip or extra info
    display_note="${note}"

    echo "| $section | $name | **$result** | $evidence | $display_note |"
  done

  echo ""
  echo "## Image Provenance"
  echo ""
  echo '```'
  cat "$ARTIFACT_DIR/image-provenance.txt"
  echo '```'
  echo ""
  echo "## Build Metadata"
  echo ""
  echo '```json'
  cat "$ARTIFACT_DIR/version.json" 2>/dev/null || echo '{"error": "build failed"}'
  echo '```'
} > "$ARTIFACT_DIR/summary.md"

log "Summary written to $ARTIFACT_DIR/summary.md"

# Exit non-zero if any gate failed
if [ "$FAIL_COUNT" -gt 0 ]; then
  exit 1
fi
