# Unofficial Provider Risk Assessment

## What Is an Unofficial Provider?

An unofficial provider is any service that proxies AI model access through means not sanctioned by the model vendor. In this architecture, **ClewdR** is the primary unofficial provider — it proxies Claude through Claude.ai browser sessions using stolen/exported cookies.

## Risk Matrix

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Cookie expiration | Medium | High (daily-weekly) | Automated health checks, multiple instances, rotation runbook |
| Anthropic account suspension | High | Medium | Separate accounts per instance, monitor for warnings |
| Terms of service violation | High | Certain | Legal review, isolate from premium billing, document risk acceptance |
| Response quality degradation | Medium | Low | Compare responses against official API periodically |
| Data privacy concerns | High | Low-Medium | Do not send PII through unofficial providers |
| Rate limiting | Medium | High | Multiple instances, load distribution, backoff |
| Service discontinuation | High | Medium | Keep official provider pools as fallback, never depend solely on ClewdR |

## Isolation Rules

1. **Network isolation**: ClewdR runs in the `provider-net` zone, unreachable from the public internet.
2. **Pool isolation**: ClewdR instances are grouped in `unofficial-clewdr` pool, never mixed with official pools.
3. **Policy isolation**: Only `standard` (fallback) and `risky` (primary) route policies include the ClewdR pool.
4. **Billing isolation**: Traffic through ClewdR-eligible channels is priced differently than premium channels.
5. **User isolation**: Lab/risky models are only visible to explicitly authorized user groups.

## When to Use ClewdR

Appropriate:
- Internal development and testing
- Lab experiments where cost matters more than reliability
- As a last-resort fallback when official providers are experiencing outages
- Non-production workloads

Not appropriate:
- Customer-facing production traffic (use premium policy)
- Compliance-sensitive workloads
- Workloads requiring SLA guarantees
- Workloads processing PII or sensitive data
