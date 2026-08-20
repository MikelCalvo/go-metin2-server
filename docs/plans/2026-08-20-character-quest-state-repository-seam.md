# Character Quest-State Repository Seam — 2026-08-20

## Objective

Land the second narrow repository-style seam for durable PvE quest-flag state already projected onto migration `0004_character_quest_state`, so export and future backfill tooling can target a named interface instead of anonymous `FileStore` type asserts, and hermetic tests can prove reduced bootstrap filesystem coupling without claiming a DB-backed runtime.

## Contract frozen by this slice

1. `queststate.CharacterQuestStateExporter` exposes:
   - `ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error)`
2. `FileStore` continues to satisfy `Store` and `CharacterQuestStateExporter` with no disk-path behavior change.
3. New hermetic `MemoryStore` also satisfies those two interfaces from an in-memory snapshot:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies flags on Load/Save/Export
   - missing committed snapshot returns `ErrSnapshotNotFound` from `Load` and an empty migration-shaped export from `ExportCharacterQuestState`
   - deliberately omits backup / restore / crash-temp cleanup
4. `MemoryStore` also owns hermetic `ApplyTransition` / `PreviewTransition` so NPC/kill-quest gameplay tests can exercise the same compare-and-set primitive without a disk-backed snapshot.
5. `gameRuntime.ExportCharacterQuestState` asserts the named `CharacterQuestStateExporter` interface instead of an anonymous method set.
6. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** add import/backfill into live stores or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- broadening `Store` into backup/restore/crash-temp
- login-ticket / item-template / static-content / ground-item repository seams
- ground-item restart durability
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/queststate -run 'MemoryStore|CharacterQuestStateExporter' -count=1`
- `go test ./internal/minimal -run 'ExportsCharacterQuestStateThroughMemoryStoreSeam|CharacterQuestStateExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add matching hermetic seams for item-template / static-content exports once callers need the same coupling reduction. Login-ticket handoff now has its own `AuthLoginTicketHandoffExporter` + hermetic `MemoryStore`.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep ground-item restart durability deferred until a world-state repository exists.
