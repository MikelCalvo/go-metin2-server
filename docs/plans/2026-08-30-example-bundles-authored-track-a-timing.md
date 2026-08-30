# Example Bundles Authored Track A Timing — 2026-08-30

## Objective

Compose the already-owned portable combat-profile Track A timing seams
(`chase_delay_ms`, `return_delay_ms`, `homeward_delay_ms`, `max_step`,
`reaction_delay_ms`) into the checked-in formula and PvE vertical example
bundles so manual QA and canonicalize proofs exercise non-default cadence /
step caps, not only bootstrap `5s` / `1s` / `100`.

This mirrors the earlier radii composition slice
(`2026-08-22-example-bundles-authored-aggro-leash-radii.md`) after the
runtime GREEN landings through `reaction_delay_ms`.

## Contract frozen by this slice

1. `docs/examples/bootstrap-combat-profile-formula-bundle.json` authors
   `qa_formula_practice_mob` with the existing formula/radius fields plus:
   - `chase_delay_ms = 2000`
   - `return_delay_ms = 2000`
   - `homeward_delay_ms = 2000`
   - `max_step = 50`
   - `reaction_delay_ms = 2000`
2. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` and its
   checked-in canonical twin author the same timing fields on
   `qa_pve_vertical_practice_mob`.
3. Focused canonicalize / validate / import tests assert those fields
   survive content-bundle expansion unchanged beside derived formula damage
   and authored radii.
4. Manual checklist §5.10.1 / §5.11 document the authored timing values and
   the retaliation / chase smoke expectations that differ from bootstrap
   defaults.

## Why these values

- `2000` ms chase delay stays above the owned `<= 1000` fail-closed floor so
  multi-beat hostility remains independently observable before the first chase
  step, while still being clearly shorter than bootstrap `5s` for QA timing.
- `2000` ms return / homeward / reaction delays stay inside the owned
  `250..60000` ms bounds and are clearly longer than bootstrap `1s`.
- `max_step = 50` is half of bootstrap `100`, so multi-step homeward / chase
  smoke visibly differs without inventing per-executor step fields.
- Built-in `practice_mob` / `training_dummy` and the byte-canonical
  `bootstrap-npc-service-bundle.json` stay on defaults; this slice only widens
  the formula / PvE authoring fixtures that already own custom profiles.

## What this is not yet

- pack AI / pathfinding / target switching
- aggro hysteresis / a drop radius distinct from the acquire radius
- cross-map return MOVE / `GC WARP` choreography
- absolute chase / return / homeward / reaction due-at rematerialize across
  daemon restart (cancelled as re-arm-from-now)
- changing built-in `practice_mob` defaults
- inventing new runtime executors beyond the already-owned Track A consumers

## TDD and validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/minimal/pve_vertical_authoring_test.go internal/minimal/shared_world_test.go
go test ./internal/contentbundle -run 'Test(CanonicalizeCombatProfileFormulaExampleDerivesDamageAndProfileReward|CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|CanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin)$' -count=1
go test ./internal/ops -run 'TestLocalContentBundleValidateEndpointExpands(CombatProfileFormulaExample|PveVerticalAuthoringExample)$' -count=1
go test ./internal/minimal -run 'Test(GameRuntimeImportsExampleFormulaCombatProfileBundleBeforeSpawnGroups|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1
git diff --check
```

## Follow-up options

1. Compose non-default `retaliation_point_delta` into the same fixtures only
   if manual QA still improvises stronger retaliation floors.
2. Keep hysteresis / pack AI / cross-map MOVE cancelled until a docs freeze
   names an honest client-visible contract.
