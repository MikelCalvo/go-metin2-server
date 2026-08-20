# Auth Login-Ticket Handoff Repository Seam — 2026-08-20

## Objective

Land the third narrow repository-style seam for durable authd-to-gamed login-ticket handoff state already projected onto migration `0007_auth_login_ticket_handoff`, so export and future backfill tooling can target a named interface instead of anonymous `FileStore` type asserts, and hermetic tests can prove reduced bootstrap filesystem coupling without claiming a DB-backed runtime.

## Contract frozen by this slice

1. `loginticket.AuthLoginTicketHandoffExporter` exposes:
   - `ExportAuthLoginTicketHandoff() (AuthLoginTicketHandoffExport, error)`
2. `FileStore` continues to satisfy `Store` and `AuthLoginTicketHandoffExporter` with no disk-path behavior change.
3. New hermetic `MemoryStore` also satisfies those two interfaces from an in-memory pending-ticket map:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies tickets/characters on Issue/Load/Consume/List/Export
   - empty pending set returns an empty migration-shaped export
   - deliberately omits backup / restore / crash-temp / issued-before cleanup
4. `MemoryStore` owns hermetic `Issue` / `Load` / `Consume` / `List` so auth handoff tests can exercise the one-shot ticket primitive without a disk-backed store.
5. `gameRuntime.ExportAuthLoginTicketHandoff` asserts the named `AuthLoginTicketHandoffExporter` interface instead of an anonymous method set.
6. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** add import/backfill into live stores or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- broadening `Store` into backup/restore/crash-temp
- item-template / static-content / ground-item repository seams
- ground-item restart durability
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/loginticket -run 'MemoryStore|AuthLoginTicketHandoffExporter' -count=1`
- `go test ./internal/minimal -run 'ExportsAuthLoginTicketHandoffThroughMemoryStoreSeam|AuthLoginTicketHandoffExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add matching hermetic seams for static-content exports once callers need the same coupling reduction. Item-template state now has its own `ItemTemplateStateExporter` + hermetic `MemoryStore`.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep ground-item restart durability deferred until a world-state repository exists.
