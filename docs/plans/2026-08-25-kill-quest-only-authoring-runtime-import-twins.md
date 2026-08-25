# Kill-Quest-Only Authoring Runtime Import Twins — 2026-08-25

## Objective

Close the remaining coverage gap for the checked-in kill-quest-only authoring
fixtures: canonicalize and loopback validate already expand

- `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json`
- `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json`

but live runtime import of those exact JSON files was only covered indirectly
through inline Go structs / broader suites.

## Contract owned by this slice

1. `TestGameRuntimeImportsKillQuestOnlyDropTableAuthoringExample` loads the
   checked-in drop-table authoring fixture from disk.
2. Runtime `ImportContentBundle(...)` strips `drop_tables` / `reward_drop_table_ref`
   and materializes one spawn-backed actor:
   - `practice.qa_kill_quest_only_table_mob` / `QAKillQuestOnlyTableMob`
   - empty EXP/gold/drop channels
   - gated kill-quest credit + require gate from `loot.qa_kill_quest_only`
3. `TestGameRuntimeImportsKillQuestOnlyRegenAuthoringExample` loads the
   checked-in regen authoring fixture from disk.
4. Runtime `ImportContentBundle(...)` strips `regen_spawns` / `drop_tables` /
   `reward_drop_table_ref` and materializes one spawn-backed actor:
   - `practice.qa_kill_quest_only_regen_mob` / `QAKillQuestOnlyRegenMob`
   - empty EXP/gold/drop channels
   - gated kill-quest credit + require gate from `loot.qa_kill_quest_only_regen`
5. Both imports retain the in-bundle `quest:first_steps.met_guide` writer so the
   require gate stays valid.
6. Spec / QA / authoring plans point at the focused runtime twins.

## Explicit non-goals

- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / ops validate twins
- pack AI / synchronized respawn

## Validation

```bash
gofmt -w internal/minimal/kill_quest_only_authoring_test.go
go test ./internal/minimal -run 'TestGameRuntimeImportsKillQuestOnly(DropTable|Regen)AuthoringExample$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
