# CLI import-export drill printer — 2026-08-27

## Objective

Add a confirmation-gated, print-only `metin2-migrate import-export-drill`
command so operators can turn a retained lab export/quarantine tree into the
concrete shell steps that invoke the already-landed confirmation-gated
`import-export` mutation across every tip kind — without the printer opening a
database, executing imports, embedding a DSN value, inventing upsert policy, or
registering a stock production driver.

## Why now

- [CLI import-export](2026-08-27-cli-import-export.md) owns the mutating CLI
  seam for tip kinds, but operators still invent host-local wrappers to walk a
  retained `/var/metin2/exports/.../<kind>/quarantine.json` tree into
  `import-export`.
- `export-quarantine-drill`, `backup-restore-drill`, and
  `migration-run-retention` already own path-aware printers for their runbooks;
  SQL import is the missing companion after the nine tip import seams + CLI
  wiring landed.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (auto-run, stock driver registration, upsert, remote admin).

## Contract frozen by this slice

```bash
metin2-migrate import-export-drill \
  --export-tree /var/metin2/exports/20260827T120000Z-abcdef012345 \
  --driver <database/sql-driver-name> \
  [--dsn-env METIN2_IMPORT_DSN] \
  --i-confirm-print-sql-import-drill
```

Behavior:

1. `--export-tree` is required and must be an absolute cleaned path (the retained
   `YYYYMMDDTHHMMSSZ-<commit12>` tree produced by `export-quarantine-drill`).
2. `--driver` is required. The printer treats it as an opaque
   `database/sql` driver name literal for the printed script; it does **not**
   `sql.Open`, register drivers, or validate that the driver is linked into the
   current binary.
3. `--dsn-env` defaults to `METIN2_IMPORT_DSN`. The printed script reads the DSN
   only from that environment variable (`DSN="${METIN2_IMPORT_DSN:?...}"`) so the
   printer never embeds a DSN value, password, or connection string in stdout.
4. `--i-confirm-print-sql-import-drill` is required before any script is emitted
   (same confirmation-gated print pattern as `artifact-gc-aside-purge`). Missing
   confirmation is an error exit (`1`) with no stdout script.
5. On success, stdout is a plain-text shell script that:
   - sets `EXPORT_TREE`, `DRIVER`, and `DSN_ENV`;
   - fails closed if `$DSN_ENV` is unset/empty when the operator later runs it;
   - for each tip kind in the fixed `exportQuarantineKinds` order, prints:
     - existence check for `$EXPORT_TREE/<kind>/quarantine.json`;
     - `metin2-migrate import-export --kind <kind> --export ... --driver ... --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/<kind>/import-result.json"`;
     - `metin2-migrate import-export-status --kind <kind> --import-result "$EXPORT_TREE/<kind>/import-result.json" > "$EXPORT_TREE/<kind>/import-result-status.json"`;
   - never executes HTTP/SQL, never writes files itself, never opens a database,
     never embeds a DSN value or executable SQL DDL/DML.
6. Covered kinds (order fixed, same vocabulary as `quarantine-export` /
   `import-export`):
   - `account-character-roster`
   - `character-item-state`
   - `character-point-state`
   - `character-quest-state`
   - `character-safebox-state`
   - `auth-login-ticket-handoff`
   - `item-template-state`
   - `static-actor-content-state`
   - `bootstrap-ground-item-state`
7. On contract failure, exit `1` with a short stderr reason and **no** stdout
   script. Missing/unknown flags / unexpected args → usage exit `2`. Usage text
   lists `import-export-drill`.
8. Help / unknown-command usage lists `import-export-drill`.

## What this is not yet

- automatic / scheduled execution of the printed import script
- ~~folding `import-export-drill` into `contrib/lab-retention-gc` print-only samples~~ Done — see [contrib import-export drill print helper](2026-08-27-contrib-import-export-drill-print-helper.md)
- ~~hermetic `/bin/sh` execution proof against a real driver-backed database~~ Done — see [hermetic import-export drill SQLite execution proof](2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md)
- ~~read-only status helper for retained `import-result.json` beside each drill import~~ Done — see [CLI import-export status](2026-08-28-cli-import-export-status.md)
- upsert / merge / truncate-and-reload policy
- ~~opt-in print of `--i-confirm-scoped-replace` from `import-export-drill`~~ Done — see [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md)
- production DB engine selection as a stock default
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/migratecli/import_export_drill.go` (new)
- `internal/migratecli/import_export_drill_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage + Run docstring)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md` (short pointer beside import-export)
- `docs/workflow/lab-deployment-topology.md` (short pointer beside export drill)
- `docs/plans/2026-08-27-cli-import-export.md` (mark drill follow-up)
- `docs/plans/2026-08-09-db-migration-contract.md` (Track E tip)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- this plan

## TDD and validation

Focused coverage:

- missing flags / missing confirmation / unexpected args → usage or error, no
  stdout script
- relative `--export-tree` / blank `--driver` / blank `--dsn-env` → error, no
  stdout
- happy path prints all nine kinds with quarantine.json checks + import-export
  lines, uses `$DSN` from env var, never embeds `postgres://` / `memory://` /
  `CREATE TABLE`
- help / unknown-command usage lists `import-export-drill`

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ImportExportDrill|RejectsUnknownCommandMentionsImportExportDrill' -count=1
gofmt -l internal/migratecli/import_export_drill.go internal/migratecli/import_export_drill_test.go internal/migratecli/migratecli.go
git diff --check
```

## Exit criteria

- `metin2-migrate import-export-drill` is documented beside `import-export` /
  `export-quarantine-drill`
- Track E / migration-contract mark the print-only import drill as owned
- untagged `go test ./internal/migratecli` stays green without SQLite
- auto-run / stock driver / upsert remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed import commands from the CLI.
- Do not embed DSN values in printer stdout.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
