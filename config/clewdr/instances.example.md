# ClewdR Instance Configuration Reference

## Instance Layout

The control plane deploys three ClewdR instances. Each instance MUST use a separate
Claude.ai account/cookie to distribute load and reduce per-account rate limiting.

| Instance | Container | Internal URL | Cookie Account | Bifrost Provider ID |
|----------|-----------|-------------|---------------|---------------------|
| ClewdR 1 | `cp-clewdr-1` | `http://clewdr-1:8484` | Account A (dedicated) | `clewdr-1` |
| ClewdR 2 | `cp-clewdr-2` | `http://clewdr-2:8484` | Account B (dedicated) | `clewdr-2` |
| ClewdR 3 | `cp-clewdr-3` | `http://clewdr-3:8484` | Account C (dedicated) | `clewdr-3` |

All instances are on the `provider-net` Docker network. Only Bifrost can reach them.

## Endpoints Exposed by ClewdR

Each instance exposes these OpenAI-compatible endpoints (used by Bifrost):

| Endpoint | Purpose |
|----------|---------|
| `/v1/messages` | Claude Messages API (Anthropic format) |
| `/v1/chat/completions` | OpenAI Chat Completions format (Claude.ai backend) |
| `/code/v1/messages` | Claude Code Messages API |
| `/code/v1/chat/completions` | Claude Code Chat Completions format |

Bifrost should use `/v1/chat/completions` since it routes OpenAI-compatible requests.

## Configuration via clewdr.toml

ClewdR auto-generates `clewdr.toml` on first run in its data directory. The file
stores all persistent config including cookies. Key sections:

```toml
# Verified field names from clewdr/src/config/clewdr_config.rs.
# See docs/capability-audit.md -- ClewdR Automation Surfaces for full API reference.
# ClewdR config is also manageable via GET/POST /api/config endpoints.

# Server settings (require restart)
ip = "0.0.0.0"
port = 8484

# Authentication
password = "auto-generated-64-char-string"        # API key for inference (Bearer token)
admin_password = "auto-generated-64-char-string"   # Admin API and Web UI login

# Cookie behavior (hot-reloadable -- changes apply without restart)
skip_rate_limit = true      # Skip cookies in cooldown, try next
skip_restricted = false     # Skip cookies with restriction warnings
skip_non_pro = false        # Only use Pro account cookies
max_retries = 5             # Retry attempts across cookies

# API behavior (hot-reloadable)
preserve_chats = false      # Keep conversations on Claude.ai
web_search = false          # Enable web search in conversations
use_real_roles = true       # Use user/assistant role names

# Network (hot-reloadable)
# proxy = "socks5://proxy-host:1080"    # Outbound proxy

# Cookies are managed programmatically via POST /api/cookie endpoint.
# See docs/capability-audit.md -- Cookie Management section.
# cookie_array = [...]
```

_Corrected 2026-03-21: verified against clewdr/src/config/clewdr_config.rs and clewdr/src/router.rs._

Environment variables (with `CLEWDR_` prefix) override clewdr.toml values.
See `deploy/env/clewdr-1.env.example` for the full env var reference.

## Per-Instance Setup Checklist

Repeat for each ClewdR instance (1, 2, 3):

- [ ] Verify the container is running: `docker ps | grep cp-clewdr-N`
- [ ] Retrieve auto-generated admin password: `docker logs cp-clewdr-N 2>&1 | head -30`
- [ ] Access the admin UI using one of the methods below
- [ ] Log in with the admin password
- [ ] Navigate to the Claude tab
- [ ] Paste a browser cookie from a **dedicated** Claude.ai account
- [ ] Verify cookie status shows as healthy (green)
- [ ] Note the API password (shown in settings or logs) — needed for Bifrost config
- [ ] Test with a direct API call:
  ```bash
  curl -X POST http://clewdr-N:8484/v1/chat/completions \
    -H "Authorization: Bearer YOUR_CLEWDR_PASSWORD" \
    -H "Content-Type: application/json" \
    -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}'
  ```
- [ ] Register this instance as a custom provider in Bifrost (see `config/bifrost/config.example.json`)

## Accessing Internal ClewdR Instances

ClewdR instances are on the `provider-net` (no published ports). Use one of these methods:

### Option 1: SSH tunnel (recommended for remote servers)
```bash
# From your local machine — forwards local port to container port
# First, temporarily uncomment the ports line in docker-compose.yml for the instance
ssh -L 18484:127.0.0.1:18484 your-server
# Then open http://localhost:18484 in your browser
```

### Option 2: Temporary port exposure (development only)
```yaml
# In docker-compose.yml, uncomment for the instance you need:
# clewdr-1: ports: ["127.0.0.1:18484:8484"]
# clewdr-2: ports: ["127.0.0.1:18485:8484"]
# clewdr-3: ports: ["127.0.0.1:18486:8484"]
```
Then restart: `docker compose --env-file env/common.env up -d clewdr-1`

### Option 3: Docker exec (for CLI-only access)
```bash
docker exec -it cp-clewdr-1 sh
# Inside the container, use curl or wget for API calls
```

## Cookie Rotation

When a cookie expires or gets restricted:

1. Access the ClewdR admin UI for the affected instance
2. Remove the old cookie
3. Paste a new cookie from the same or a replacement account
4. Verify the new cookie status is healthy
5. No Bifrost or New-API changes needed — the instance URL stays the same

## Scaling ClewdR

To add more ClewdR instances:

1. Add a new service block in `docker-compose.yml` (copy `clewdr-3` as template)
2. Create a new `env/clewdr-4.env` file
3. Add a new volume `clewdr-4-data`
4. Connect it to `provider-net`
5. Register the new instance as a custom provider in Bifrost
6. Update the routing rule weights to include the new instance
