#!/usr/bin/env bash
# StarAPIHub — First-run setup script
#
# Generates .env files from .env.example templates with random passwords,
# validates prerequisites, and optionally starts the stack.
#
# Usage:
#   ./setup.sh                  # Interactive setup
#   ./setup.sh --self-test      # Validate environment without generating files
#   ./setup.sh --with-clewdr    # Include ClewdR services (requires cookie setup)
#   ./setup.sh --no-start       # Generate .env files only, don't start the stack

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/../deploy" && pwd)"
ENV_DIR="$DEPLOY_DIR/env"

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' RED='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

info()  { printf "${CYAN}[INFO]${RESET} %s\n" "$1"; }
ok()    { printf "${GREEN}  [OK]${RESET} %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${RESET} %s\n" "$1"; }
err()   { printf "${RED}[FAIL]${RESET} %s\n" "$1"; }

# ── Parse arguments ───────────────────────────────────────
SELF_TEST=false
WITH_CLEWDR=false
NO_START=false
for arg in "$@"; do
    case "$arg" in
        --self-test)   SELF_TEST=true ;;
        --with-clewdr) WITH_CLEWDR=true ;;
        --no-start)    NO_START=true ;;
        -h|--help)
            echo "Usage: $0 [--self-test] [--with-clewdr] [--no-start]"
            echo ""
            echo "  --self-test    Validate environment without generating files"
            echo "  --with-clewdr  Include ClewdR services (requires cookie setup later)"
            echo "  --no-start     Generate .env files only, don't start the stack"
            exit 0
            ;;
        *) echo "Unknown argument: $arg"; exit 1 ;;
    esac
done

# ── Random password generator ─────────────────────────────
# Uses openssl if available, falls back to /dev/urandom
generate_password() {
    local length="${1:-32}"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "$((length / 2))"
    elif [ -r /dev/urandom ]; then
        head -c "$((length / 2))" /dev/urandom | od -An -tx1 | tr -d ' \n' | head -c "$length"
    else
        err "Neither openssl nor /dev/urandom available. Cannot generate secure passwords."
        exit 1
    fi
}

# ── Prerequisite checks ──────────────────────────────────
check_prerequisites() {
    local failed=0

    printf "\n${BOLD}Checking prerequisites...${RESET}\n\n"

    # Docker
    if command -v docker >/dev/null 2>&1; then
        if docker info >/dev/null 2>&1; then
            ok "Docker is installed and running"
        else
            err "Docker is installed but not running. Start Docker Desktop or the Docker daemon."
            failed=1
        fi
    else
        err "Docker is not installed. Install from https://docs.docker.com/get-docker/"
        failed=1
    fi

    # Docker Compose v2
    if docker compose version >/dev/null 2>&1; then
        local compose_ver
        compose_ver=$(docker compose version --short 2>/dev/null || echo "unknown")
        ok "Docker Compose v2 available ($compose_ver)"
    else
        err "Docker Compose v2 not available. Install the compose plugin: https://docs.docker.com/compose/install/"
        failed=1
    fi

    # Port checks
    for port in 80 3000 8080; do
        if lsof -i ":$port" -sTCP:LISTEN >/dev/null 2>&1; then
            local proc
            proc=$(lsof -i ":$port" -sTCP:LISTEN -t 2>/dev/null | head -1)
            local proc_name
            proc_name=$(ps -p "$proc" -o comm= 2>/dev/null || echo "unknown")
            warn "Port $port is in use by $proc_name (PID $proc)"
            if [ "$port" = "80" ]; then
                warn "  Nginx needs port 80. Stop the conflicting process or change PUBLIC_HTTP_PORT in common.env"
            fi
        else
            ok "Port $port is available"
        fi
    done

    # Disk space (rough check — need ~2GB for images)
    local avail_gb
    avail_gb=$(df -g "$DEPLOY_DIR" 2>/dev/null | awk 'NR==2{print $4}' || echo "0")
    if [ "$avail_gb" -lt 4 ] 2>/dev/null; then
        warn "Less than 4 GB disk space available ($avail_gb GB). The stack needs ~2-3 GB for Docker images."
    else
        ok "Disk space: ${avail_gb} GB available"
    fi

    return $failed
}

# ── Self-test mode ────────────────────────────────────────
if [ "$SELF_TEST" = true ]; then
    printf "${BOLD}========================================${RESET}\n"
    printf "${BOLD}  StarAPIHub Self-Test${RESET}\n"
    printf "${BOLD}========================================${RESET}\n"

    ERRORS=0

    # Prerequisites
    check_prerequisites || ERRORS=$((ERRORS + 1))

    # Check .env files
    printf "\n${BOLD}Checking .env files...${RESET}\n\n"
    for example in "$ENV_DIR"/*.env.example; do
        base=$(basename "$example" .env.example)
        envfile="$ENV_DIR/${base}.env"
        if [ -f "$envfile" ]; then
            # Verify no CHANGE_ME placeholders remain
            if grep -q 'CHANGE_ME' "$envfile" 2>/dev/null; then
                err "$base.env still contains CHANGE_ME placeholders"
                ERRORS=$((ERRORS + 1))
            else
                ok "$base.env exists and has no placeholders"
            fi
            # Verify non-empty
            if [ ! -s "$envfile" ]; then
                err "$base.env is empty"
                ERRORS=$((ERRORS + 1))
            fi
        else
            warn "$base.env does not exist (run setup.sh to generate)"
        fi
    done

    # Check password consistency between common.env and new-api.env
    if [ -f "$ENV_DIR/common.env" ] && [ -f "$ENV_DIR/new-api.env" ]; then
        pg_pass=$(grep '^POSTGRES_PASSWORD=' "$ENV_DIR/common.env" 2>/dev/null | cut -d= -f2-)
        dsn_pass=$(grep '^SQL_DSN=' "$ENV_DIR/new-api.env" 2>/dev/null | sed 's|.*://[^:]*:\([^@]*\)@.*|\1|')
        if [ -n "$pg_pass" ] && [ -n "$dsn_pass" ] && [ "$pg_pass" = "$dsn_pass" ]; then
            ok "Postgres password matches between common.env and new-api.env"
        elif [ -n "$pg_pass" ] && [ -n "$dsn_pass" ]; then
            err "Postgres password MISMATCH between common.env and new-api.env"
            ERRORS=$((ERRORS + 1))
        fi

        redis_pass=$(grep '^REDIS_PASSWORD=' "$ENV_DIR/common.env" 2>/dev/null | cut -d= -f2-)
        redis_conn_pass=$(grep '^REDIS_CONN_STRING=' "$ENV_DIR/new-api.env" 2>/dev/null | sed 's|.*://:\([^@]*\)@.*|\1|')
        if [ -n "$redis_pass" ] && [ -n "$redis_conn_pass" ] && [ "$redis_pass" = "$redis_conn_pass" ]; then
            ok "Redis password matches between common.env and new-api.env"
        elif [ -n "$redis_pass" ] && [ -n "$redis_conn_pass" ]; then
            err "Redis password MISMATCH between common.env and new-api.env"
            ERRORS=$((ERRORS + 1))
        fi
    fi

    printf "\n${BOLD}========================================${RESET}\n"
    if [ "$ERRORS" -gt 0 ]; then
        printf "${RED}Self-test found $ERRORS issue(s).${RESET}\n"
        exit 1
    else
        printf "${GREEN}All self-tests passed.${RESET}\n"
        exit 0
    fi
fi

# ── Main setup flow ───────────────────────────────────────
printf "${BOLD}========================================${RESET}\n"
printf "${BOLD}  StarAPIHub Setup${RESET}\n"
printf "${BOLD}========================================${RESET}\n"

# Check prerequisites first
if ! check_prerequisites; then
    err "Fix the issues above before continuing."
    exit 1
fi

# Check if .env files already exist
EXISTING_ENV=()
for example in "$ENV_DIR"/*.env.example; do
    base=$(basename "$example" .env.example)
    envfile="$ENV_DIR/${base}.env"
    if [ -f "$envfile" ]; then
        EXISTING_ENV+=("$base.env")
    fi
done

if [ ${#EXISTING_ENV[@]} -gt 0 ]; then
    printf "\n${YELLOW}Existing .env files found: ${EXISTING_ENV[*]}${RESET}\n"
    printf "Overwrite with fresh credentials? [y/N] "
    read -r REPLY
    if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
        info "Keeping existing .env files. Run --self-test to validate them."
        # Skip to stack start
        SKIP_GENERATE=true
    else
        SKIP_GENERATE=false
    fi
else
    SKIP_GENERATE=false
fi

if [ "${SKIP_GENERATE:-false}" = false ]; then
    printf "\n${BOLD}Generating .env files with fresh credentials...${RESET}\n\n"

    # Generate shared passwords
    PG_PASS=$(generate_password 32)
    REDIS_PASS=$(generate_password 32)
    SESSION_SECRET=$(generate_password 64)

    # common.env
    sed \
        -e "s/CHANGE_ME_STRONG_PASSWORD_HERE/$PG_PASS/" \
        -e "s/CHANGE_ME_REDIS_PASSWORD_HERE/$REDIS_PASS/" \
        "$ENV_DIR/common.env.example" > "$ENV_DIR/common.env"
    ok "common.env (Postgres: ${PG_PASS:0:8}..., Redis: ${REDIS_PASS:0:8}...)"

    # new-api.env
    sed \
        -e "s/CHANGE_ME_STRONG_PASSWORD_HERE/$PG_PASS/" \
        -e "s/CHANGE_ME_REDIS_PASSWORD_HERE/$REDIS_PASS/" \
        -e "s/CHANGE_ME_RANDOM_SESSION_SECRET/$SESSION_SECRET/" \
        "$ENV_DIR/new-api.env.example" > "$ENV_DIR/new-api.env"
    ok "new-api.env (DSN and Redis passwords match common.env)"

    # bifrost.env — no secrets to replace, just copy with defaults
    if [ ! -f "$ENV_DIR/bifrost.env" ]; then
        cat > "$ENV_DIR/bifrost.env" << 'EOF'
PORT=8080
HOST=0.0.0.0
LOG_STYLE=pretty
LOG_LEVEL=info
EOF
        ok "bifrost.env (defaults)"
    else
        ok "bifrost.env (kept existing)"
    fi

    # clewdr-*.env — generate with base settings
    for i in 1 2 3; do
        cat > "$ENV_DIR/clewdr-$i.env" << 'EOF'
CLEWDR_IP=0.0.0.0
CLEWDR_PORT=8484
CLEWDR_CHECK_UPDATE=false
CLEWDR_AUTO_UPDATE=false
CLEWDR_SKIP_RATE_LIMIT=true
EOF
        ok "clewdr-$i.env"
    done

    printf "\n${GREEN}All .env files generated.${RESET}\n"
fi

# ── Start the stack ───────────────────────────────────────
if [ "$NO_START" = true ]; then
    info "Skipping stack start (--no-start). To start manually:"
    info "  cd $DEPLOY_DIR"
    info "  docker compose --env-file env/common.env up -d"
    if [ "$WITH_CLEWDR" = true ]; then
        info "  docker compose --env-file env/common.env --profile clewdr up -d"
    fi
    exit 0
fi

printf "\n${BOLD}Starting the stack...${RESET}\n\n"

COMPOSE_CMD="docker compose --env-file env/common.env"
if [ "$WITH_CLEWDR" = true ]; then
    COMPOSE_CMD="$COMPOSE_CMD --profile clewdr"
    info "Including ClewdR services (you'll need to set up cookies after startup)"
else
    info "Starting without ClewdR (official API providers only)"
    info "To include ClewdR later: docker compose --env-file env/common.env --profile clewdr up -d"
fi

cd "$DEPLOY_DIR"
$COMPOSE_CMD up -d

printf "\n${BOLD}Waiting for services to become healthy...${RESET}\n"

# Wait up to 90 seconds for health checks
MAX_WAIT=90
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    UNHEALTHY=$($COMPOSE_CMD ps --format json 2>/dev/null | grep -c '"Health":"starting"' || true)
    if [ "$UNHEALTHY" -eq 0 ]; then
        break
    fi
    printf "."
    sleep 5
    WAITED=$((WAITED + 5))
done
printf "\n"

# Show status
$COMPOSE_CMD ps

# Run smoke tests if available
SMOKE_RUNNER="$SCRIPT_DIR/smoke/run-all.sh"
if [ -x "$SMOKE_RUNNER" ] || [ -f "$SMOKE_RUNNER" ]; then
    printf "\n${BOLD}Running smoke tests...${RESET}\n\n"
    bash "$SMOKE_RUNNER" || warn "Some smoke tests failed. Check the output above."
fi

# ── Success message ───────────────────────────────────────
printf "\n${BOLD}========================================${RESET}\n"
printf "${GREEN}  StarAPIHub is running!${RESET}\n"
printf "${BOLD}========================================${RESET}\n\n"
printf "  Nginx (public gateway):  http://localhost\n"
printf "  New-API admin UI:        http://localhost:3000\n"
printf "  Bifrost UI:              http://localhost:8080\n"
if [ "$WITH_CLEWDR" = true ]; then
    printf "  ClewdR-1:                http://localhost:18484\n"
    printf "  ClewdR-2:                http://localhost:18485\n"
    printf "  ClewdR-3:                http://localhost:18486\n"
fi
printf "\n"
printf "  ${BOLD}Next steps:${RESET}\n"
printf "  1. Configure Bifrost providers at http://localhost:8080\n"
printf "     (Add your Anthropic/OpenAI API keys)\n"
printf "  2. Create New-API channels at http://localhost:3000\n"
printf "     (See config/new-api/channels.example.md)\n"
if [ "$WITH_CLEWDR" = true ]; then
    printf "  3. Set up ClewdR cookies at http://localhost:18484\n"
    printf "     (Check docker logs cp-clewdr-1 for admin password)\n"
fi
printf "\n"
printf "  ${BOLD}Verify it works:${RESET}\n"
printf "  curl http://localhost/api/status\n"
printf "\n"
printf "  ${BOLD}Run smoke tests:${RESET}\n"
printf "  bash scripts/smoke/run-all.sh\n"
printf "\n"
printf "  ${BOLD}Validate environment:${RESET}\n"
printf "  bash scripts/setup.sh --self-test\n"
