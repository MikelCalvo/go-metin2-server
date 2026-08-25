# Kill-Quest-Only Drop-Table Authoring Example — 2026-08-20

## Objective

Land a deterministic checked-in authoring fixture for the already-owned kill-quest-only `drop_tables` path so manual QA and loopback validate coverage can prove gated kill-quest credit expansion without dummy EXP/gold/drop channels or item templates.

## Contract frozen by this slice

1. New fixture: `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json`
   - one gated kill-quest-only `drop_tables` row (`loot.qa_kill_quest_only`)
   - one referencing `spawn_groups` practice mob (`practice.qa_kill_quest_only_table_mob`)
   - one minimal `quest:first_steps.met_guide` `quest_flag` writer
   - no item templates and no combat reward channels
2. Canonicalization expands the table onto the spawn group, strips `drop_tables` / `reward_drop_table_ref`, and keeps the require gate + writer.
3. Focused tests:
   - `TestCanonicalizeKillQuestOnlyDropTableAuthoringExampleExpandsWithoutCombatChannels`
   - `TestLocalContentBundleValidateEndpointExpandsKillQuestOnlyDropTableAuthoringExample`
   - `TestGameRuntimeImportsKillQuestOnlyDropTableAuthoringExample`
4. Specs / QA checklist reference the fixture beside the combat+kill-quest authoring examples.

## What this is not yet

- randomized / weighted loot tables
- quest item rewards or turn-in item grants
- changing runtime death-reward execution beyond the already-owned empty-combat + kill-quest path
- SQL-backed content repositories

## TDD and validation

Focused coverage:

- `go test ./internal/contentbundle -run 'TestCanonicalizeKillQuestOnlyDropTableAuthoringExampleExpandsWithoutCombatChannels$' -count=1`
- `go test ./internal/ops -run 'TestLocalContentBundleValidateEndpointExpandsKillQuestOnlyDropTableAuthoringExample$' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. ~~Add a matching one-count `regen_spawns` kill-quest-only authoring fixture beside this spawn-group example.~~ Done in `2026-08-20-kill-quest-only-regen-authoring-example.md`.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Optionally migrate selected content-bundle / NPC gameplay tests onto hermetic MemoryStores once callers want less temp-dir coupling.
4. ~~Add a focused runtime-import twin for the checked-in kill-quest-only drop-table fixture.~~ Done:
   `TestGameRuntimeImportsKillQuestOnlyDropTableAuthoringExample`
   (`docs/plans/2026-08-25-kill-quest-only-authoring-runtime-import-twins.md`).
