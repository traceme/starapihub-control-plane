# ClewdR Operations Guide

## Role

ClewdR is an **unofficial Claude proxy** that routes requests through Claude.ai using browser cookies. It provides OpenAI-compatible endpoints, making it consumable by Bifrost as if it were a standard OpenAI provider.

## Hard Rules

1. **Internal-only**: ClewdR must NEVER be exposed to the public internet.
2. **Never silently premium**: ClewdR must NEVER be in a premium route policy pool. It may only appear in `standard` (as last-resort fallback) or `risky` (as primary) policies.
3. **No SLA**: ClewdR provides no uptime, latency, or reliability guarantees.
4. **Cookie-dependent**: Service availability depends on valid Claude.ai browser cookies.

## Instance Model: One Cookie Per Instance

Each ClewdR instance should use a **separate Claude.ai cookie/account**. This:
- Distributes rate limiting across accounts
- Isolates failures (one expired cookie doesn't take down all instances)
- Makes credential rotation easier (rotate one instance at a time)

Default deployment: 3 instances (`clewdr-1`, `clewdr-2`, `clewdr-3`).

## Configuration

ClewdR is configured primarily through its **web admin UI** and `clewdr.toml` config file.

### First Boot

1. ClewdR generates a random admin password on first start
2. The password is printed to container logs:
   ```bash
   docker logs cp-clewdr-1
   # Look for the admin password line
   ```
3. ClewdR creates a default `clewdr.toml` in its data directory

### Adding Cookies

1. Port-forward to the ClewdR instance (it's internal-only):
   ```bash
   docker exec -it cp-clewdr-1 sh
   # Or use SSH tunneling:
   # ssh -L 18484:localhost:8484 your-server
   ```
2. Open `http://localhost:8484` (or the forwarded port)
3. Log in with the admin password from container logs
4. Go to the **Claude** tab
5. Paste browser cookies extracted from Claude.ai
6. Save — ClewdR tracks cookie health automatically

### Cookie Extraction

To get Claude.ai cookies:
1. Log in to Claude.ai in a browser
2. Open DevTools → Application → Cookies
3. Copy all cookies for `claude.ai` domain
4. Or use a cookie export browser extension

### Environment Variables

All ClewdR config options can be set via `CLEWDR_` prefixed env vars (nested keys use `__`):

```bash
CLEWDR_IP=0.0.0.0            # Binding IP (default: 127.0.0.1)
CLEWDR_PORT=8484              # Listening port (default: 8484)
CLEWDR_PASSWORD=my-api-key    # API key for inference endpoints (auto-generated if empty)
CLEWDR_ADMIN_PASSWORD=secret  # Admin UI password (auto-generated if empty)
CLEWDR_PROXY=socks5://host:1080  # Outbound proxy for Claude.ai
CLEWDR_MAX_RETRIES=5          # Retry attempts (default: 5)
CLEWDR_SKIP_RATE_LIMIT=true   # Skip rate-limited cookies (default: true)
CLEWDR_CHECK_UPDATE=false     # Disable update checks in Docker
CLEWDR_AUTO_UPDATE=false      # Disable auto-update in Docker
```

### Key Configuration Options (in clewdr.toml)

| Setting | Purpose | Hot-Reload |
|---------|---------|-----------|
| `password` | API key for inference endpoints | No |
| `admin_password` | Admin UI authentication | No |
| `ip`, `port` | Server bind address | No |
| `proxy` | Outbound HTTP/SOCKS5 proxy for Claude.ai | Yes |
| `rproxy` | Reverse proxy URL override for Claude API | Yes |
| `max_retries` | Retry attempts for failed requests | Yes |
| `skip_rate_limit` | Skip rate-limited cookies | Yes |
| `skip_restricted` | Skip restricted/limited accounts | Yes |
| `skip_non_pro` | Skip free-tier accounts | Yes |
| `use_real_roles` | Use real system roles | Yes |
| `preserve_chats` | Preserve conversation history | Yes |

### Admin API Endpoints

| Method | Endpoint | Purpose | Auth |
|--------|----------|---------|------|
| GET | `/api/auth` | Verify admin token | Bearer admin_password |
| GET | `/api/config` | Get current config | Bearer admin_password |
| POST | `/api/config` | Update config (hot-reloadable fields) | Bearer admin_password |
| GET | `/api/cookies` | Get all cookie statuses | Bearer admin_password |
| POST | `/api/cookie` | Add new cookie | Bearer admin_password |
| PUT | `/api/cookie` | Update cookie flags | Bearer admin_password |
| DELETE | `/api/cookie` | Remove a cookie | Bearer admin_password |
| GET | `/api/version` | Get version info | None |

## Endpoints

| Endpoint | Protocol | Use |
|----------|----------|-----|
| `http://clewdr-N:8484/v1/messages` | Anthropic Messages API | Bifrost (via anthropic provider type) |
| `http://clewdr-N:8484/v1/chat/completions` | OpenAI Chat Completions | Bifrost (via openai provider type) |
| `http://clewdr-N:8484/code/v1/messages` | Claude Code Messages API | Not used in this architecture |
| `http://clewdr-N:8484/code/v1/chat/completions` | Claude Code Chat Completions | Not used in this architecture |
| `http://clewdr-N:8484/` | Admin Web UI | Operator access only |

For Bifrost integration, use the OpenAI-compatible endpoint (`/v1/chat/completions`) or the Anthropic-compatible endpoint (`/v1/messages`) depending on how you register ClewdR in Bifrost.

## Health Checks

### Container-Level
```bash
# Basic HTTP check (returns the admin UI page)
wget -q -O - http://clewdr-N:8484/
```

### Cookie Health
ClewdR tracks cookie health in its dashboard. Check for:
- Expired cookies (need replacement)
- Rate-limited cookies (need cooldown)
- Blocked cookies (account may be suspended)

### Automated Monitoring
```bash
# Check if ClewdR is responding to API requests
curl -s -o /dev/null -w "%{http_code}" \
  http://clewdr-1:8484/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-password>" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"ping"}],"max_tokens":5}'
# Expect 200 if healthy, 4xx/5xx if cookies expired
```

## Failure Modes

| Failure | Symptom | Response |
|---------|---------|----------|
| Cookie expired | 401/403 from Claude.ai | Replace cookie via admin UI |
| Cookie rate-limited | 429 from Claude.ai | Wait for cooldown, or rotate to another instance |
| Account suspended | Persistent 403 | Remove cookie, use different account |
| ClewdR process crash | Connection refused | Container restart (handled by `restart: unless-stopped`) |
| All instances down | Bifrost marks pool unhealthy | Bifrost falls back to official pools (per route policy) |

## Credential Rotation

1. Extract fresh cookies from a Claude.ai browser session
2. Port-forward to the target ClewdR instance
3. Open admin UI → Claude tab
4. Replace old cookies with new ones
5. Verify health in the dashboard
6. Repeat for each instance (stagger to avoid downtime)

**Do NOT rotate all instances simultaneously** — keep at least one healthy while rotating others.

## Scaling

To add a new ClewdR instance:

1. Add a new service in `deploy/docker-compose.yml` (copy `clewdr-3` pattern)
2. Create a new env file in `deploy/env/`
3. Add the instance to `policies/provider-pools.example.yaml` under `unofficial-clewdr`
4. Sync the new provider to Bifrost config
5. Start the new container and configure its cookies

## Risk Documentation

See also: `docs/unofficial-provider-risk.md` for a complete risk analysis.
