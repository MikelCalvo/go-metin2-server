# Kill-Quest-Only Drop-Table Authoring — 2026-08-20

## Objective

Align authoring-only `drop_tables` validation with the already-owned spawn-group kill-quest rule so a complete kill-quest credit descriptor may stand alone without forcing dummy EXP/gold/drop channels.

## Contract frozen by this slice

1. `validDropTables` accepts a table when combat reward channels are empty **if and only if** the table carries a complete kill-quest credit descriptor (including optional require gate when present).
2. Completely empty tables (no combat channels and no kill-quest credit) remain rejected with `ErrInvalidBundle`.
3. Canonicalization expands kill-quest-only tables onto referencing `spawn_groups` / one-count `regen_spawns` the same way as combat+kill-quest tables, then strips `drop_tables` and `reward_drop_table_ref`.
4. Specs (`content-spawn-groups-bootstrap.md`, `quest-state-bootstrap.md`) document the kill-quest-only authoring path explicitly.

## What this is not yet

- randomized / weighted loot tables
- quest item rewards or turn-in item grants
- changing runtime death-reward execution beyond the already-owned empty-combat + kill-quest path
- new example fixtures (existing drop-table / regen / PvE authoring bundles remain combat+kill-quest examples)

## TDD and validation

Focused coverage:

- `go test ./internal/contentbundle -run 'TestCanonicalize(ExpandsKillQuestOnlyDropTable|ExpandsRegenSpawnKillQuestOnlyDropTable|RejectsEmptyDropTableWithoutKillQuestCredit)$' -count=1`
- `go test ./internal/contentbundle -run 'KillQuest|DropTable|RegenSpawn' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Wire hermetic `queststate.MemoryStore` into kill-quest / PvE gameplay tests that still use `FileStore`. Done in `2026-08-20-hermetic-queststate-memorystore-gameplay-tests.md`.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
