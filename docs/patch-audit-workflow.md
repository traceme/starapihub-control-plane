# Patch Audit Workflow

## Purpose

This workflow is mandatory before changing any upstream source tree for the commercial-appliance variant.

It exists to stop casual vendor drift.

## When to Use It

Run this workflow if a proposed change touches:

- `new-api/`
- `bifrost/`
- `clewdr/`

Do not skip it just because the change seems small.

## Decision Rule

Default answer: **do not patch upstreams**.

A patch is allowed only if:

1. the requirement matters to the commercial appliance
2. existing APIs/config/env/reverse-proxy options were seriously evaluated
3. the remaining gap is real
4. the patch is narrow and maintainable

## Audit Steps

### Step 1: Name the Requirement

State the exact requirement being missed.

Good examples:

- deterministic request correlation across layers
- machine-operable sync endpoint to remove manual UI dependency
- machine-readable health/status needed for automation

Bad examples:

- cleaner code
- nicer admin UI
- easier than writing a script

### Step 2: Record External Alternatives

Document what was tried or evaluated through:

- existing upstream APIs
- config files
- env vars
- reverse proxy behavior
- control-plane automation

If this section is weak, the patch is not ready.

### Step 3: Bound the Patch

State:

- subsystem
- files likely to change
- expected behavior change
- expected test coverage

If the likely blast radius is unclear, stop.

### Step 4: Estimate Upgrade Risk

Classify the patch:

- `low`: tiny protocol/metadata change
- `medium`: moderate handler or API surface change
- `high`: deep logic, wide file spread, schema churn

High-risk patches should normally be rejected.

### Step 5: Check Upstreamability

Ask:

- would upstream maintainers plausibly accept this?
- is the behavior generally useful, not appliance-only glue?

If the answer is clearly no, the patch needs extra scrutiny.

### Step 6: Write It Down

Before implementation, create or update the corresponding entry in:

- [upstream-patches.md](/Users/mac/projects/OpenRouterAround/starapihub/docs/upstream-patches.md)

At minimum mark it as `proposed`.

### Step 7: Implement Minimally

When approved:

- touch the fewest files possible
- preserve upstream conventions
- avoid opportunistic refactors
- add tests where practical

### Step 8: Verify and Reclassify

After implementation:

- verify the behavior
- update the patch inventory entry to `active`
- update [version-matrix.md](/Users/mac/projects/OpenRouterAround/starapihub/control-plane/docs/version-matrix.md) when the patched combination is validated

## Rejection Conditions

Reject the patch if:

- it avoids building straightforward external automation
- it touches too many files
- it changes core routing/billing behavior unnecessarily
- it creates UI-heavy divergence
- it does not materially improve the commercial appliance

## Minimal Patch Checklist

- [ ] Requirement is explicit
- [ ] External alternatives were evaluated
- [ ] Files touched are bounded
- [ ] Upgrade risk is acceptable
- [ ] Upstreamability was considered
- [ ] Patch inventory updated
- [ ] Verification defined

If any box is not checked, do not patch upstreams yet.
