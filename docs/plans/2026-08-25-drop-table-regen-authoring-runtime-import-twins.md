# Drop-Table / Regen Authoring Runtime Import Twins — 2026-08-25

## Objective

Close the remaining coverage gap for the checked-in combat+kill-quest
authoring fixtures: canonicalize and loopback validate already expand

- `docs/examples/bootstrap-drop-table-authoring-bundle.json`
- `docs/examples/bootstrap-regen-authoring-bundle.json`

but live runtime import of those exact JSON files was only covered indirectly
through inline Go structs / broader suites.

## Contract owned by this slice

1. `TestGameRuntimeImportsDropTableAuthoringExample` loads the checked-in
   drop-table authoring fixture from disk.
2. Runtime `ImportContentBundle(...)` strips `drop_tables` /
   `reward_drop_table_ref` and materializes one spawn-backed actor:
   - `practice.qa_reward_table_mob` / `QATableRewardMob`
   - EXP `75` / gold `60` / sorted drop vnums `27001,27002`
   - gated kill-quest credit + require gate from `loot.qa_reward`
3. `TestGameRuntimeImportsRegenAuthoringExample` loads the checked-in regen
   authoring fixture from disk.
4. Runtime `ImportContentBundle(...)` strips `regen_spawns` / `drop_tables` /
   `reward_drop_table_ref` and materializes one spawn-backed actor:
   - `practice.qa_regen_mob` / `QARegenMob`
   - EXP `90` / gold `45` / sorted drop vnums `27001,27002`
   - gated kill-quest credit + require gate from `loot.qa_regen_reward`
5. Both imports retain the in-bundle `quest:first_steps.met_guide` writer so the
   require gate stays valid.
6. Spec / QA / roadmap docs point at the focused runtime twins.

## Explicit non-goals

- weighted/random loot or branching quest scripts
- new NPC service kinds
- pack AI / synchronized respawn
- changing the already-owned canonicalize / ops validate twins
- changing kill-quest-only or multi-count runtime-import twins already landed

## Validation

```bash
gofmt -w internal/minimal/drop_table_regen_authoring_test.go
go test ./internal/minimal -run 'TestGameRuntimeImports(DropTable|Regen)AuthoringExample$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
