#!/usr/bin/env bash
# Aggregated health check — polls all upstream services and returns unified JSON.
#
# Checks: New-API, Bifrost, Postgres (via New-API DSN), Redis (via New-API),
# and optionally ClewdR instances (if --with-clewdr is passed).
#
# Output: JSON with per-service status, overall health, and timestamp.
#
# Usage:
#   ./health-check.sh                    # Core services only
#   ./health-check.sh --with-clewdr      # Include ClewdR instances
#   ./health-check.sh --quiet            # Exit code only (0=healthy, 1=degraded)
#
# Environment:
#   NEWAPI_URL   — New-API URL (default: http://localhost:3000)
#   BIFROST_URL  — Bifrost URL (default: http://localhost:8080)
#   CLEWDR_URLS  — Comma-separated ClewdR URLs (default: http://localhost:18484,http://localhost:18485,http://localhost:18486)
#   CONNECT_TIMEOUT — curl timeout in seconds (default: 5)

set -euo pipefail

NEWAPI_URL="${NEWAPI_URL:-http://localhost:3000}"
BIFROST_URL="${BIFROST_URL:-http://localhost:8080}"
CLEWDR_URLS="${CLEWDR_URLS:-http://localhost:18484,http://localhost:18485,http://localhost:18486}"
CONNECT_TIMEOUT="${CONNECT_TIMEOUT:-5}"

WITH_CLEWDR=false
QUIET=false

for arg in "$@"; do
    case "$arg" in
        --with-clewdr) WITH_CLEWDR=true ;;
        --quiet)       QUIET=true ;;
        -h|--help)
            echo "Usage: $0 [--with-clewdr] [--quiet]"
            exit 0
            ;;
    esac
done

# Check a single HTTP endpoint. Returns "healthy" or "unhealthy".
check_http() {
    local url="$1"
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout "$CONNECT_TIMEOUT" "$url" 2>/dev/null || echo "000")
    if [ "$code" -ge 200 ] && [ "$code" -lt 400 ]; then
        echo "healthy"
    else
        echo "unhealthy"
    fi
}

# Check a TCP port. Returns "healthy" or "unhealthy".
check_tcp() {
    local host="$1" port="$2"
    if bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null; then
        echo "healthy"
    else
        echo "unhealthy"
    fi
}

# Build JSON output
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
OVERALL="healthy"
SERVICES=""

# New-API
newapi_status=$(check_http "$NEWAPI_URL/api/status")
[ "$newapi_status" != "healthy" ] && OVERALL="degraded"
SERVICES="\"new_api\":{\"status\":\"$newapi_status\",\"url\":\"$NEWAPI_URL\"}"

# Bifrost
bifrost_status=$(check_http "$BIFROST_URL/health")
[ "$bifrost_status" != "healthy" ] && OVERALL="degraded"
SERVICES="$SERVICES,\"bifrost\":{\"status\":\"$bifrost_status\",\"url\":\"$BIFROST_URL\"}"

# Postgres — check via Docker container directly
pg_status="unknown"
if docker exec cp-postgres pg_isready -U newapi >/dev/null 2>&1; then
    pg_status="healthy"
else
    pg_status="unhealthy"
    OVERALL="degraded"
fi
SERVICES="$SERVICES,\"postgres\":{\"status\":\"$pg_status\"}"

# Redis — check via Docker container directly
redis_status="unknown"
if docker exec cp-redis redis-cli ping >/dev/null 2>&1; then
    redis_status="healthy"
else
    redis_status="unhealthy"
    OVERALL="degraded"
fi
SERVICES="$SERVICES,\"redis\":{\"status\":\"$redis_status\"}"

# ClewdR instances (optional)
if [ "$WITH_CLEWDR" = true ]; then
    IFS=',' read -ra CLEWDR_ARRAY <<< "$CLEWDR_URLS"
    CLEWDR_JSON=""
    for i in "${!CLEWDR_ARRAY[@]}"; do
        url="${CLEWDR_ARRAY[$i]}"
        instance_name="clewdr_$((i + 1))"
        clewdr_status=$(check_http "$url")
        [ "$clewdr_status" != "healthy" ] && OVERALL="degraded"
        [ -n "$CLEWDR_JSON" ] && CLEWDR_JSON="$CLEWDR_JSON,"
        CLEWDR_JSON="$CLEWDR_JSON\"$instance_name\":{\"status\":\"$clewdr_status\",\"url\":\"$url\"}"
    done
    SERVICES="$SERVICES,$CLEWDR_JSON"
fi

JSON="{\"status\":\"$OVERALL\",\"timestamp\":\"$TIMESTAMP\",\"services\":{$SERVICES}}"

if [ "$QUIET" = true ]; then
    [ "$OVERALL" = "healthy" ] && exit 0 || exit 1
else
    echo "$JSON" | python3 -m json.tool 2>/dev/null || echo "$JSON"
fi

[ "$OVERALL" = "healthy" ] && exit 0 || exit 1
