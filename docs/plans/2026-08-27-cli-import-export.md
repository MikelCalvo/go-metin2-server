# CLI import-export for quarantined migration-shaped exports — 2026-08-27

## Objective

Wire the first `metin2-migrate import-export` command so operators can apply a
retained, already-quarantinable migration-shaped export through the landed
programmatic SQL import seams (`ImportAccountCharacterRoster`,
`ImportCharacterItemState`, `ImportCharacterPointState`,
`ImportCharacterQuestState`, `ImportCharacterSafeboxState`,
`ImportAuthLoginTicketHandoff`, `ImportItemTemplateState`,
`ImportStaticActorContentState`, `ImportBootstrapGroundItemState`) without
inventing upsert policy, registering a stock production driver, or exposing a
daemon mutation route.

## Why now

- All tip-catalog SQL import primitives are green under the build-tagged SQLite
  harness (`0002`/`0003`/`0004`/`0007`/`0009`/`0010`/`0011`/`0013`/`0014`/`0015`).
- Offline `quarantine-export` already owns kind decoding / fail-closed shape
  checks; the missing cohesive operator step is CLI mutation against an
  operator-supplied `database/sql` driver+DSN.
- Track E / migration-contract follow-ups still name "CLI wiring" as deferred
  after the ninth import seam. Owning it next closes the offline backfill
  runbook gap for the playable PvE durable-state surface.

## Contract frozen by this slice

1. New CLI command: `metin2-migrate import-export`
2. Required flags:
   - `--kind <kind>` — same kind vocabulary as `quarantine-export`
   - `--export <path|->` — retained export JSON (regular non-symlink file or stdin)
   - `--driver <database/sql-driver>`
   - `--dsn <dsn>`
   - `--i-confirm-sql-import` — explicit mutation acknowledgement
3. Before opening the database the command:
   - rejects missing/unknown flags and unsupported kinds as usage (`exit 2`);
   - reads a bounded UTF-8 export (1 MiB cap, same as `quarantine-export`);
   - strict-decodes unknown-fields-disallowed JSON into the matching export type;
   - requires `--i-confirm-sql-import` before `sql.Open`.
4. After `sql.Open` the command dispatches to the matching `Import*` primitive.
   That primitive still re-runs quarantine, opens one transaction, gates on the
   applied ledger tip, inserts with parameterized `INSERT` (no upsert), and
   rolls back on conflict / FK / row-count drift.
5. Success stdout is metadata-only JSON from the `Import*Result` (counts + ids /
   names already owned by each seam). Stderr stays empty on success.
6. Failures write stderr only, redact the supplied DSN via the existing
   `writeMigrationCommandError` helper, and exit `1`.
7. The command does **not**:
   - register or select a production driver in stock binaries;
   - invent upsert / merge / truncate-and-reload policy;
   - mutate bootstrap FileStores / MemoryStores;
   - rewrite `schema_migrations`;
   - expose a `gamed` / `authd` ops mutation route;
   - embed executable SQL or DSN text in stdout.
8. Untagged tests cover usage errors, confirmation gate, invalid export before
   mutation, empty-export happy path against the existing migrate-CLI fake SQL
   driver (ledger preloaded with the required tip), DSN redaction, and usage
   listing. Build-tagged SQLite harness coverage remains owned by the per-package
   `Import*` tests.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- ~~scheduled / automatic import drill printer~~ Done for the confirmation-gated print-only `import-export-drill` surface — see [CLI import-export drill printer](2026-08-27-cli-import-export-drill.md). Automatic / scheduled execution of the printed import script remains deferred.
- DB-backed live runtime repositories
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/migratecli/import_export.go` (new)
- `internal/migratecli/import_export_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage + Run docstring)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md` (short pointer)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- this plan

## TDD and validation

Focused coverage:

- missing flags / unsupported kind / missing confirmation → usage or error, no DB open
- invalid export JSON / shape → error, no mutating exec events
- empty valid export + ledger tip present → exit 0, metadata JSON, begin/query/commit
- DSN appears only as `<redacted-dsn>` on runtime errors
- `help` / unknown-command usage lists `import-export`

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ImportExport|RejectsUnknownCommand' -count=1
gofmt -l internal/migratecli/import_export.go internal/migratecli/import_export_test.go internal/migratecli/migratecli.go
git diff --check
```

## Exit criteria

- `metin2-migrate import-export` is documented beside `quarantine-export` / `apply`
- Track E / migration-contract mark CLI import wiring as owned for tip kinds
- untagged `go test ./internal/migratecli` stays green without SQLite
- upsert / production-engine selection remain explicitly deferred
