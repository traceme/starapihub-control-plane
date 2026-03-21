# Network Topology

## Network Zones

The deployment uses four logical network zones to enforce security boundaries.

```
┌─────────────────────────────────────────────────────────────┐
│                        INTERNET                             │
└────────────────────────────┬────────────────────────────────┘
                             │ HTTPS (443)
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  PUBLIC ZONE (public-net)                                   │
│  ┌─────────────┐                                            │
│  │    Nginx     │ TLS termination, rate limiting, WAF       │
│  │   (WAF)      │ Only service with internet access         │
│  └──────┬───────┘                                            │
│         │ HTTP (3000)                                        │
│         ▼                                                    │
│  ┌─────────────┐                                            │
│  │   New-API   │ Auth, billing, quotas, admin UI            │
│  │   :3000     │ Public API surface                         │
│  └──────┬───────┘                                            │
└─────────┼───────────────────────────────────────────────────┘
          │ HTTP (8080)
          ▼
┌─────────────────────────────────────────────────────────────┐
│  CORE ZONE (core-net)                                       │
│  ┌─────────────┐                                            │
│  │   Bifrost   │ Routing, load balancing, failover          │
│  │   :8080     │ NOT publicly accessible                    │
│  └──────┬───────┘                                            │
└─────────┼───────────────────────────────────────────────────┘
          │ HTTP (8484, provider APIs)
          ▼
┌─────────────────────────────────────────────────────────────┐
│  PROVIDER ZONE (provider-net)                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  ClewdR #1  │  │  ClewdR #2  │  │  ClewdR #3  │         │
│  │  :8484      │  │  :8484      │  │  :8484      │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│  + outbound to official provider APIs (OpenAI, Anthropic…)  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  OPS ZONE (ops-net) — optional                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Prometheus  │  │   Grafana   │  │  Loki/Logs  │         │
│  │  :9090      │  │  :3001      │  │  :3100      │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

## Access Rules

| From | To | Allowed | Port |
|------|-----|---------|------|
| Internet | Nginx | Yes | 443 |
| Internet | New-API | **No** (through Nginx only) | — |
| Internet | Bifrost | **No** | — |
| Internet | ClewdR | **No** | — |
| Nginx | New-API | Yes | 3000 |
| New-API | Bifrost | Yes | 8080 |
| Bifrost | ClewdR instances | Yes | 8484 |
| Bifrost | Official APIs | Yes (outbound) | 443 |
| ClewdR | Claude.ai | Yes (outbound) | 443 |
| Ops tools | All services | Yes (metrics/logs) | Various |

## Docker Network Mapping

In `docker-compose.yml`, these zones map to Docker networks:

| Zone | Docker Network | Subnet |
|------|---------------|--------|
| public | `public-net` | 172.30.0.0/24 |
| core | `core-net` | 172.30.1.0/24 |
| provider | `provider-net` | 172.30.2.0/24 |
| ops | `ops-net` | 172.30.3.0/24 |

Services that bridge zones are attached to multiple networks:
- **Nginx**: `public-net`
- **New-API**: `public-net` + `core-net`
- **Bifrost**: `core-net` + `provider-net`
- **ClewdR**: `provider-net` only
- **Postgres/Redis**: `core-net` only (accessed by New-API)

## Secret Placement

| Secret | Stored With | NOT Stored In |
|--------|-------------|---------------|
| New-API DB credentials | New-API env / secrets | Bifrost, ClewdR, control plane |
| New-API session secret | New-API env | Bifrost, ClewdR, control plane |
| Official API keys (OpenAI, Anthropic, etc.) | Bifrost config.json or env | New-API, ClewdR, control plane |
| ClewdR admin password | ClewdR clewdr.toml or env | New-API, Bifrost, control plane |
| ClewdR browser cookies | ClewdR admin UI (stored in clewdr.toml) | New-API, Bifrost, control plane |
| TLS certificates | Nginx / cert volume | All other services |

The control plane may provide **templates** showing where each secret goes, but it does not centralize all secrets into one runtime store.

## DNS / Service Discovery

In Docker Compose, services use container names for internal DNS:

| Logical Name | Container DNS | Used By |
|-------------|---------------|---------|
| New-API | `new-api` | Nginx |
| Bifrost | `bifrost` | New-API (as channel base URL) |
| ClewdR #1 | `clewdr-1` | Bifrost (as provider endpoint) |
| ClewdR #2 | `clewdr-2` | Bifrost (as provider endpoint) |
| ClewdR #3 | `clewdr-3` | Bifrost (as provider endpoint) |
| PostgreSQL | `postgres` | New-API |
| Redis | `redis` | New-API |

In Kubernetes, replace container names with `<service-name>.<namespace>.svc.cluster.local`.
