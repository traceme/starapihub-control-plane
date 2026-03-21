# Integration Tests

## Overview

Integration tests verify end-to-end behavior across the full stack: Client -> New-API -> Bifrost -> Provider/ClewdR. Unlike smoke tests (which check individual service health), integration tests validate that the entire request path works correctly for real-world scenarios.

## Test Strategy

### What Integration Tests Cover

| Category | Test Scenario | Priority |
|----------|--------------|----------|
| **Request path** | Full chat completion through all three layers | P0 |
| **Model routing** | Each logical model reaches the correct provider pool | P0 |
| **Billing accuracy** | New-API deducts correct token balance for each model | P1 |
| **Fallback chain** | When primary provider fails, traffic shifts to fallback | P1 |
| **ClewdR isolation** | ClewdR-backed models never appear in premium channel | P0 |
| **User groups** | Group-restricted models reject unauthorized users | P1 |
| **Rate limiting** | New-API enforces per-user rate limits correctly | P2 |
| **Correlation** | X-Request-ID appears in New-API logs, Bifrost logs, and provider logs | P1 |
| **Streaming** | SSE streaming works end-to-end for chat completions | P1 |
| **Error propagation** | Provider errors surface correctly to the client | P2 |

### What Integration Tests Do NOT Cover

- Upstream system internals (covered by their own test suites)
- Load/performance testing (separate concern)
- UI testing for admin panels
- Secret rotation (operational procedure, not automated test)

## Current Status

Integration tests are currently **manual procedures** documented below. Automation is planned for a future phase.

### Why Manual First

1. The upstream systems lack test-mode APIs that would let us safely run automated tests without real credentials.
2. Full integration tests require real provider API keys (or mock providers), which adds infrastructure complexity.
3. Manual tests provide the same coverage and are sufficient for the initial deployment phase.

## Manual Test Procedures

### Test 1: Full Request Path (P0)

**Goal:** Verify a chat completion request flows from client through New-API, Bifrost, to the provider and back.

```bash
# Send a request to a premium model
curl -X POST https://api.example.com/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: integration-test-001" \
  -d '{
    "model": "claude-sonnet",
    "messages": [{"role": "user", "content": "Say hello in one word."}],
    "max_tokens": 5
  }'
```

**Expected:** HTTP 200 with a valid chat completion response.

**Verify:**
- Response contains `choices[0].message.content`
- Check New-API logs: `docker logs <newapi> | grep integration-test-001`
- Check Bifrost logs: `docker logs <bifrost> | grep integration-test-001`

### Test 2: Model Routing Correctness (P0)

**Goal:** Verify each logical model is routed to the expected provider.

For each model in `policies/logical-models.example.yaml`:
1. Send a request with that model name
2. Check Bifrost logs to confirm which provider received the request
3. Verify the provider matches the expected pool from the route policy

### Test 3: Fallback Chain (P1)

**Goal:** Verify traffic shifts to fallback when primary is unavailable.

1. Send a request to `cheap-chat` (standard tier) -- should succeed via primary
2. Disable the primary Anthropic provider in Bifrost config
3. Restart Bifrost: `docker-compose restart bifrost`
4. Send another request to `cheap-chat` -- should succeed via ClewdR fallback
5. Check Bifrost logs to confirm ClewdR was used
6. Re-enable the primary provider and restart Bifrost

### Test 4: Premium Isolation (P0)

**Goal:** Verify premium models never fall back to ClewdR.

1. Disable all official providers in Bifrost
2. Send a request to `claude-sonnet` (premium tier)
3. **Expected:** HTTP 502/503 (no providers available), NOT a ClewdR response
4. Re-enable official providers

### Test 5: Billing Accuracy (P1)

**Goal:** Verify New-API deducts the correct token balance.

1. Note the user's current balance in New-API admin UI
2. Send a request with known token count
3. Check the user's balance after the request
4. Verify the deduction matches the model's configured pricing

## Future Automation Plan

### Phase 1: Script-Based Automation

Convert manual procedures above into bash scripts with assertions. Store in `tests/integration/scripts/`.

### Phase 2: Mock Provider

Build a lightweight HTTP server that mimics provider responses. This eliminates the need for real API keys during testing.

Candidate approaches:
- Simple Go HTTP server returning canned responses
- Use Bifrost's test utilities if available
- Use a tool like `mockserver` or `wiremock`

### Phase 3: CI Integration

- Run smoke tests on every deployment
- Run integration tests nightly or on-demand
- Alert on failures via webhook/email

## Prerequisites for Running Integration Tests

- Full stack running (docker-compose up)
- Valid API keys configured in Bifrost
- A test user account in New-API with sufficient balance
- Access to container logs (`docker logs`)
- At least one working ClewdR instance (for fallback tests)
