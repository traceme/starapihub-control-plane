# Logs Contract

## Overview

The appliance produces multiple log streams, each serving a different operational need. This document defines what each log type contains, where it comes from, and how operators access it. Use this as the canonical reference when investigating requests, diagnosing failures, or reviewing usage.

## Operator Access Paths

| Log Type | Access Method | What You See |
|----------|--------------|--------------|
| Gateway requests | Dashboard Logs page | HTTP requests through nginx: method, path, status, latency, model, request ID |
| Billing/usage | New-API admin UI (`http://<host>:3000`) > Logs | Token counts, costs, user, channel, model per request |
| Container logs | `docker logs cp-<service>` (e.g. `cp-nginx`, `cp-newapi`, `cp-bifrost`, `cp-clewdr`) | Raw application stdout/stderr per service |
| Cross-layer trace | `starapihub trace <request-id>` | Correlated logs across nginx / New-API / Bifrost / ClewdR for a single request |

## Gateway Request Logs (Dashboard)

This is the primary log source visible on the Dashboard Logs page. It shows every HTTP request that passes through the nginx gateway.

### Data Source

nginx access log at `/var/log/nginx/access.log`, written by the `cp-nginx` container.

### Data Flow

```
nginx access.log --> shared Docker volume (cp-nginx-logs)
                 --> dashboard poller (every 5 seconds)
                 --> SQLite (log_entries table)
                 --> /api/logs endpoint
                 --> Dashboard LogViewer
```

### nginx log_format

The nginx configuration defines the following log format (from `config/nginx/nginx.conf`):

```
log_format main '$remote_addr - $remote_user [$time_local] '
                '"$request" $status $body_bytes_sent '
                '"$http_referer" "$http_user_agent" '
                'req_id=$req_id '
                'upstream_time=$upstream_response_time';
```

### Parser Regex

The dashboard poller (in `dashboard/internal/poller/poller.go`) extracts fields using this regex:

```
^(\S+)\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+\S+"\s+(\d+)\s+req_id=(\S+)\s+upstream=(\S*)
```

### Fields Displayed on Dashboard

| Field | Source | Description |
|-------|--------|-------------|
| Timestamp | `$time_local` | When nginx received the request |
| Method | `$request` (1st token) | HTTP method (GET, POST, etc.) |
| Path | `$request` (2nd token) | Request URI path |
| Status | `$status` | HTTP response status code |
| Latency (ms) | Computed | End-to-end request time in milliseconds |
| Model | Extracted from path | AI model name (if inference request) |
| Request ID | `$req_id` | Unique identifier for cross-layer tracing |
| Upstream Time | `$upstream_response_time` | Time spent waiting for upstream backend |

### Retention

Logs are stored in a SQLite database at `/app/data/dashboard.db` (volume: `dashboard-data`). There is no automatic purging -- the database grows with usage. Operators should monitor disk usage if the appliance runs for extended periods under high traffic.

### Configuration

The nginx log file path is configurable via the `NGINX_LOG_PATH` environment variable (default: `/var/log/nginx/access.log`), set in `deploy/env/dashboard.env.example`.

## New-API Usage Logs

New-API maintains its own usage/billing logs in its internal database. These are a fundamentally different data source from gateway request logs:

- **Gateway logs** show _what happened at the network level_: which requests came in, how fast they were, what status code was returned.
- **Usage logs** show _who used what and how much it cost_: user identity, token name, model, prompt tokens, completion tokens, quota cost, channel.

### Access

Navigate to the New-API admin UI at `http://<host>:3000` (port 3000) and open the Logs section.

### Fields Available

User, token name, model, prompt tokens, completion tokens, quota cost, channel, timestamp.

## Container Logs

Each Docker service writes to stdout/stderr, accessible via standard Docker log commands:

```bash
docker logs cp-<service-name>
```

### Service Names

| Service | Container Name |
|---------|---------------|
| nginx | `cp-nginx` |
| New-API | `cp-newapi` |
| Bifrost | `cp-bifrost` |
| ClewdR | `cp-clewdr` |
| Dashboard | `cp-dashboard` |

Container logs are useful for debugging startup failures, crashes, and application-level errors not visible in gateway request logs.

## Infrastructure Details

### Docker Volume Mapping

The gateway request log pipeline depends on a shared Docker volume between the nginx and dashboard containers:

| Container | Volume Mount | Mode |
|-----------|-------------|------|
| nginx (`cp-nginx`) | `nginx-logs:/var/log/nginx` | read-write |
| dashboard (`cp-dashboard`) | `nginx-logs:/var/log/nginx:ro` | read-only |

**Volume name:** `cp-nginx-logs`

> **Note:** DEFECT-001 (Phase 17) was caused by this shared volume initially being misconfigured. The current setup is verified working -- the dashboard poller reads the nginx access log through the shared read-only mount every 5 seconds.
