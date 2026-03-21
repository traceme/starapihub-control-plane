#!/usr/bin/env bash
# ClewdR cookie health monitor — checks all ClewdR instances for cookie status.
#
# Calls GET /api/cookies on each instance (requires admin auth) and reports:
#   - Number of valid / exhausted / invalid cookies per instance
#   - Instances with zero valid cookies (CRITICAL)
#   - Cookies approaching rate limit reset (WARNING)
#   - Overall cookie pool health
#
# Usage:
#   ./check-clewdr-cookies.sh                          # Interactive output
#   ./check-clewdr-cookies.sh --json                   # JSON output (for cron/monitoring)
#   ./check-clewdr-cookies.sh --quiet                  # Exit code only (0=ok, 1=critical, 2=warning)
#
# Environment:
#   CLEWDR_1_URL          — ClewdR instance 1 URL (default: http://localhost:18484)
#   CLEWDR_1_ADMIN_TOKEN  — Admin Bearer token for instance 1
#   CLEWDR_2_URL          — ClewdR instance 2 URL (default: http://localhost:18485)
#   CLEWDR_2_ADMIN_TOKEN  — Admin Bearer token for instance 2
#   CLEWDR_3_URL          — ClewdR instance 3 URL (default: http://localhost:18486)
#   CLEWDR_3_ADMIN_TOKEN  — Admin Bearer token for instance 3
#   CONNECT_TIMEOUT       — curl timeout in seconds (default: 5)
#
# Admin tokens can also be read from .env files if CLEWDR_n_ADMIN_TOKEN is not set.
# The script checks docker logs for auto-generated admin passwords as a fallback.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONNECT_TIMEOUT="${CONNECT_TIMEOUT:-5}"

# Defaults
CLEWDR_1_URL="${CLEWDR_1_URL:-http://localhost:18484}"
CLEWDR_2_URL="${CLEWDR_2_URL:-http://localhost:18485}"
CLEWDR_3_URL="${CLEWDR_3_URL:-http://localhost:18486}"

OUTPUT_MODE="interactive"
for arg in "$@"; do
    case "$arg" in
        --json)   OUTPUT_MODE="json" ;;
        --quiet)  OUTPUT_MODE="quiet" ;;
        -h|--help)
            echo "Usage: $0 [--json] [--quiet]"
            echo ""
            echo "Checks ClewdR cookie health across all instances."
            echo "Set CLEWDR_n_ADMIN_TOKEN env vars for authentication."
            echo "If not set, attempts to read from docker logs."
            exit 0
            ;;
    esac
done

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ] && [ "$OUTPUT_MODE" = "interactive" ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' RED='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

# Try to get admin token from docker logs if not provided
get_admin_token_from_logs() {
    local container="$1"
    # ClewdR logs: "Web Admin Password: <token>"
    docker logs "$container" 2>&1 | grep -oE 'Web Admin Password: [A-Za-z0-9]+' | tail -1 | sed 's/Web Admin Password: //' || echo ""
}

# Check one ClewdR instance
# Returns: JSON object with cookie counts and status
check_instance() {
    local url="$1"
    local token="$2"
    local name="$3"

    if [ -z "$token" ]; then
        echo "{\"name\":\"$name\",\"status\":\"no_auth\",\"error\":\"No admin token available\"}"
        return
    fi

    local response
    response=$(curl -s --connect-timeout "$CONNECT_TIMEOUT" \
        -H "Authorization: Bearer $token" \
        "$url/api/cookies" 2>/dev/null) || true

    if [ -z "$response" ]; then
        echo "{\"name\":\"$name\",\"status\":\"unreachable\",\"error\":\"Connection failed\"}"
        return
    fi

    # Check for auth error
    if echo "$response" | grep -qi "unauthorized\|forbidden\|401\|403" 2>/dev/null; then
        echo "{\"name\":\"$name\",\"status\":\"auth_failed\",\"error\":\"Admin token rejected\"}"
        return
    fi

    # Parse cookie counts using python3 (available on most systems)
    local parsed
    parsed=$(python3 -c "
import json, sys
try:
    d = json.loads('''$response''')
    valid = len(d.get('valid', []))
    exhausted = len(d.get('exhausted', []))
    invalid = len(d.get('invalid', []))
    total = valid + exhausted + invalid

    # Check utilization on valid cookies
    high_util = 0
    for c in d.get('valid', []):
        session_util = c.get('session_utilization', 0) or 0
        weekly_util = c.get('seven_day_utilization', 0) or 0
        if session_util > 80 or weekly_util > 80:
            high_util += 1

    # Determine status
    if total == 0:
        status = 'no_cookies'
    elif valid == 0:
        status = 'critical'
    elif valid <= exhausted + invalid:
        status = 'warning'
    elif high_util > 0:
        status = 'warning'
    else:
        status = 'healthy'

    print(json.dumps({
        'name': '$name',
        'status': status,
        'valid': valid,
        'exhausted': exhausted,
        'invalid': invalid,
        'total': total,
        'high_utilization': high_util
    }))
except Exception as e:
    print(json.dumps({'name': '$name', 'status': 'parse_error', 'error': str(e)}))
" 2>/dev/null) || echo "{\"name\":\"$name\",\"status\":\"parse_error\",\"error\":\"Failed to parse response\"}"

    echo "$parsed"
}

# ── Resolve admin tokens ─────────────────────────────────
CLEWDR_1_ADMIN_TOKEN="${CLEWDR_1_ADMIN_TOKEN:-}"
CLEWDR_2_ADMIN_TOKEN="${CLEWDR_2_ADMIN_TOKEN:-}"
CLEWDR_3_ADMIN_TOKEN="${CLEWDR_3_ADMIN_TOKEN:-}"

# Fallback: try docker logs
[ -z "$CLEWDR_1_ADMIN_TOKEN" ] && CLEWDR_1_ADMIN_TOKEN=$(get_admin_token_from_logs "cp-clewdr-1")
[ -z "$CLEWDR_2_ADMIN_TOKEN" ] && CLEWDR_2_ADMIN_TOKEN=$(get_admin_token_from_logs "cp-clewdr-2")
[ -z "$CLEWDR_3_ADMIN_TOKEN" ] && CLEWDR_3_ADMIN_TOKEN=$(get_admin_token_from_logs "cp-clewdr-3")

# ── Check all instances ───────────────────────────────────
RESULT_1=$(check_instance "$CLEWDR_1_URL" "$CLEWDR_1_ADMIN_TOKEN" "clewdr-1")
RESULT_2=$(check_instance "$CLEWDR_2_URL" "$CLEWDR_2_ADMIN_TOKEN" "clewdr-2")
RESULT_3=$(check_instance "$CLEWDR_3_URL" "$CLEWDR_3_ADMIN_TOKEN" "clewdr-3")

# ── Determine overall status ─────────────────────────────
OVERALL="healthy"
EXIT_CODE=0

for result in "$RESULT_1" "$RESULT_2" "$RESULT_3"; do
    status=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "unknown")
    case "$status" in
        critical|no_cookies)
            OVERALL="critical"
            EXIT_CODE=1
            ;;
        warning|auth_failed|unreachable)
            [ "$OVERALL" != "critical" ] && OVERALL="warning"
            [ "$EXIT_CODE" -eq 0 ] && EXIT_CODE=2
            ;;
        no_auth)
            [ "$OVERALL" = "healthy" ] && OVERALL="warning"
            [ "$EXIT_CODE" -eq 0 ] && EXIT_CODE=2
            ;;
    esac
done

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# ── Output ────────────────────────────────────────────────
case "$OUTPUT_MODE" in
    json)
        echo "{\"status\":\"$OVERALL\",\"timestamp\":\"$TIMESTAMP\",\"instances\":[$RESULT_1,$RESULT_2,$RESULT_3]}" | python3 -m json.tool 2>/dev/null || echo "{\"status\":\"$OVERALL\",\"timestamp\":\"$TIMESTAMP\",\"instances\":[$RESULT_1,$RESULT_2,$RESULT_3]}"
        ;;
    quiet)
        ;;
    interactive)
        printf "${BOLD}========================================${RESET}\n"
        printf "${BOLD}  ClewdR Cookie Health Report${RESET}\n"
        printf "${BOLD}========================================${RESET}\n"
        printf "  Timestamp: %s\n\n" "$TIMESTAMP"

        for result in "$RESULT_1" "$RESULT_2" "$RESULT_3"; do
            name=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('name','?'))" 2>/dev/null)
            status=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('status','?'))" 2>/dev/null)

            case "$status" in
                healthy)    color="$GREEN" ;;
                warning)    color="$YELLOW" ;;
                critical|no_cookies) color="$RED" ;;
                *)          color="$YELLOW" ;;
            esac

            printf "  ${BOLD}%s${RESET}: ${color}%s${RESET}" "$name" "$status"

            # Print cookie counts if available
            valid=$(echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('valid',''))" 2>/dev/null || echo "")
            if [ -n "$valid" ] && [ "$valid" != "" ]; then
                exhausted=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('exhausted',0))" 2>/dev/null)
                invalid=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('invalid',0))" 2>/dev/null)
                high_util=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('high_utilization',0))" 2>/dev/null)
                printf " — ${GREEN}%s valid${RESET}" "$valid"
                [ "$exhausted" != "0" ] && printf ", ${YELLOW}%s exhausted${RESET}" "$exhausted"
                [ "$invalid" != "0" ] && printf ", ${RED}%s invalid${RESET}" "$invalid"
                [ "$high_util" != "0" ] && printf " (${YELLOW}%s high utilization${RESET})" "$high_util"
            else
                error=$(echo "$result" | python3 -c "import json,sys; print(json.load(sys.stdin).get('error',''))" 2>/dev/null || echo "")
                [ -n "$error" ] && printf " — %s" "$error"
            fi
            printf "\n"
        done

        printf "\n  ${BOLD}Overall${RESET}: "
        case "$OVERALL" in
            healthy)  printf "${GREEN}HEALTHY${RESET}\n" ;;
            warning)  printf "${YELLOW}WARNING${RESET}\n" ;;
            critical) printf "${RED}CRITICAL${RESET} — one or more instances have zero valid cookies!\n" ;;
        esac
        printf "${BOLD}========================================${RESET}\n"

        if [ "$OVERALL" = "critical" ]; then
            printf "\n${RED}ACTION REQUIRED:${RESET} Add fresh cookies to instances with zero valid cookies.\n"
            printf "Access the admin UI for each instance and paste a Claude.ai session cookie.\n"
        elif [ "$OVERALL" = "warning" ]; then
            printf "\n${YELLOW}NOTE:${RESET} Some instances have degraded cookie pools.\n"
            printf "Consider adding fresh cookies or waiting for rate limit resets.\n"
        fi
        ;;
esac

exit $EXIT_CODE
