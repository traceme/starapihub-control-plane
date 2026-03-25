# Release Status — StarAPIHub

## Current State

| Field | Value |
|-------|-------|
| **Latest promoted release** | — (none yet; update after first promotion) |
| **Latest validated (not yet promoted)** | `0.2.0` (appliance + upstream) |
| **Latest failed release** | — |
| **Current nightly health** | Check: [nightly.yml runs](../../.github/workflows/nightly.yml) |

## Release History

| Version | Mode | Status | Promoted | Evidence | Notes |
|---------|------|--------|----------|----------|-------|
| `0.2.0` | appliance | Validated | No | `artifacts/releases/0.2.0/` | 20/20 gates passed |
| `0.2.0` | upstream | Validated | No | `artifacts/releases/0.2.0/` | 17/17 applicable gates passed |
| `0.1.0` | appliance | Validated | No | `artifacts/releases/0.1.0/` | Initial release candidate |

## How to Update This File

### After a new release workflow run:

1. Add a row to the Release History table
2. Set Status to `Validated` if all gates passed, `Failed` if any gate failed
3. Set Promoted to `No` initially

### After promotion:

1. Update the "Latest promoted release" field in Current State
2. Set Promoted to `Yes` and add the promotion date in Notes
3. Move previous promoted version's Promoted field to `Superseded`

### After a rollback:

1. Update the "Latest promoted release" to the rollback target
2. Mark the failed version's Status as `Rolled back`
3. Update "Latest failed release" in Current State
4. Add rollback date and reason in Notes

### After a nightly failure:

1. Do NOT change promotion status automatically
2. Investigate the failure
3. If rollback is needed, follow the rollback section above

## Image Digests

For current promoted image digests, check the `release-manifest.json` from the promoted version's release workflow artifacts.

Quick lookup:
```bash
# Find the promoted version's release workflow run
gh run list --workflow=release.yml --status=success --limit=5

# Download the manifest
gh run download <RUN_ID> --name release-<VERSION>-<MODE>
cat releases/<VERSION>/release-manifest.json | jq '.images'
```

## Nightly Health Trend

Check recent nightly runs:
```bash
gh run list --workflow=nightly.yml --limit=7
```

A green streak of ≥3 nights increases promotion confidence. See `promotion-criteria.md` for details.
