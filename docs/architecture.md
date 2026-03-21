# Architecture Overview

## Design Principle

This system follows a **control plane / data plane** separation:

- The **data plane** (request hot path) carries user inference traffic through upstream systems that we do not modify.
- The **control plane** (this project) manages configuration, policy, deployment, and operations externally.

## Four Layers

### Layer 1: New-API (Public Gateway)

**Owner of**: user accounts, tokens, balances, quotas, billing, model visibility, admin UI.

New-API is the only publicly exposed service. Clients authenticate here and send inference requests using logical model names. New-API routes these requests to backend "channels" which point to Bifrost.

**Key integration point**: New-API channels. Each channel has a type, base URL, API key, and model mapping. The control plane defines a small number of Bifrost-facing channels (e.g., `bifrost-premium`, `bifrost-standard`, `bifrost-risky`) rather than one channel per provider.

### Layer 2: Bifrost (Internal Router)

**Owner of**: provider selection, load balancing, retries, fallback, circuit breaking, caching, provider-side observability.

Bifrost receives requests from New-API and routes them to the appropriate provider pool. It handles multi-provider failover and can cache responses.

**Key integration point**: Bifrost config.json (or equivalent). The control plane generates provider definitions, pool assignments, and route rules that Bifrost loads at startup or via config reload.

### Layer 3: Providers (Inference Execution)

Two categories:

| Category | Examples | Trust Level |
|----------|----------|-------------|
| **Official** | OpenAI API, Anthropic API, AWS Bedrock, Google Vertex | High — billed, SLA-backed |
| **Unofficial** | ClewdR instances (Claude.ai proxy) | Low — cookie-based, no SLA, may break |

Official providers are accessed directly by Bifrost using API keys. ClewdR instances are accessed by Bifrost as OpenAI-compatible endpoints on the internal network.

### Layer 4: External Control Plane (This Project)

**Owner of**: logical model registry, route policy registry, deploy orchestration, config generation, sync guidance, smoke tests, runbooks.

The control plane never handles inference traffic. It operates on a separate schedule — typically during deploy, config change, or incident response.

## System Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │         EXTERNAL CONTROL PLANE              │
                    │  (config, policy, deploy, ops — this repo)  │
                    └──────┬──────────┬──────────────┬────────────┘
                           │ config   │ config       │ docs/guidance
                           │ + API    │ (file)       │
                           ▼          ▼              ▼
┌──────────┐    ┌──────────────┐  ┌──────────┐  ┌──────────────┐
│  Client   │───▶│   New-API    │──▶│ Bifrost  │──▶│  Providers   │
│  SDK/App  │    │  (public)    │  │(internal)│  │ (official +  │
└──────────┘    │  billing,    │  │ routing, │  │  ClewdR)     │
                │  auth, quota │  │ fallback │  └──────────────┘
                └──────────────┘  └──────────┘
```

## Network Zones

| Zone | Services | Access |
|------|----------|--------|
| `public` | Nginx/WAF, New-API (port 3000) | Internet-facing |
| `core` | Bifrost (port 8080) | Internal only — reachable from New-API |
| `provider` | ClewdR instances (port 8484 each), outbound to official APIs | Internal only — reachable from Bifrost |
| `ops` (optional) | Prometheus, Grafana, log aggregator | Operator access only |

## Loose Coupling Guarantees

1. **No source patches**: Upstream repos are never modified. All integration happens through external interfaces.
2. **Upstream upgradeable**: Any upstream version can be bumped by changing the image tag in docker-compose. No rebasing custom patches.
3. **Config is external**: All policy and routing config is defined in this project and pushed/synced into upstream systems.
4. **Replaceable components**: Any upstream system can theoretically be replaced by another implementation exposing compatible interfaces.
5. **Control plane is optional at runtime**: If this project's scripts/tools stop running, the deployed stack continues serving traffic with its last-applied config.

## Key Design Decisions

### Why channel-based routing through New-API?

New-API supports multiple "channels" — each is essentially a backend provider endpoint with an API key and model mapping. Instead of configuring one channel per provider (which duplicates Bifrost's job), we configure a small number of channels that point to Bifrost:

- `bifrost-premium` — routes through official-only pools
- `bifrost-standard` — routes through mixed pools with official preference
- `bifrost-risky` — routes through pools that may include ClewdR

This keeps New-API's channel config simple and lets Bifrost handle the complex routing decisions.

### Why not put the control plane on the hot path?

Adding another hop would increase latency, create a single point of failure, and require running a persistent service. The control plane's job is config management and operations — not request routing. The upstream systems already handle routing well.

### Why isolate ClewdR?

ClewdR proxies Claude.ai using browser cookies. This is:
- Not covered by any SLA
- Subject to rate limiting and blocking by Anthropic
- Potentially against terms of service

It must never be silently used for premium/paying traffic. It's useful for lab experiments, cost reduction on internal traffic, and as a last-resort fallback — but only when the operator explicitly opts in.
