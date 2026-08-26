# Auth-login-ticket-handoff SQL import/backfill — 2026-08-26

## Objective

Add the eighth programmatic SQL import/backfill seam for quarantined
`0007_auth_login_ticket_handoff` exports: validate through the existing
quarantine contract, insert rows into `auth_login_tickets` inside one
transaction, and prove the round-trip on the build-tagged SQLite harness after
the catalog tip includes version `7`.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, `0004` quest-state, `0009` item-template-state, `0010`
ground-item-state, `0011` point-state, and `0014`/`0015` safebox seams — without
selecting a production engine, shipping a stock release driver, wiring a
daemon/ops mutation endpoint, inventing upsert/idempotent rewrite policy, or
claiming DB-backed live login-ticket issue/load/consume.

## Why now

- Export + quarantine for tip-`0007` auth-login-ticket handoff are green offline
  and on loopback; the missing cohesive step is INSERT execution against a real
  engine for the authd→gamed one-shot handoff surface the PvE boot path already
  uses (login → select → entergame).
- Track E and migration-contract follow-ups still name content-shaped SQL import
  (`0007`, then `0008`/`0012`/`0013`) as deferred after the character/ground/
  safebox/template imports. Owning tickets next closes the first non-character
  auth handoff import beside the landed JSON ticket FileStore path.

## Contract frozen by this slice

1. New primitive lives in `internal/loginticket`:
   - `ImportAuthLoginTicketHandoff(ctx, executor, export) (AuthLoginTicketHandoffImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineAuthLoginTicketHandoffExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `7` /
     `auth_login_ticket_handoff` (empty/missing ledger or tip `< 7` fail closed);
   - inserts ticket rows with parameterized `INSERT` statements (no
     `OR REPLACE` / upsert) using durable columns only:
     - omit `created_at` / `updated_at` (database defaults);
     - bind `issued_at` / optional `consumed_at` as UTC RFC3339Nano TEXT;
     - bind `characters_snapshot_json` as the quarantined transitional payload;
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`7` / `auth_login_ticket_handoff`);
   - `ticket_count` / `active_ticket_count`;
   - sorted `login_keys`.
   It never includes ticket payloads, SQL text, DSNs, or FileStore snapshot
   bytes.
4. Empty quarantined exports (present empty `tickets` slice) succeed as a no-op
   transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / active-login-key unique-index collisions fail closed
   and roll the transaction back (no partial import).
6. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores / live pending tickets;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - invent upsert / merge / truncate-and-reload policy;
   - claim DB-backed runtime ticket issue/load/consume (JSON FileStore remains
     the restart path).
7. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0007` on a temp SQLite DB, imports a quarantined sample export
   (active + optional consumed rows with characters snapshot JSON), and
   `SELECT`s the durable rows back. Default untagged `go test ./...` stays free
   of the SQLite dependency.
8. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0007` + `0009` + `0010` + `0011` + `0014`/`0015`;
   `0008`/`0012`/`0013` imports, CLI wiring, production-engine selection, upsert
   policy, and scheduled purge fold remain deferred.

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0008`+`0012`+`0013` static-actor content
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for login tickets
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/loginticket/auth_login_ticket_handoff_import.go` (new)
- `internal/loginticket/auth_login_ticket_handoff_import_test.go` (new; untagged fail-closed cases)
- `internal/loginticket/auth_login_ticket_handoff_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 7` → import sample ticket handoff → SELECT matches
  login_key / issued_at / login / login_normalized / empire / consumed_at /
  characters_snapshot_json
- sqlite harness: second import of the same primary keys fails closed (unique conflict)
- sqlite harness: import before migrations / without `0007` fails closed
- sqlite harness: empty export succeeds as no-op after ledger gate
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/loginticket -run 'AuthLoginTicketHandoffImport' -count=1
go test -tags=sqlite_harness ./internal/loginticket -run 'SQLiteHarness.*AuthLoginTicketHandoffImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0007` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0007`+`0009`+`0010`+`0011`+`0014`/`0015`
  SQL import as owned and keep `0008`/`0012`/`0013` / CLI / engine selection
  deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
