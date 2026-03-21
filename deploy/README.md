# Deployment Guide

## Fastest Path (Automated Setup)

```bash
cd control-plane
bash scripts/setup.sh              # Without ClewdR (official APIs only)
bash scripts/setup.sh --with-clewdr  # With ClewdR (requires cookie setup)
```

The setup script generates `.env` files with random passwords, starts the stack, and runs smoke tests. See below for manual setup.

## Prerequisites

- Docker Engine 24+ and Docker Compose v2 (the `docker compose` plugin, not legacy `docker-compose`)
- At least 4 GB RAM and 2 CPU cores for the full stack
- For production: a domain name with TLS certificates (see nginx config)

## Architecture Overview

```
Internet
  |
  v
[nginx :443/:80]        public-net
  |
  v
[new-api :3000]         public-net + core-net
  |
  v
[bifrost :8080]         core-net + provider-net
  |
  +---> [clewdr-1 :8484]  provider-net
  +---> [clewdr-2 :8484]  provider-net
  +---> [clewdr-3 :8484]  provider-net
  +---> Anthropic API      (outbound internet)
  +---> OpenAI API         (outbound internet)

[postgres :5432]        core-net
[redis :6379]           core-net
```

Only nginx has published ports. All other services are internal.

## Quick Start (Manual)

### 1. Prepare Environment Files

```bash
cd control-plane/deploy/env
cp common.env.example common.env
cp new-api.env.example new-api.env
cp bifrost.env.example bifrost.env
# Only if using ClewdR:
cp clewdr-1.env.example clewdr-1.env
cp clewdr-2.env.example clewdr-2.env
cp clewdr-3.env.example clewdr-3.env
```

Edit each `.env` file. At minimum:

| File | What to change |
|------|----------------|
| `common.env` | `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, version pins |
| `new-api.env` | `SQL_DSN` and `REDIS_CONN_STRING` (must match passwords in common.env), `SESSION_SECRET` |
| `bifrost.env` | Usually fine with defaults |
| `clewdr-*.env` | Optionally set `CLEWDR_PASSWORD` and `CLEWDR_ADMIN_PASSWORD` |

### 2. Prepare TLS Certificates

```bash
cd control-plane/deploy
mkdir -p certs
cp /path/to/your/fullchain.pem certs/server.crt
cp /path/to/your/privkey.pem   certs/server.key
chmod 600 certs/server.key
```

### 3. Set Your Domain in Nginx Config

Edit `control-plane/config/nginx/nginx.conf` and replace `api.example.com` with your actual domain (appears twice: HTTPS server and HTTP redirect).

### 4. Launch the Stack

```bash
cd control-plane/deploy

# Without ClewdR (official API providers only):
docker compose --env-file env/common.env up -d

# With ClewdR (requires cookie setup in step 6B):
docker compose --env-file env/common.env --profile clewdr up -d
```

### 5. Verify Health

```bash
# Check all containers are running and healthy
docker compose --env-file env/common.env ps

# Check individual service health
docker compose --env-file env/common.env exec new-api \
  wget -q -O - http://localhost:3000/api/status

docker compose --env-file env/common.env exec bifrost \
  wget -q -O - http://localhost:8080/health

# Check nginx can reach new-api
curl -k https://localhost/health
```

### 6. Configure Upstream Systems

After the stack is running, configure in this order:

**Step A — Bifrost providers:**
Bifrost needs provider API keys before it can route traffic.

Option 1 (UI): Temporarily expose Bifrost's port for setup:
```bash
# Uncomment the ports line for bifrost in docker-compose.yml, then:
docker compose --env-file env/common.env up -d bifrost
# Access Bifrost UI at http://localhost:8080
# Add Anthropic/OpenAI keys, configure providers
# Then re-comment the ports line and restart
```

Option 2 (Config file): Mount a pre-built config:
```bash
# Edit config/bifrost/config.example.json, save as config.json
# Uncomment the config mount line in docker-compose.yml
docker compose --env-file env/common.env up -d bifrost
```

See `config/bifrost/config.example.json` for the template.

**Step B — ClewdR cookies:**
Each ClewdR instance needs a Claude.ai browser cookie from a dedicated account.

```bash
# Get the auto-generated admin password from logs
docker logs cp-clewdr-1 2>&1 | head -20

# Temporarily expose for setup (uncomment ports in docker-compose.yml)
# Or use SSH tunnel from your local machine:
ssh -L 18484:127.0.0.1:18484 your-server
# Then access http://localhost:18484 in your browser
# Navigate to Claude tab -> paste cookie
```

Repeat for clewdr-2 and clewdr-3 with different accounts.

**Step C — New-API channels:**
Create channels in New-API's admin UI pointing to Bifrost.

```bash
# Access New-API admin at https://your-domain/
# Default credentials: check New-API upstream documentation
# Create channels as described in config/new-api/channels.example.md
```

**Step D — Smoke test:**
```bash
cd control-plane
bash scripts/smoke/run-all.sh
```

### 7. Verify It Works

Send a test request through the full path:

```bash
# Check the gateway is responding
curl http://localhost/api/status

# List available models (requires API key from New-API admin)
curl http://localhost/api/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"

# Send a chat completion request
curl http://localhost/api/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

If using ClewdR, replace `claude-sonnet` with the appropriate logical model name from your channel config.

### 8. Validate Environment

Run the self-test to verify all credentials match and services are healthy:

```bash
bash scripts/setup.sh --self-test
```

## Development Mode

For local development, expose internal services on localhost only:

```yaml
# In docker-compose.yml, uncomment:
# new-api:  ports: ["127.0.0.1:3000:3000"]
# bifrost:  ports: ["127.0.0.1:8080:8080"]
# clewdr-1: ports: ["127.0.0.1:18484:8484"]
# clewdr-2: ports: ["127.0.0.1:18485:8484"]
# clewdr-3: ports: ["127.0.0.1:18486:8484"]
```

Then restart: `docker compose --env-file env/common.env up -d`

## Viewing Logs

```bash
# All services
docker compose --env-file env/common.env logs -f

# Specific service
docker compose --env-file env/common.env logs -f new-api
docker compose --env-file env/common.env logs -f bifrost
docker compose --env-file env/common.env logs -f clewdr-1

# New-API file logs (if --log-dir is set)
docker compose --env-file env/common.env exec new-api ls /app/logs/
```

## Upgrading Upstream Components

See `docs/upgrade-strategy.md` for detailed guidance. Quick version:

```bash
# Edit env/common.env to pin new versions
# e.g., NEWAPI_VERSION=v1.2.3, BIFROST_VERSION=v2.0.0

# Pull new images
docker compose --env-file env/common.env pull

# Rolling restart (databases stay up)
docker compose --env-file env/common.env up -d
```

## Stopping and Cleanup

```bash
# Stop all services (data volumes preserved)
docker compose --env-file env/common.env down

# Stop and remove ALL data volumes (DESTRUCTIVE — loses database, config, logs)
docker compose --env-file env/common.env down -v
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| nginx returns 502 | Is new-api healthy? `docker logs cp-new-api` |
| new-api can't connect to DB | Does `SQL_DSN` in new-api.env match postgres credentials in common.env? |
| Bifrost returns empty responses | Are provider keys configured? Check Bifrost UI or config.json |
| ClewdR returns 401 | Is the cookie valid? Check ClewdR admin UI |
| Streaming hangs | Check `proxy_read_timeout` in nginx.conf matches `STREAMING_TIMEOUT` |
| Container keeps restarting | Check `docker logs <container-name>` for startup errors |
