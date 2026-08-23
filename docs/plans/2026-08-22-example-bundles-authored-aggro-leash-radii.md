# Example Bundles Authored Aggro/Leash Radii — 2026-08-22

## Objective

Compose the already-owned portable `combat_profiles.aggro_radius` /
`leash_radius` seam into the checked-in formula and PvE vertical example
bundles so manual QA and canonicalize proofs exercise non-default acquire /
leash radii, not only bootstrap `200` / `400`.

## Contract frozen by this slice

1. `docs/examples/bootstrap-combat-profile-formula-bundle.json` authors
   `qa_formula_practice_mob` with:
   - existing formula fields (`max_hp = 20`, `attack_value = 9`,
     `defense_value = 4`, `respawn_delay_ms = 2000`, profile death reward)
   - `aggro_radius = 150`
   - `leash_radius = 350`
2. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` authors the
   same radii on `qa_pve_vertical_practice_mob`.
3. Focused canonicalize / validate / import tests assert those radii survive
   content-bundle expansion unchanged beside derived formula damage.
4. Manual checklist §5.10.1 documents the authored radii and the proximity /
   leash smoke expectations that differ from bootstrap defaults.

## Why these values

- `150` is clearly inside bootstrap default aggro (`200`) so QA can prove a
  narrowed acquire radius without needing a wider leash first.
- `350` stays below bootstrap default leash (`400`) and above authored aggro,
  satisfying the owned `leash >= effective aggro` validation rule.
- Built-in `practice_mob` / `training_dummy` and the byte-canonical
  `bootstrap-npc-service-bundle.json` stay on defaults; this slice only widens
  the formula / PvE authoring fixtures that already own custom profiles.

## What this is not yet

- multi-count `regen_spawns` / pack placement
- weighted/random loot
- hysteresis / separate drop radius
- cross-map return MOVE choreography
- changing built-in `practice_mob` defaults

## TDD and validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/minimal/pve_vertical_authoring_test.go internal/minimal/shared_world_test.go
go test ./internal/contentbundle -run 'Test(CanonicalizeCombatProfileFormulaExampleDerivesDamageAndProfileReward|CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop)$' -count=1
go test ./internal/ops -run 'TestLocalContentBundleValidateEndpointExpands(CombatProfileFormulaExample|PveVerticalAuthoringExample)$' -count=1
go test ./internal/minimal -run 'Test(GameRuntimeImportsExampleFormulaCombatProfileBundleBeforeSpawnGroups|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Add a checked-in negative invalid-bundle fixture for orphan quest gates /
   `count != 1` regen rows if QA still improvises reject cases.~~ Done: see
   [checked-in invalid content-bundle fixtures](2026-08-23-checked-in-invalid-content-bundle-fixtures.md).
2. ~~Freeze multi-count regen pack placement before widening `regen_spawns.count`.~~
   Done for docs/spec and authoring GREEN: see
   [multi-count regen pack placement contract freeze](2026-08-23-multi-count-regen-pack-placement-contract-freeze.md)
   and
   [multi-count regen pack placement authoring](2026-08-23-multi-count-regen-pack-placement-authoring.md).
