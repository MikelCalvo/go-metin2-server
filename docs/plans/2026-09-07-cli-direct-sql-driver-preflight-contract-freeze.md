# CLI direct SQL-driver preflight contract freeze — 2026-09-07

## Objective

Freeze a command-level **linked `database/sql` driver gate** for every
`metin2-migrate` command that directly calls `sql.Open` with an
operator-provided DSN. The gate makes an unlinked or malformed driver fail
closed with the existing explicit config error **before** `sql.Open` and,
for `apply`, before lock or audit reservation.

This follows the landed linked-driver inspector
([CLI sql-drivers contract freeze](2026-09-07-cli-sql-drivers-contract-freeze.md)).
It does not select, bundle, or make a production database engine the default.

## Why this follow-up is needed

The completed inspector answers whether a binary links a driver:

```bash
metin2-migrate drivers --require-driver "$DRIVER"
```

It also makes the generated `migration-run-retention` script stop before its
first DSN-touching command. Direct operator use of the individual commands
still reaches `sql.Open` after only checking that `--driver` and `--dsn` are
non-empty:

- `ledger-snapshot`
- `status`
- `apply`
- `import-export`

That behavior is safe in the narrow sense that an unregistered name makes
`database/sql` fail before a connection is established, but it is inconsistent
with daemon startup and the new inspector. It also leaves manual recovery or
migration use with a less deliberate error path.

This slice closes that consistency gap without widening the migration surface:
all four commands reuse `config.RequireRegisteredDatabaseDriver`, the same
primitive used by `ValidateDatabaseDriverAvailability` and `drivers
--require-driver`.

## Contract to freeze before GREEN

### A. Commands covered

Only these direct `sql.Open` call sites are in scope:

| Command | Existing database action after a successful gate |
| --- | --- |
| `ledger-snapshot` | reads metadata-only `schema_migrations` rows |
| `status` | reads the ledger and produces a metadata-only plan |
| `apply` | executes the explicitly confirmed migration plan |
| `import-export` | invokes a separately confirmed SQL import/backfill seam |

Read-only retained-artifact commands, `drivers`, `migration-run-retention`,
`backup-restore-drill`, `import-export-drill`, daemon HTTP handlers, and
runtime database configuration are not modified in this slice.

### B. Gate semantics

After each command has passed its existing required-flag validation, it must
call:

```go
config.RequireRegisteredDatabaseDriver(driverName)
```

with the raw `--driver` value before invoking `sql.Open`.

- A linked valid name proceeds to the command's existing `sql.Open` behavior.
- An unlinked name exits `1`, writes no JSON to stdout, and emits a short
  command-prefixed stderr error containing `database driver is unavailable`
  and the requested name.
- A supplied driver containing a NUL or embedded whitespace exits `1`, writes
  no JSON to stdout, and reuses the existing
  `ErrDatabaseConfigInvalid`-backed error. This is not a new CLI usage error:
  the required `--driver` flag was supplied but its database/sql name is
  invalid.
- The existing missing/blank `--driver` or `--dsn` behavior remains a usage
  error (`2`) with the command's existing usage text.
- The gate never opens, pings, queries, or otherwise contacts the DSN target.
- Stderr continues through `writeMigrationCommandError` wherever the command
  already carries a DSN, so a supplied DSN is redacted rather than echoed.

The gate reports binary linkage only. Passing it does **not** prove that the
DSN is reachable, that credentials are valid, that `schema_migrations` exists,
that a file-backed runtime store is SQL-backed, or that a migration/import is
safe to run.

### C. Required order

Preserve current validation and mutation safety ordering.

1. `ledger-snapshot`: validate flags and required driver/DSN first; gate
   immediately before the existing `sql.Open`.
2. `status`: validate flags, required driver/DSN, and `--target-version` first;
   gate immediately before the existing `sql.Open`.
3. `import-export`: preserve kind, confirmation, retained-export decoding, and
   quarantine validation before the gate; gate immediately before the existing
   `sql.Open`. An invalid retained export must still fail without consulting or
   opening the database target.
4. `apply`: preserve flags, retained ledger decoding, plan/rollback
   confirmation, plan-artifact/preflight validation, and no-pending audit
   eligibility checks. Gate after those non-DB checks but **before** reserving
   a local apply lock, creating an audit artifact, or calling `sql.Open`.

For `apply`, driver linkage failure must leave a requested `--lock-file` and
`--audit-file` absent. This makes a binary-capability mistake distinct from an
interrupted migration window and avoids stale-lock recovery work for a command
that never reached the target.

No command is required to render a list of available names in its stderr. An
operator who needs the complete sorted list uses `metin2-migrate drivers`.

### D. Security and compatibility constraints

- Keep stock release binaries free of an automatically registered production
  driver.
- Do not add aliases such as `sqlite` ↔ `sqlite3`.
- Do not add `--require-driver` flags to the four existing commands; their
  supplied `--driver` is itself the requirement.
- Do not change normal linked-driver success output, SQL plans, migration
  direction rules, lock/audit JSON schemas, DSN redaction, or exit codes
  unrelated to a driver gate.
- Do not add daemon migration apply/import endpoints, remote admin transport,
  token auth, forwarded-header trust, or proxy awareness.
- Do not make a DB connection while parsing flags, printing usage, validating a
  retained artifact, or inspecting linked drivers.

## TDD plan for GREEN

Focused tests should use the existing migration CLI test driver to prove that
an unlinked name causes **no `Open` events** and no JSON stdout:

1. `ledger-snapshot` with an unlinked driver and a secret-shaped DSN exits `1`,
   identifies unavailable linkage, and redacts the DSN.
2. `status` has the same failure behavior after a valid target-version parse.
3. `import-export` accepts and quarantines a structurally valid retained export,
   then rejects an unlinked driver before `sql.Open`; existing invalid-export
   ordering remains unchanged.
4. `apply` with a valid offline ledger/preflight path and an unlinked driver
   rejects before `sql.Open`, leaves requested lock/audit paths absent, and
   redacts the DSN.
5. Existing linked test-driver coverage for `ledger-snapshot`, `status`,
   `apply`, and `import-export` remains green, proving that a valid linkage gate
   does not alter the established success path.
6. Existing missing-driver/DSN usage tests remain green, proving that the
   gate does not change exit `2` contracts.
7. Build-tagged SQLite harness coverage remains green; the harness-only
   `sqlite` registration must satisfy the same gate without becoming a stock
   dependency.

Expected validation after GREEN:

```bash
go test ./internal/migratecli -run 'LedgerSnapshot|Status|Apply|ImportExport|Drivers' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Likely files for GREEN

- `internal/migratecli/migratecli.go`
- `internal/migratecli/import_export.go`
- `internal/migratecli/migratecli_test.go`
- `internal/migratecli/import_export_test.go`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- this plan (flip freeze → Done)

## Status

Contract frozen on `lane/persistence`; GREEN is intentionally deferred to the
next cohesive migration-CLI slice. The current linked-driver inspector remains
fully usable for manual/runbook preflight.

## Exit criteria for this freeze

- the four in-scope direct `sql.Open` commands are named;
- linkage failure, malformed driver, stdout, stderr, DSN-redaction, and
  missing-flag behavior are explicit;
- `apply` ordering explicitly protects lock/audit paths;
- test boundaries distinguish no-open failures from linked-driver success;
- no production Go code, production driver registration, or DB target is
  introduced by this freeze.

## Anti-goals / ordering constraints

- Do not open a RED test or implement GREEN before this contract is committed.
- Do not change the working `migration-run-retention` printer gate in this
  slice.
- Do not select or install a production SQL driver.
- Do not imply that driver linkage proves target health or durable runtime-store
  migration.
- Do not push `origin/main`; push only `origin/lane/persistence`.
