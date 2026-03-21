# Observability and Request Correlation

## Correlation Strategy

To trace a request end-to-end, the system uses `X-Request-ID` as the primary correlation key.

### Header Flow

```
Client → Nginx → New-API → Bifrost → Provider
         ↓
    X-Request-ID generated (if not present)
    X-Real-IP set
    X-Forwarded-For set
```

### Per-Layer Logging

| Layer | Log Source | Key Fields |
|-------|-----------|------------|
| Nginx | access.log | `X-Request-ID`, client IP, response time, status |
| New-API | application logs + DB usage records | user ID, token ID, model, channel, tokens consumed |
| Bifrost | structured logs (JSON) | model, provider selected, route, latency, retry count |
| ClewdR | application logs | cookie used, upstream response status |

### Correlation Query

To find what happened to a specific request:

```bash
# 1. Get the X-Request-ID from client response headers or Nginx logs
REQUEST_ID="abc-123-def"

# 2. Search Nginx logs
docker logs cp-nginx 2>&1 | grep "$REQUEST_ID"

# 3. Search New-API logs
docker logs cp-new-api 2>&1 | grep "$REQUEST_ID"

# 4. Search Bifrost logs
docker logs cp-bifrost 2>&1 | grep "$REQUEST_ID"
```

> **LIMITATION**: Whether New-API and Bifrost propagate and log `X-Request-ID` depends on their implementations. If they don't, correlation falls back to timestamp + model name matching. This is a known constraint of the no-source-modification rule.

## Headers to Preserve

The control plane recommends these correlation headers:

| Header | Purpose | Set By |
|--------|---------|--------|
| `X-Request-ID` | Unique request identifier | Nginx (or client) |
| `X-Real-IP` | Original client IP | Nginx |
| `X-Forwarded-For` | Full proxy chain | Nginx |

### Aspirational Headers (require upstream support)

These headers would improve correlation but may not be supported without source changes:

| Header | Purpose | Would Be Set By |
|--------|---------|----------------|
| `X-Logical-Model` | Client-facing model name | New-API |
| `X-Route-Policy` | Which route policy was applied | Bifrost |
| `X-Provider-Used` | Which provider fulfilled request | Bifrost |
| `X-End-User` | End user identifier | New-API |

## Metrics

### Nginx Metrics
- Request rate, response codes, latency histogram
- Export via nginx stub_status or Prometheus exporter

### New-API Metrics
- Built-in usage tracking in database
- Query via admin API or dashboard

### Bifrost Metrics
- Native Prometheus endpoint (if telemetry plugin enabled)
- Provider latency, request count, error rate per provider

### ClewdR Metrics
- Health dashboard in admin UI
- Cookie status, rate limit state
