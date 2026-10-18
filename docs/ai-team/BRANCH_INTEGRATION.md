# Cherri — Branch Integration Manifest

**Source of Truth Base:** `agent/cherri-p10-apple-actions-coverage`  
**Setup & Coordination Branch:** `agent/team-orchestration-20261018` (PR #6)

## Active Work Branches (Targeted for Propagation)
- `agent/cherri-p10-apple-actions-coverage` (Current development base)
- `agent/team-orchestration-20261018` (Draft setup PR)
- `agent/cherri-v2-legacy-parity`
- `agent/cherri-v2-product-completion`
- `agent/ios27-shortcuts-runtime-poc`
- `agent/canonical-backend-production-cutover`
- `agent/shared-shortcuts-engine-v2-repair`

## Default Branch
- `main` — Receives additive team documentation only without merging feature code.

## Audited Exceptions (Skipped Archival / Snapshot Branches)
- `cloud-cherri-requests`
- `sync/upstream-2026-10`

## Future Branch Inheritance
All future branches created from `agent/cherri-p10-apple-actions-coverage` or `main` automatically inherit this configuration. Older branches can be checked with `tools/ai-team/ensure-team.ps1 -Check`.
