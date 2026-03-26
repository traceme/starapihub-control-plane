# Failure Drill Documentation

## Purpose

These drills verify that the system behaves correctly when components fail. Run them periodically in a staging environment (or cautiously in production during low-traffic windows).

Each drill states which alerts should fire. Alert severities are defined in [alert-model.md](alert-model.md).

## Drill 1: Official Provider Outage

**Scenario**: Anthropic API is down or returning errors.

**Steps**:
1. In Bifrost config, temporarily disable/remove the Anthropic provider key
2. Send requests for `claude-sonnet` through the premium channel

**Expected behavior**:
- Premium policy: Bifrost tries Anthropic, fails, falls back to OpenAI pool (if Claude models are also available there via alternative routing). If no fallback exists for Claude models, returns 502.
- Standard policy: Bifrost tries official, fails, falls back to ClewdR pool.
- Risky policy: ClewdR is primary — unaffected.

**Verification**:
- Premium `claude-sonnet` requests should fail gracefully (not hang)
- Standard/risky requests should succeed via ClewdR
- New-API should record the failures in logs

**Expected alerts**: WARNING `provider-error` for Anthropic (auth/connection failure). No CRITICAL unless all providers for a tier are unreachable.

**Cleanup**: Re-enable the Anthropic provider key.

## Drill 2: All ClewdR Instances Down

**Scenario**: All ClewdR instances are unreachable or have expired cookies.

**Steps**:
1. Stop all ClewdR containers:
   ```bash
   docker compose stop clewdr-1 clewdr-2 clewdr-3
   ```
2. Send requests through all three channel tiers

**Expected behavior**:
- Premium: Unaffected (ClewdR not in premium pools)
- Standard: Official providers handle all traffic (ClewdR fallback unavailable but not needed unless officials fail too)
- Risky: ClewdR primary fails, falls back to official pool. Requests succeed but at higher cost.

**Verification**:
- Premium requests work normally
- Standard requests work normally
- Risky requests succeed via official fallback
- Bifrost logs show ClewdR marked unhealthy

**Expected alerts**: WARNING `service-down` for each ClewdR instance. CRITICAL `cookie-exhaustion` if cookie-status runs during the outage (instances unreachable = below threshold).

**Cleanup**: `docker compose start clewdr-1 clewdr-2 clewdr-3`

## Drill 3: Bifrost Down

**Scenario**: Bifrost process crashes or container stops.

**Steps**:
1. Stop Bifrost:
   ```bash
   docker compose stop bifrost
   ```
2. Send any API request through New-API

**Expected behavior**:
- All requests fail with 502 (New-API cannot reach Bifrost)
- New-API itself stays up and responsive (returns error, doesn't crash)
- Admin UI and non-relay endpoints still work

**Verification**:
- `curl https://api.example.com/api/status` returns success (New-API is up)
- `curl https://api.example.com/v1/chat/completions` returns 502
- New-API error logs show connection refused to Bifrost

**Expected alerts**: CRITICAL `service-down` for Bifrost. All inference traffic blocked.

**Cleanup**: `docker compose start bifrost`

## Drill 4: ClewdR Not Publicly Accessible

**Scenario**: Verify network isolation — ClewdR should never be reachable from the internet.

**Steps**:
1. From an external machine (not on the Docker network), try to reach ClewdR:
   ```bash
   curl -v http://your-server-ip:8484/
   curl -v http://your-server-ip:18484/
   ```

**Expected behavior**:
- Connection refused or timeout (ClewdR ports not exposed to host)
- No response from any ClewdR instance

**Verification**:
- All external connection attempts fail
- `docker compose ps` shows ClewdR has no port mappings

**Expected alerts**: None — this is a security verification, not a failure condition.

## Drill 5: Database Failure

**Scenario**: PostgreSQL crashes.

**Steps**:
1. Stop PostgreSQL:
   ```bash
   docker compose stop postgres
   ```
2. Send API requests and try admin UI operations

**Expected behavior**:
- In-flight requests that don't need DB may succeed
- New requests may fail if New-API can't check auth/quota
- New-API should not crash — should return errors gracefully
- Bifrost and ClewdR are unaffected (they don't use this DB)

**Expected alerts**: CRITICAL `service-down` for PostgreSQL. New-API depends on it — expect health check to flag New-API as degraded.

**Cleanup**: `docker compose start postgres`

## Drill 6: Single ClewdR Cookie Expiry

**Scenario**: One ClewdR instance has an expired cookie.

**Steps**:
1. Don't actually expire the cookie — instead, observe natural expiry
2. Or in staging: intentionally set a bad cookie via ClewdR admin UI

**Expected behavior**:
- Bifrost detects the failed ClewdR instance (error responses)
- Bifrost routes to other ClewdR instances in the pool
- If all ClewdR instances fail, Bifrost falls back per route policy
- Bifrost marks the bad instance as unhealthy

**Verification**:
- Requests through risky/standard continue succeeding
- Bifrost logs show one provider marked unhealthy
- No impact on premium traffic

**Expected alerts**: WARNING `cookie-exhaustion` for the affected instance (below per-instance threshold). INFO `service-down` for the single ClewdR instance if health check detects it.

## Drill 7: New-API Down

**Scenario**: New-API crashes or becomes unresponsive.

**Steps**:
1. Stop New-API:
   ```bash
   docker compose stop new-api
   ```
2. Send API requests from a client
3. Try accessing the admin UI

**Expected behavior**:
- All client requests fail (Nginx returns 502 — cannot reach New-API)
- Bifrost and ClewdR remain healthy but idle (they receive no traffic without New-API)
- Nginx itself stays up and returns 502 errors (not connection refused)
- No data loss — New-API's state is in PostgreSQL, which is still running

**Verification**:
- `curl https://api.example.com/v1/chat/completions` returns 502
- `docker compose ps` shows New-API stopped, all other services healthy
- After restart: `docker compose start new-api` — all traffic resumes normally
- Billing records from before the outage are intact

**Expected alerts**: CRITICAL `service-down` for New-API. All client traffic blocked.

**Cleanup**: `docker compose start new-api`

**Key takeaway**: New-API is a single point of failure for all client traffic. There is no bypass. This is by design — New-API owns auth and billing. To reduce this risk in production, consider running multiple New-API replicas behind a load balancer.

## Drill 8: Provider Rate Limiting

**Scenario**: An official provider starts returning 429 (rate limited) on most requests.

**Steps**:
1. Send a burst of requests through the premium channel for a single model
2. Or simulate by temporarily reducing Bifrost's retry timeout to trigger faster failover

**Expected behavior**:
- Bifrost detects 429 responses and retries with backoff
- If retries exhaust, Bifrost tries the next provider in the pool chain
- For premium policy with only official pools: if all official providers are rate-limited, the client gets an error (no ClewdR fallback)
- For standard policy: Bifrost may fall back to ClewdR pool

**Verification**:
- Bifrost logs show retry attempts and eventual fallback
- Client receives either a successful response (from fallback) or a clear error (if no fallback available)
- No silent routing to ClewdR for premium-tier requests

**Expected alerts**: INFO `provider-error` (429 rate limit). Escalates to WARNING if retries exhaust and no fallback available.

**Cleanup**: Wait for rate limit window to pass. No config changes needed.

## Drill Schedule

| Drill | Frequency | Environment |
|-------|-----------|-------------|
| 1: Provider outage | Quarterly | Staging |
| 2: All ClewdR down | Quarterly | Staging or Production (off-peak) |
| 3: Bifrost down | Quarterly | Staging |
| 4: ClewdR isolation | Monthly | Production |
| 5: Database failure | Quarterly | Staging |
| 6: Cookie expiry | Observe naturally | Production |
| 7: New-API down | Quarterly | Staging |
| 8: Provider rate limiting | Quarterly | Staging |
