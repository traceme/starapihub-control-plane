#!/usr/bin/env bash
# StarAPIHub — Self-signed TLS certificate generator
#
# Generates a self-signed ECDSA P-256 certificate for the appliance's nginx
# ingress. Output files match the paths expected by nginx.conf:
#   ssl_certificate     /etc/nginx/certs/server.crt
#   ssl_certificate_key /etc/nginx/certs/server.key
#
# Usage:
#   ./gen-certs.sh                              # Generate for localhost
#   ./gen-certs.sh --domain myapp.example.com   # Generate for custom domain
#   ./gen-certs.sh --force                      # Regenerate even if certs exist

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$(cd "$SCRIPT_DIR/../deploy/certs" && pwd)"

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

# ── Defaults ──────────────────────────────────────────────
DOMAIN="localhost"
FORCE=false

# ── Parse arguments ──────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --force)
            FORCE=true
            shift
            ;;
        *)
            echo -e "${YELLOW}Unknown option: $1${RESET}" >&2
            echo "Usage: $0 [--domain DOMAIN] [--force]"
            exit 1
            ;;
    esac
done

# ── Idempotency check ───────────────────────────────────
if [[ -f "$CERT_DIR/server.crt" && -f "$CERT_DIR/server.key" && "$FORCE" != "true" ]]; then
    echo -e "${CYAN}Certificates already exist at $CERT_DIR/server.crt -- use --force to regenerate${RESET}"
    exit 0
fi

# ── Generate certificate ────────────────────────────────
echo -e "${BOLD}Generating self-signed TLS certificate...${RESET}"

openssl req -x509 \
    -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -nodes \
    -days 365 \
    -keyout "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -subj "/CN=$DOMAIN" \
    -addext "subjectAltName=DNS:$DOMAIN,DNS:localhost,IP:127.0.0.1" \
    2>/dev/null

# ── Set permissions ──────────────────────────────────────
chmod 600 "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/server.crt"

# ── Summary ──────────────────────────────────────────────
echo -e "${GREEN}Generated self-signed TLS certificate:${RESET}"
echo -e "  Certificate: ${CYAN}$CERT_DIR/server.crt${RESET}"
echo -e "  Private key: ${CYAN}$CERT_DIR/server.key${RESET}"
echo -e "  Domain (CN): ${BOLD}$DOMAIN${RESET}"
echo -e "  SANs:        DNS:$DOMAIN, DNS:localhost, IP:127.0.0.1"
echo -e "  Validity:    365 days"

if [[ "$DOMAIN" != "localhost" ]]; then
    echo ""
    echo -e "${YELLOW}Note: Update server_name in config/nginx/nginx.conf to match '$DOMAIN'${RESET}"
fi
