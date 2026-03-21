# Smoke Test Verification Guide

## Purpose

Smoke tests confirm that the deployed stack is functional after initial deployment, config changes, or upstream upgrades. They validate connectivity, authentication, routing, billing recording, and isolation boundaries. Each test targets a specific integration point rather than testing upstream systems internally.

## Prerequisites

- The full stack is running via `deploy/docker-compose.yml`
- Environment files are configured in `deploy/env/`
- At least one New-API admin account exists
- At least one New-API user token (`sk-...`) is provisioned
- Bifrost has at least one official provider key configured
- ClewdR has at least one valid cookie (for risky/standard tests only)

## Test Inventory

### Test 1: Nginx Reachability

**What it validates**: Public ingress is working, TLS terminates correctly, traffic reaches New-API.

**Procedure**:
```bash
curl -s -o /dev/null -w "%{http_code}" https://api.example.com/api/status
```

**Expected result**: HTTP 200. If this fails, check Nginx container logs and TLS certificate mounts.

**Failure indicates**: Nginx is down, TLS is misconfigured, or New-API is unreachable from the public zone.

### Test 2: New-API Authentication

**What it validates**: New-API accepts valid tokens and rejects invalid ones.

**Procedure**:
```bash
# Valid token
curl -s -w "\n%{http_code}" https://api.example.com/v1/models \
  -H "Authorization: Bearer sk-valid-token"

# Invalid token
curl -s -w "\n%{http_code}" https://api.example.com/v1/models \
  -H "Authorization: Bearer sk-invalid-garbage"
```

**Expected result**: Valid token returns 200 with a model list. Invalid token returns 401.

**Failure indicates**: New-API database is down, token table is empty, or auth middleware is misconfigured.

### Test 3: Model Visibility

**What it validates**: Logical model names defined in the policy registry are visible to authenticated users.

**Procedure**:
```bash
curl -s https://api.example.com/v1/models \
  -H "Authorization: Bearer sk-valid-token" | jq '.data[].id'
```

**Expected result**: Output includes logical model names (`claude-sonnet`, `claude-opus`, `cheap-chat`, etc.) that match entries in `policies/logical-models.example.yaml`.

**Failure indicates**: New-API channels are not configured, or the model list in the channel does not match the policy registry.

### Test 4: Premium Route (Official Providers Only)

**What it validates**: Requests through the premium channel reach Bifrost and are served by official providers. ClewdR is never used.

**Procedure**:
```bash
curl -s https://api.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-valid-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "messages": [{"role": "user", "content": "Say hello in exactly 3 words."}],
    "max_tokens": 20
  }'
```

**Expected result**: HTTP 200 with a valid completion response. Bifrost logs (`docker logs cp-bifrost`) should show the request was routed to an official provider (e.g., `anthropic`), not a ClewdR instance.

**Failure indicates**: New-API channel `bifrost-premium` is misconfigured, Bifrost cannot reach official providers, or provider API keys are invalid.

### Test 5: Risky Route (ClewdR Allowed)

**What it validates**: Requests for lab/risky models are routed through ClewdR-eligible pools.

**Procedure**:
```bash
curl -s https://api.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-valid-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lab-claude",
    "messages": [{"role": "user", "content": "Say hello in exactly 3 words."}],
    "max_tokens": 20
  }'
```

**Expected result**: HTTP 200. Bifrost logs should show routing to a ClewdR instance or the unofficial pool.

**Failure indicates**: ClewdR cookies are expired, Bifrost routing rules for risky models are missing, or the `bifrost-risky` channel in New-API is misconfigured.

### Test 6: Billing Record Created

**What it validates**: New-API records token usage after a successful inference request.

**Procedure**:
```bash
# 1. Note the user's current balance
curl -s https://api.example.com/api/user/self \
  -H "Authorization: Bearer sk-valid-token" | jq '.data.quota'

# 2. Send an inference request (use Test 4 command)

# 3. Check balance again — it should have decreased
curl -s https://api.example.com/api/user/self \
  -H "Authorization: Bearer sk-valid-token" | jq '.data.quota'
```

**Expected result**: The user's quota decreases after the request. The admin dashboard (Logs section) shows a usage record with the correct model name and token count.

**Failure indicates**: New-API billing is not recording, the channel is not counting tokens, or the pricing model is not set.

### Test 7: Bifrost Health

**What it validates**: Bifrost is running and responsive on the internal network.

**Procedure**:
```bash
docker exec cp-new-api wget -q -O - http://bifrost:8080/health
```

**Expected result**: A health response (200 OK) confirming Bifrost is operational.

**Failure indicates**: Bifrost container is down, or the `core-net` Docker network is misconfigured.

### Test 8: ClewdR Network Isolation

**What it validates**: ClewdR instances are NOT reachable from outside the Docker network.

**Procedure**:
```bash
# From the host machine (not inside Docker):
curl -s --connect-timeout 3 http://localhost:8484/ && echo "FAIL: ClewdR is exposed" || echo "PASS: ClewdR is isolated"

# Verify no port bindings:
docker inspect cp-clewdr-1 --format '{{json .NetworkSettings.Ports}}' | grep -c "HostPort"
```

**Expected result**: Connection refused or timeout. No host port bindings for ClewdR containers.

**Failure indicates**: docker-compose.yml has `ports:` mappings for ClewdR (it should not), or firewall rules are too permissive.

### Test 9: Streaming Response

**What it validates**: SSE streaming works end-to-end through all layers without buffering.

**Procedure**:
```bash
curl -sN https://api.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-valid-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "messages": [{"role": "user", "content": "Count from 1 to 5."}],
    "max_tokens": 50,
    "stream": true
  }'
```

**Expected result**: Response arrives as SSE chunks (`data: {...}` lines) incrementally, not as a single blob. The final chunk is `data: [DONE]`.

**Failure indicates**: Nginx is buffering the response (check `proxy_buffering off` in nginx config), or New-API/Bifrost is not forwarding chunks.

### Test 10: X-Request-ID Propagation

**What it validates**: The correlation header is present in the response and logged by Nginx.

**Procedure**:
```bash
# Send a request and capture response headers
curl -sI https://api.example.com/v1/models \
  -H "Authorization: Bearer sk-valid-token" \
  -H "X-Request-ID: smoke-test-12345"

# Check Nginx logs for the ID
docker logs cp-nginx 2>&1 | grep "smoke-test-12345"
```

**Expected result**: The response includes `X-Request-ID: smoke-test-12345`. The Nginx access log contains the same ID.

**Failure indicates**: Nginx is not configured to pass through or inject `X-Request-ID`.

## Running All Tests

The script `scripts/smoke/run-all.sh` executes all tests sequentially and reports pass/fail:

```bash
cd control-plane
bash scripts/smoke/run-all.sh
```

The script reads configuration (API base URL, test token) from `deploy/env/smoke.env` or environment variables:

| Variable | Purpose | Example |
|----------|---------|---------|
| `SMOKE_BASE_URL` | Public API URL | `https://api.example.com` |
| `SMOKE_TOKEN` | Valid New-API token | `sk-test-...` |
| `SMOKE_ADMIN_TOKEN` | Admin token for admin API checks | `sk-admin-...` |

## Post-Test Checklist

After all tests pass:

- [ ] All 10 tests returned expected results
- [ ] Bifrost logs show correct provider selection for premium vs risky
- [ ] New-API billing records match the test requests
- [ ] ClewdR is confirmed unreachable from outside Docker
- [ ] No error spikes in any container logs
- [ ] Document the test run date and any deviations in a log file

## When to Run Smoke Tests

| Trigger | Required Tests |
|---------|---------------|
| Initial deployment | All 10 |
| Upstream version upgrade | All 10 |
| New-API channel change | Tests 3, 4, 5, 6 |
| Bifrost config change | Tests 4, 5, 7 |
| ClewdR cookie rotation | Test 5 |
| Nginx config change | Tests 1, 9, 10 |
| Network or firewall change | Tests 7, 8 |
| After failure drill | All 10 |
