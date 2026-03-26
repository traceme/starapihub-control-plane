# External Control Plane for New-API + Bifrost + ClewdR

This project is an **external integration and control plane** that deploys, connects, and operates three upstream AI gateway systems.

Default mode is still **external integration first**. For the stricter commercial-appliance target, the preferred rule is:

- use APIs, config, env, and orchestration first
- allow only a **tiny, explicit, justified** upstream patch set when commercial requirements cannot be met externally

## Upstream Systems

| System | Role | Exposure | Source |
|--------|------|----------|--------|
| **New-API** | Public API gateway, user management, billing, quotas, admin UI | Public (northbound) | `../new-api/` |
| **Bifrost** | Provider routing, load balancing, retries, fallback, caching | Internal only | `../bifrost/` |
| **ClewdR** | Claude proxy (unofficial), OpenAI-compatible endpoints | Internal only, isolated | `../clewdr/` |

## Hard Rules

1. **No broad source divergence** in New-API, Bifrost, or ClewdR — they are treated as upstream vendor projects.
2. **Hot path stays clean**: `Client -> New-API -> Bifrost -> ClewdR/Official Providers`. The control plane is never on the inference path.
3. **ClewdR is isolated**: never silently mixed into premium traffic, only used in explicit risky/lab/fallback pools.
4. **Billing truth lives in New-API**, routing truth lives in Bifrost, policy truth lives here.
5. **If upstream patches exist, they must be minimal and documented** in the appliance patch inventory.

## Request Path vs Control Path

```
REQUEST PATH (hot, latency-sensitive):
  Client/SDK -> [Public Domain] -> New-API -> Bifrost -> Provider (Official API / ClewdR)

CONTROL PATH (cold, operator-driven):
  This Project -> generates config/templates/policies
                -> calls admin APIs where available
                -> writes mounted config files
                -> orchestrates deploy and rollout
                -> collects health and observability data
```

## Directory Structure

```
control-plane/
├── README.md                  # This file
├── docs/                      # Architecture, integration, and operational docs
│   ├── architecture.md        # 4-layer architecture, hot path vs control path
│   ├── request-path.md        # Client->New-API->Bifrost->Provider flow
│   ├── billing-vs-routing.md  # New-API=billing, Bifrost=routing, CP=policy
│   ├── network-topology.md    # Network zones (public/core/provider/ops)
│   ├── policies.md            # Logical model, route policy, provider pool registries
│   ├── new-api-integration.md # Channel strategy, model visibility, admin steps
│   ├── bifrost-integration.md # Provider registry, pools, routing rules
│   ├── clewdr-operations.md   # Instance isolation, cookies, health checks
│   ├── config-sync.md         # Generated config, sync scripts, operator workflows
│   ├── verification.md        # Smoke test descriptions and what each validates
│   ├── observability.md       # Request correlation and monitoring
│   ├── runbook.md             # Startup, shutdown, rotation, incident response
│   ├── failure-drills.md      # Component failure scenarios and expected behavior
│   ├── upgrade-strategy.md    # Base upgrade workflow for upstream releases
│   ├── version-matrix.md      # Appliance-to-upstream compatibility matrix
│   ├── patch-audit-workflow.md # Required review flow before any upstream patch
│   ├── rollout-plan.md        # Phased rollout from dev to production
│   ├── unofficial-provider-risk.md  # ClewdR risk assessment
│   ├── openrouter-operations.md     # OpenRouter setup, sync, rotation, troubleshooting
│   ├── provider-model-mapping.md    # Three-layer model name contract (public/channel/provider)
│   ├── promotion-criteria.md        # Release promotion gates and evidence
│   ├── rollback-runbook.md          # Upstream and appliance rollback procedures
│   ├── release-status.md            # Current promoted/validated/failed versions
│   ├── provider-onboarding.md       # Repeatable provider onboarding checklist
│   ├── provider-secrets.md          # Credential map, rotation, backup, recovery
│   ├── provider-verification.md     # Provider coverage matrix, failure classification
│   ├── install.md                   # Canonical first-install guide (start here)
│   ├── secrets-bootstrap.md         # First-run secret prerequisites and validation
│   ├── backup-restore.md            # Backup scope, restore procedures, drill checklist
│   ├── day-2-operations.md          # Daily/weekly checklists, failure triage, escalation
│   └── alert-model.md              # Severity definitions and signal catalog (v1.8)
├── deploy/                    # Deployment skeletons
│   ├── docker-compose.yml     # Full stack compose
│   ├── README.md              # Deployment guide
│   └── env/                   # Environment variable templates
├── config/                    # Upstream-facing config templates
│   ├── nginx/                 # Public ingress configuration
│   ├── new-api/               # New-API channel/model config guidance
│   ├── bifrost/               # Bifrost provider/route config templates
│   └── clewdr/                # ClewdR instance config guidance
├── policies/                  # External policy registries
│   ├── logical-models.example.yaml
│   ├── route-policies.example.yaml
│   └── provider-pools.example.yaml
├── scripts/                   # Operational scripts
│   ├── starapihub-cron.sh     # Cron alert wrapper (install to /usr/local/bin/)
│   ├── smoke/                 # Smoke test scripts
│   └── sync/                  # Config generation and sync helpers
└── tests/                     # Test documentation and fixtures
    └── smoke/                 # Smoke test docs
```

## Quick Start

**New operators: start with [`docs/install.md`](docs/install.md)** for the canonical end-to-end install path.

For deeper context:

1. Read `docs/architecture.md` for the 4-layer architecture and design decisions.
2. Read `docs/rollout-plan.md` for the phased deployment approach.
3. Copy and customize `deploy/env/*.env.example` files.
4. Review `policies/` and adjust logical models and route policies.
5. Run `deploy/docker-compose.yml` to bring up the stack.
6. Follow `docs/config-sync.md` to push config into upstream systems.
7. Run `scripts/smoke/run-all.sh` to verify (see `docs/verification.md` for test details).
8. Read `docs/runbook.md` for day-to-day operations.
9. For the commercial-appliance mode, also read `docs/version-matrix.md`, `docs/patch-audit-workflow.md`, and the root-level upgrade/patch docs.

## Integration Approach

This project integrates upstream systems exclusively through:

- **HTTP APIs** — admin endpoints where available (New-API channel management, etc.)
- **Configuration files** — mounted into upstream containers
- **Environment variables** — passed via compose/k8s env
- **Reverse proxy** — nginx routing and header injection
- **Deployment orchestration** — compose/k8s manifests with network segmentation
- **Health checks** — HTTP probes on upstream health endpoints
- **Observability** — log aggregation and request correlation headers

Where an upstream system lacks a needed API or config surface, this project should first provide the best external workaround.

For the commercial-appliance target only, if a requirement such as auditability or near-zero manual ops still cannot be reached, the project may introduce a **small upstream patch**. Any such patch must stay:

- isolated
- reviewable
- plausibly upstreamable
- explicitly tracked in the patch inventory
