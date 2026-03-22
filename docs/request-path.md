# Request Path Documentation

## End-to-End Request Flow

```
  ┌────────┐   HTTPS    ┌───────┐   HTTP    ┌──────────┐   HTTP    ┌──────────┐   HTTP    ┌──────────────┐
  │ Client │───────────▶│ Nginx │─────────▶│ New-API  │─────────▶│ Bifrost  │─────────▶│  Provider    │
  │ SDK    │            │ WAF   │          │ :3000    │          │ :8080    │          │  (Official / │
  └────────┘            └───────┘          └──────────┘          └──────────┘          │   ClewdR)    │
                                                                                       └──────────────┘
```

## Step-by-Step

### 1. Client -> Nginx (Public Ingress)

- Client sends `POST https://api.example.com/v1/chat/completions`
- Nginx terminates TLS
- Nginx injects `X-Request-ID` if not present
- Nginx proxies to New-API on the `core` network

**Headers injected by Nginx**:
```
X-Request-ID: <uuid>              # If not already present
X-Oneapi-Request-Id: <uuid>       # Same ID in New-API's header format
X-Real-IP: <client-ip>            # Original client IP
X-Forwarded-For: <client-ip>      # Standard forwarded header
X-Forwarded-Proto: https          # Original protocol
```

### 2. Nginx -> New-API (Auth + Billing)

New-API receives the request and:

1. **Authenticates** the client using the `Authorization: Bearer sk-...` token
2. **Checks quota/balance** — rejects if insufficient
3. **Resolves the model name** — maps the requested model to an internal channel
4. **Selects a channel** — picks from available channels (e.g., `bifrost-premium`)
5. **Proxies the request** to the channel's base URL (Bifrost)
6. **Records usage** — logs tokens consumed, updates billing

**What New-API adds to the forwarded request**:
- The channel's configured API key (used by Bifrost for identification, not billing)
- Model name mapping (if the channel maps model names)

### 3. New-API -> Bifrost (Routing + Execution)

Bifrost receives the request and:

1. **Identifies the route** based on model name and/or headers
2. **Selects a provider pool** based on route policy (premium, standard, risky)
3. **Picks a provider** from the pool using its load-balancing strategy
4. **Forwards the request** to the selected provider
5. **Handles failures** — retries, falls back to next provider in pool, or next pool in fallback chain
6. **Streams the response** back to New-API

### 4. Bifrost -> Provider (Inference)

For **official providers** (OpenAI, Anthropic, Bedrock, etc.):
- Bifrost sends the request using the provider's native API format
- Authentication uses API keys stored in Bifrost's config

For **ClewdR instances**:
- Bifrost sends the request to `http://clewdr-N:8484/v1/chat/completions`
- ClewdR proxies it through Claude.ai using stored cookies
- Response comes back in OpenAI-compatible format

### 5. Response Path

The response flows back through the same chain in reverse:

```
Provider -> Bifrost -> New-API -> Nginx -> Client
```

- **Bifrost** may add routing metadata to response headers
- **New-API** records token usage for billing, may add rate-limit headers
- **Nginx** passes everything through

For **streaming responses** (`stream: true`):
- Each layer proxies SSE chunks as they arrive
- No layer should buffer the entire response before forwarding

## Request Correlation

To trace a request end-to-end across all layers:

| Header | Set By | Purpose |
|--------|--------|---------|
| `X-Request-ID` | Nginx (or client) | Unique request identifier across all layers |
| `Authorization` | Client | User authentication (consumed by New-API) |

### Correlation Strategy

1. **Nginx access log**: Contains `X-Request-ID`, client IP, upstream response time
2. **New-API log**: Contains `X-Request-ID` (if forwarded), user ID, token ID, model, channel, tokens used
3. **Bifrost log**: Contains request metadata, provider selected, latency, retry count
4. **ClewdR log**: Contains request status, cookie used, upstream latency

To correlate: search all log sources for the same `X-Request-ID`.

> **Verified behavior** (see docs/observability.md for source citations):
> - Nginx generates X-Request-ID and forwards it as both `X-Request-ID` and `X-Oneapi-Request-Id`
> - New-API generates its own internal ID (`X-Oneapi-Request-Id` in response), ignoring incoming headers
> - Bifrost reads `x-request-id` from the request or generates a UUID
> - Cross-layer correlation requires matching by timestamp + model when IDs diverge

## Failure Modes

| Failure | Detected By | Behavior |
|---------|-------------|----------|
| Client auth failure | New-API | 401 returned immediately |
| Quota exceeded | New-API | 429 returned immediately |
| All providers in pool down | Bifrost | Bifrost tries fallback pools, then returns 502 |
| ClewdR cookie expired | ClewdR (returns error) | Bifrost marks provider unhealthy, tries next |
| Official API rate limit | Provider (returns 429) | Bifrost retries with backoff or falls back |
| Bifrost itself down | New-API (connection refused) | New-API returns 502 to client |
| New-API itself down | Nginx (connection refused) | Nginx returns 502 to client |

## Latency Budget

Approximate per-hop overhead (excluding inference time):

| Hop | Expected Overhead |
|-----|-------------------|
| Nginx -> New-API | < 1ms |
| New-API auth + channel select | 1-5ms |
| New-API -> Bifrost | < 1ms (internal network) |
| Bifrost routing + provider select | < 1ms (~11µs at 5K RPS per Bifrost docs) |
| Bifrost -> Provider | Network-dependent |

Total control-plane overhead (not including inference): **~5-10ms**

The external control plane adds **zero latency** to the request path — it only operates at config/deploy time.
