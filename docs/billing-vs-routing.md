# Billing vs Routing Responsibility Boundaries

## Core Principle

**New-API is the billing truth. Bifrost is the routing truth. The control plane is the policy truth.**

These three responsibilities must never be conflated. Each system owns exactly one concern.

## Responsibility Matrix

| Concern | Owner | NOT Owned By |
|---------|-------|-------------|
| User accounts, tokens, passwords | New-API | Bifrost, ClewdR, Control Plane |
| Balance, quota, rate limits | New-API | Bifrost, ClewdR, Control Plane |
| Token usage recording | New-API | Bifrost, Control Plane |
| Per-user model visibility | New-API | Bifrost, Control Plane |
| Price-per-token rates | New-API | Bifrost, Control Plane |
| Provider selection logic | Bifrost | New-API, Control Plane |
| Load balancing, retries, fallback | Bifrost | New-API, Control Plane |
| Circuit breaking | Bifrost | New-API, Control Plane |
| Provider health tracking | Bifrost | New-API, Control Plane |
| Inference execution | Providers / ClewdR | New-API, Bifrost, Control Plane |
| Logical model definitions | Control Plane | New-API, Bifrost |
| Route policy definitions | Control Plane | New-API, Bifrost |
| ClewdR isolation policy | Control Plane | New-API, Bifrost |
| Deploy orchestration | Control Plane | New-API, Bifrost, ClewdR |

## How Billing Works

1. Client sends request to New-API with `Authorization: Bearer sk-xxx`
2. New-API authenticates the token, checks the user's balance/quota
3. New-API proxies the request through a channel (to Bifrost)
4. When the response returns, New-API counts input/output tokens
5. New-API debits the user's balance at the configured price-per-token rate
6. The billing record is associated with the user, token, and model name

**What New-API does NOT know**: which actual provider fulfilled the request. New-API only knows it sent the request to a channel (Bifrost endpoint) and got a response back.

**Implication**: Token pricing in New-API should be based on the logical model and channel tier, not the actual provider. A request routed through `bifrost-premium` may cost more than one through `bifrost-standard`, regardless of which provider Bifrost selected.

## How Routing Works

1. Bifrost receives a request (from New-API) with a model name
2. Bifrost matches the model name to a route
3. The route specifies a pool (or pool chain with fallback order)
4. Bifrost selects a provider from the pool using its load-balancing algorithm
5. Bifrost handles retries and fallback across pools if the first attempt fails
6. Bifrost returns the response (or error) to New-API

**What Bifrost does NOT know**: who the end user is, what their balance is, or what they should be charged. Bifrost only knows about providers, pools, and routes.

## How Policy Works

The control plane defines:

1. **Logical models** — e.g., `claude-sonnet` maps to New-API channel `bifrost-premium` and Bifrost route `premium-claude-sonnet`
2. **Route policies** — e.g., `premium` policy means official providers only, no ClewdR
3. **Provider pools** — e.g., `official-claude` pool contains only Anthropic API keys; `risky-claude` pool may include ClewdR

The control plane generates config/documentation that operators use to configure New-API and Bifrost. The control plane never directly handles billing or routing at runtime.

## Pricing Strategy

### Recommended Approach

Set New-API token prices based on the **channel tier**, not the actual provider:

| Channel (New-API) | Bifrost Route | Expected Provider | Price Factor |
|-------------------|---------------|-------------------|-------------|
| `bifrost-premium` | premium routes | Official APIs only | 1.0x (market rate) |
| `bifrost-standard` | standard routes | Official preferred, unofficial fallback | 0.7x |
| `bifrost-risky` | risky routes | May use ClewdR | 0.3x |

This keeps billing simple and predictable. The operator accepts that:
- Premium traffic always costs market rate because it always uses official providers
- Standard traffic is cheaper because it may occasionally use unofficial providers
- Risky traffic is cheapest because it routinely uses unofficial providers

### What NOT To Do

- Do NOT try to dynamically adjust billing based on which provider Bifrost actually selected — this would require coupling billing logic to routing decisions, violating the separation principle.
- Do NOT move billing logic into the control plane — New-API already handles this well.
- Do NOT expose ClewdR costs as "zero" to users — this encourages dependence on an unreliable provider.

## Audit Trail

To verify that billing and routing are aligned:

1. **New-API side**: Query usage records — shows user, token, model, channel, tokens consumed, amount charged
2. **Bifrost side**: Query routing logs — shows model, route, pool, provider selected, latency, status
3. **Correlation**: Match by timestamp + model + approximate request identity (or `X-Request-ID` if supported)

If New-API shows charges for `claude-sonnet` through `bifrost-premium`, but Bifrost logs show that traffic was routed through a ClewdR-containing pool, that's a policy violation that needs investigation.
