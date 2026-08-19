# Account Character-State Repository Seam — 2026-08-19

## Objective

Land the first narrow repository-style seam for durable PvE character state already projected onto migrations `0002` / `0003` / `0011`, so export and future backfill tooling can target named interfaces instead of anonymous `FileStore` type asserts, and hermetic tests can prove reduced bootstrap filesystem coupling without claiming a DB-backed runtime.

## Contract frozen by this slice

1. `accountstore.AccountLister` exposes `List() ([]Account, error)`.
2. `accountstore.AccountCharacterStateExporter` exposes:
   - `ExportAccountCharacterRoster() (AccountCharacterRosterExport, error)`
   - `ExportCharacterItemState() (CharacterItemStateExport, error)`
   - `ExportCharacterPointState() (CharacterPointStateExport, error)`
3. `FileStore` continues to satisfy `Store`, `AccountLister`, and `AccountCharacterStateExporter` with no disk-path behavior change.
4. New hermetic `MemoryStore` also satisfies those three interfaces from an in-memory account map:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies characters on Load/Save/List
   - deliberately omits backup / restore / crash-temp cleanup
5. `gameRuntime.ExportAccountCharacterRoster` / `ExportCharacterItemState` / `ExportCharacterPointState` assert the named `AccountCharacterStateExporter` interface instead of anonymous method sets.
6. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** add import/backfill into live stores or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- broadening `Store` into backup/restore/crash-temp
- quest-state / login-ticket / item-template / static-content / ground-item repository seams
- ground-item restart durability
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/accountstore -run 'MemoryStore|AccountCharacterStateExporter' -count=1`
- `go test ./internal/minimal -run 'ExportsAccountCharacterStateThroughMemoryStoreSeam|AccountCharacterRosterExport|CharacterItemStateExport|CharacterPointStateExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add matching hermetic seams for quest-state / login-ticket / item-template / static-content exports once callers need the same coupling reduction.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep ground-item restart durability deferred until a world-state repository exists.
