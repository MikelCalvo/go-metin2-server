# Kill-Quest-Only Regen Authoring Example — 2026-08-20

## Objective

Land a deterministic checked-in authoring fixture for the already-owned kill-quest-only `drop_tables` path when the referencing actor is authored through one-count `regen_spawns`, so regen authoring QA can prove gated kill-quest credit expansion without dummy EXP/gold/drop channels or item templates.

## Contract frozen by this slice

1. New fixture: `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json`
   - one gated kill-quest-only `drop_tables` row (`loot.qa_kill_quest_only_regen`)
   - one referencing one-count `regen_spawns` practice mob (`practice.qa_kill_quest_only_regen_mob`)
   - one minimal `quest:first_steps.met_guide` `quest_flag` writer
   - no item templates and no combat reward channels
2. Canonicalization expands the table onto the regen spawn, strips `regen_spawns` / `drop_tables` / `reward_drop_table_ref`, and keeps the require gate + writer.
3. Focused tests:
   - `TestCanonicalizeKillQuestOnlyRegenAuthoringExampleExpandsWithoutCombatChannels`
   - `TestLocalContentBundleValidateEndpointExpandsKillQuestOnlyRegenAuthoringExample`
   - `TestGameRuntimeImportsKillQuestOnlyRegenAuthoringExample`
4. Specs / QA checklist reference the fixture beside the spawn-group kill-quest-only and combat+kill-quest regen authoring examples.

## What this is not yet

- multi-count / pack regen authoring
- randomized / weighted loot tables
- changing runtime death-reward execution beyond the already-owned empty-combat + kill-quest path
- SQL-backed content repositories

## TDD and validation

Focused coverage:

- `go test ./internal/contentbundle -run 'TestCanonicalizeKillQuestOnlyRegenAuthoringExampleExpandsWithoutCombatChannels$' -count=1`
- `go test ./internal/ops -run 'TestLocalContentBundleValidateEndpointExpandsKillQuestOnlyRegenAuthoringExample$' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. ~~Keep multi-count regen expansion deferred until pack placement / member identity rules are frozen.~~ Docs/spec freeze and authoring GREEN landed: see [multi-count regen pack placement contract freeze](2026-08-23-multi-count-regen-pack-placement-contract-freeze.md) plus `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. ~~Add a focused runtime-import twin for the checked-in kill-quest-only regen fixture.~~ Done:
   `TestGameRuntimeImportsKillQuestOnlyRegenAuthoringExample`
   (`docs/plans/2026-08-25-kill-quest-only-authoring-runtime-import-twins.md`).
