# CLI Import-Export Status — 2026-08-28

## Objective

Add a read-only CLI helper for validating and inspecting a retained
`import-export` result artifact (`import-result.json`) without opening a
database target, re-running SQL import, or exposing DSNs / executable SQL.

`metin2-migrate import-export` and the confirmation-gated
`import-export-drill` already write metadata-only `Import*Result` JSON beside
each tip kind. Operators can retain those files during a lab cutover, but there
is no small status command to re-check a retained result during incident review
or release evidence collection. This closes the remaining status-helper gap
beside `plan-artifact-status`, `ledger-snapshot-status`,
`apply-preflight-status`, `apply-lock-status`, and `apply-audit-status`.

## Why now

- Track E / migration-contract tips still name operator runbook hardening beyond
  the SQLite harness (and still defer upsert / stock production driver).
- Every other mutating CLI artifact already has a matching `*-status` inspector;
  import results are the remaining retained mutation evidence without one.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (no auto-run, no stock driver, no upsert).

## Contract frozen by this slice

```bash
metin2-migrate import-export-status --kind <kind> --import-result <path>
```

Rules:

1. Requires `--kind` and `--import-result`; extra positional arguments are usage
   errors.
2. `--kind` must be one of the existing tip kinds already owned by
   `quarantine-export` / `import-export`.
3. Performs no database open, SQL execution, import mutation, lock reservation,
   artifact deletion, or daemon mutation.
4. Returns success with `present: false` when the import-result path is absent.
5. Returns success with `present: true` plus checksum and decoded result when the
   file is a valid metadata-only `Import*Result` for the requested kind:
   - regular non-symlink file,
   - UTF-8,
   - size capped at 128 KiB,
   - strict JSON decode with unknown fields / trailing JSON rejected,
   - `migration_version` / `migration_name` match the tip identity for `--kind`,
   - non-negative count fields,
   - identity-slice lengths match their corresponding count fields,
   - identity slices are present (empty arrays allowed; null rejected via decode
     into concrete slices after normalize).
6. Never emits DSNs, executable SQL, runtime store rows, or import mutation
   output beyond the retained metadata-only result.
7. On contract failure, exit `1` with a short stderr reason and **no** stdout
   status JSON (except the intentional `present: false` success path).
8. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists
   `import-export-status` and the supported kinds.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-import-export-status-v1",
  "present": true,
  "kind": "account-character-roster",
  "import_result_sha256": "...",
  "result": {
    "migration_version": 2,
    "migration_name": "account_character_roster",
    "account_count": 0,
    "character_count": 0,
    "account_ids": [],
    "character_ids": []
  }
}
```

`import_result_sha256` is computed over the exact retained result bytes so
operators can correlate the inspected file with lab notes / drill trees.

### Drill wiring

`metin2-migrate import-export-drill` prints, after each tip-kind
`import-export` redirect, a matching status command:

```sh
metin2-migrate import-export-status --kind <kind> --import-result "$EXPORT_TREE/<kind>/import-result.json" > "$EXPORT_TREE/<kind>/import-result-status.json"
```

The printer remains confirmation-gated and still does not execute import or
status itself.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- automatic / scheduled execution of printed import / status commands
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming a present valid import-result proves live DB row contents beyond the
  retained metadata (operators still compare quarantine exports, ledger
  evidence, and DB backups)

## Likely files to change

- `internal/migratecli/import_export_status.go` (new)
- `internal/migratecli/import_export_status_test.go` (new)
- `internal/migratecli/import_export_drill.go` (status lines after import)
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/migratecli.go` (command switch + usage)
- `docs/development.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/plans/2026-08-27-cli-import-export.md` / drill plans (pointer)
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing import-result → `present: false`, no DB open
- valid roster empty result → `present: true` + checksum + tip identity
- wrong kind / wrong migration identity / unknown fields / symlink / oversized →
  exit `1`, no stdout
- count/slice length mismatch → exit `1`
- usage / unknown-command mention `import-export-status`
- `import-export-drill` stdout includes the status redirects after each import

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ImportExportStatus|ImportExportDrill|RejectsUnknownCommandMentionsImportExport' -count=1
gofmt -l internal/migratecli/import_export_status.go internal/migratecli/import_export_status_test.go internal/migratecli/import_export_drill.go internal/migratecli/import_export_drill_test.go internal/migratecli/migratecli.go
git diff --check
```

## Exit criteria

- `metin2-migrate import-export-status` is documented beside `import-export` /
  other `*-status` helpers
- drill printer emits status redirects for every tip kind
- Track E / migration-contract mark the import-result status helper done
- untagged `go test ./internal/migratecli` stays green without SQLite
- hermetic `/bin/sh` status-redirect proof against SQLite is owned by [hermetic import-export status SQLite proof](2026-08-28-hermetic-import-export-status-sqlite.md)
- upsert / auto-run / stock production driver remain explicitly deferred

## Anti-goals / ordering constraints

- Do not open a database from the status command.
- Do not embed DSN values in status stdout.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
