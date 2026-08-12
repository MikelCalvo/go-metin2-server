# Auth Login-Ticket Handoff Migration — 2026-08-12

## Objective

Freeze the first schema-only database boundary for the authd-to-gamed login-ticket handoff without changing the shipped runtime away from its current file-backed ticket store.

The existing bootstrap runtime still issues one-shot JSON tickets from `authd` and validates them on `gamed`. This slice gives future DB/backfill tooling an explicit durable contract for that handoff state while keeping migration execution, DB repositories, and live runtime DB writes out of scope.

## Contract frozen by this slice

The embedded `db/migrations` catalog now includes `0007_auth_login_ticket_handoff`.

The `up` migration creates `auth_login_tickets` with:

- `login_key` as the non-zero `uint32`-bounded handoff key returned in `AUTH_SUCCESS` and supplied by the client in `LOGIN2`,
- `issued_at` as the non-empty timestamp boundary already required by committed JSON tickets and stale-ticket cleanup preflights,
- `login` and `login_normalized` for deterministic account lookup and future backfill checks,
- `empire` to preserve the auth-ticket selection context,
- nullable `consumed_at` to model the destructive one-shot consume boundary without deleting historical rows,
- `characters_snapshot_json` as an explicit transitional payload for the current select-screen character snapshot handoff,
- created/updated timestamps,
- a primary key on `(login_key, issued_at)`,
- a partial unique index on active `login_key` rows where `consumed_at IS NULL`,
- active-login and issued-at indexes for operator/preflight queries.

The `down` migration drops the indexes and table. The migration is manifest-pinned like the rest of the catalog, so historical SQL drift fails closed.

## What this is not yet

This slice does not:

- make login tickets DB-backed at runtime,
- change the current JSON ticket issue/load/consume behavior,
- add migration apply/rollback tooling,
- add a DB driver dependency or production DB engine,
- introduce a repository implementation for auth tickets,
- replace destructive file-ticket consume semantics with SQL updates.

The table is intentionally a contract/backfill target first. A later repository slice can decide whether `Consume` becomes a `consumed_at` update, a delete, or a stricter transaction boundary once a real DB-backed handoff store exists.

## Follow-up export slice — 2026-08-12

The committed JSON ticket store can now project pending tickets onto this migration boundary without changing runtime persistence semantics:

- `internal/loginticket.ExportAuthLoginTicketHandoff(...)` and `FileStore.ExportAuthLoginTicketHandoff()` validate the same pending-ticket snapshots accepted by issue/load/consume,
- rows are deterministic by normalized login, original login, then login key,
- rows carry `migration_version = 7` / `migration_name = auth_login_ticket_handoff`, non-zero `login_key`, UTC `issued_at`, original and normalized login, empire, nil `consumed_at`, and `characters_snapshot_json`,
- `characters_snapshot_json` contains only the normalized select-screen character snapshot payload carried by the ticket, not the ticket envelope, not credentials, and not executable SQL,
- the export reads the committed pending ticket set and does not consume tickets, create SQL rows, apply migrations, or mutate the file store.

`gamed` exposes this read-only projection through loopback-only `GET /local/login-tickets/exports/auth-login-ticket-handoff` for operator/backfill preflight. The endpoint rejects non-loopback callers with `403`, non-`GET` methods with `405`, and invalid committed ticket state with `409`.

## TDD and validation

This slice is proven by:

- `go test ./db/migrations -count=1` for catalog/manifest/checksum validation and dry-run plan coverage,
- `go test ./internal/loginticket -count=1` for the migration-shaped login-ticket handoff export contract,
- `go test ./internal/minimal ./internal/ops -count=1` for runtime migration-plan/export wiring and ops response shape coverage,
- the full repository test/vet/format checks before commit.

## Next likely slices

1. Extract a narrow login-ticket repository seam only when tests prove it reduces file-store coupling.
2. Add a driver-backed migration preflight harness before introducing apply/rollback tooling.
3. Add import/quarantine tooling for the existing migration-shaped exports only after the repository seam is explicit.
