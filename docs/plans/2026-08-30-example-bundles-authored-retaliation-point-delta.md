# Example Bundles Authored Retaliation Point Delta — 2026-08-30

## Objective

Compose the already-owned portable combat-profile `retaliation_point_delta`
seam into the checked-in formula and PvE vertical example bundles so manual QA
and canonicalize proofs exercise a non-default owner-side hostility amount,
not only bootstrap `-1`.

This mirrors the earlier Track A timing composition slice
(`2026-08-30-example-bundles-authored-track-a-timing.md`) after runtime already
owns custom negative retaliation deltas.

## Contract frozen by this slice

1. `docs/examples/bootstrap-combat-profile-formula-bundle.json` authors
   `qa_formula_practice_mob` with the existing formula / radius / timing fields
   plus `retaliation_point_delta = -2`.
2. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` and its
   checked-in canonical twin author the same delta on
   `qa_pve_vertical_practice_mob`.
3. Focused canonicalize / validate / import / first-hit gameplay tests assert
   that field survives content-bundle expansion and that accepted live hits use
   the authored `-2` immediate retaliation amount.
4. Manual checklist §5.10.1 documents the authored delta beside radii / timing.

## Why `-2`

- Negative and stronger than bootstrap `-1`, so immediate and delayed beats are
  visibly different in QA without inventing a new hostility carrier.
- Still small enough that the composed PvE vertical four-hit kill remains
  honest for ordinary create-MaxHP owners before the floor.

## What this is not yet

- pack AI / pathfinding / target switching
- aggro hysteresis / a drop radius distinct from the acquire radius
- cross-map return MOVE / `GC WARP` choreography
- absolute chase / return / homeward / reaction due-at rematerialize across
  daemon restart (cancelled as re-arm-from-now)
- changing built-in `practice_mob` defaults
- inventing new runtime retaliation executors beyond the already-owned
  immediate + delayed seams

## TDD and validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/minimal/pve_vertical_authoring_test.go internal/minimal/shared_world_test.go
go test ./internal/contentbundle -run 'Test(CanonicalizeCombatProfileFormulaExampleDerivesDamageAndProfileReward|CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|CanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin)$' -count=1
go test ./internal/ops -run 'TestLocalContentBundleValidateEndpointExpands(CombatProfileFormulaExample|PveVerticalAuthoringExample)$' -count=1
go test ./internal/minimal -run 'Test(GameRuntimeImportsExampleFormulaCombatProfileBundleBeforeSpawnGroups|GameSessionFlowAuthoredFormulaCombatProfilePracticeMobUsesProfileMaxHPAndDamage|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1
git diff --check
```

## Follow-up options

1. Keep hysteresis / pack AI / cross-map MOVE cancelled until a docs freeze
   names an honest client-visible contract.
2. Prefer the next evidence-backed player-death / reward hardening gap over
   inventing broader formula work.
