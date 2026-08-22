# Pickup Empire Restriction Matrix Docs — 2026-08-22

## Objective

Name empire anti-flags (`anti_empire_a` / `anti_empire_b` / `anti_empire_c`) beside the already-owned job/sex/`min_level` pickup restriction matrix in protocol/QA, and freeze focused player-helper coverage for those empire cases so the authored pickup restriction list matches runtime behavior (`CanUseTemplate` already rejects them).

## Contract owned by this slice

1. `item-drop-pickup-bootstrap.md` bullet for template-backed pickup placement explicitly lists empire anti-flags beside job/sex/`min_level`.
2. QA checklist pickup restriction wording includes empire beside job/sex/`min_level`.
3. `TestPickupGroundItemWithTemplateRejectsAuthoredRestrictionsWithoutMutation` covers `anti_empire_a` / `anti_empire_b` / `anti_empire_c` fail-closed with no inventory mutation (mirroring drop/use coverage).
4. No runtime behavior change beyond making the already-enforced empire path explicit in tests/docs.

## What this is not yet

- new pickup reject chat wording
- party-shaped owner-delivery
- exchange id-collision / restriction finalize reject chat
- durable safebox persistence

## TDD and validation

- `go test ./internal/player -run 'PickupGroundItemWithTemplateRejectsAuthoredRestrictionsWithoutMutation' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally add dual-sided chat for exchange finalize id-collision / selected-character / transfer-guard restriction rejects once QA wants those distinguishable from silent fail-closed.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
3. Keep party-shaped owner-delivery deferred.

## Status

Shipped: empire anti-flags are named beside job/sex/`min_level` in pickup protocol/QA, and the player pickup restriction matrix covers `anti_empire_a` / `anti_empire_b` / `anti_empire_c` fail-closed with no inventory mutation. Runtime behavior was already owned through `CanUseTemplate`; this slice only freezes the matrix/docs contract.
