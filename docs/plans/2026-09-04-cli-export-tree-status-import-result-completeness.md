# CLI Export-Tree Status Import-Result Completeness — 2026-09-04

## Objective

Extend the already-landed read-only `metin2-migrate export-tree-status`
inspector with aggregate completeness bits for retained post-import evidence:

- tip-kind `import-result.json` + `import-result-status.json`
- wipe-kind `wipe-import-result.json` + `wipe-import-result-status.json`

Operators already retain those artifacts during insert-only, scoped-replace, and
two-phase wipe→roster→reimport drills. Tree status currently counts
`import_result_present_count` and walks the child presence fields, but still
forces cutover review to reassemble “did every tip kind finish import + status?”
and “did every wipe kind finish wipe-import + status?” by hand.

## Why now

- Track E / migration-contract tips still name operator runbook hardening beyond
  the SQLite harness (and still defer upsert / stock production driver /
  cascade-delete / auto-run).
- `export-tree-status` just landed with quarantine / two-phase wipe aggregates
  ([CLI export-tree status](2026-09-04-cli-export-tree-status.md)).
- Hermetic two-phase proofs already retain before/after tree-status snapshots
  and assert quarantine / wipe-quarantine completeness; they still cannot assert
  a single tree-level import-evidence complete bit.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work.

## Contract frozen by this slice

Successful present `go-metin2-export-tree-status-v1` output gains these additive
aggregate fields (existing fields stay unchanged):

```json
{
  "format": "go-metin2-export-tree-status-v1",
  "present": true,
  "export_tree": "/var/metin2/exports/20260904T120000Z-abcdef012345",
  "kind_count": 10,
  "quarantine_present_count": 10,
  "quarantine_complete": true,
  "wipe_quarantine_present_count": 6,
  "two_phase_wipe_artifacts_complete": true,
  "import_result_present_count": 10,
  "import_result_status_present_count": 10,
  "import_result_artifacts_complete": true,
  "wipe_import_result_present_count": 6,
  "wipe_import_result_status_present_count": 6,
  "wipe_import_artifacts_complete": true,
  "kinds": []
}
```

Rules:

1. Still requires absolute `--export-tree`; still opens no database; still emits
   no DSNs / executable SQL / live row payloads.
2. Missing child artifacts remain `present: false` and do **not** fail the
   command by themselves.
3. Present but invalid child artifacts still fail closed with exit `1` and no
   stdout status JSON (unchanged).
4. Aggregate bits:
   - `import_result_status_present_count` counts tip kinds whose
     `import-result-status.json` is present and valid.
   - `import_result_artifacts_complete` is true only when every tip kind has a
     present valid `import-result.json` **and** a present valid
     `import-result-status.json`.
   - `wipe_import_result_present_count` counts wipe kinds whose
     `wipe-import-result.json` is present and valid.
   - `wipe_import_result_status_present_count` counts wipe kinds whose
     `wipe-import-result-status.json` is present and valid.
   - `wipe_import_artifacts_complete` is true only when every wipe kind has a
     present valid `wipe-import-result.json` **and** a present valid
     `wipe-import-result-status.json`.
5. Counts are derived from the same kind walk and never invent missing rows.
6. These bits remain retained-artifact evidence only: they do **not** prove live
   DB row presence/absence beyond the already-validated import-result contracts.

### Drill / hermetic wiring

No printer flag changes. Hermetic two-phase SQLite proof should assert that
`export-tree-status-after.json` reports:

- `import_result_artifacts_complete=true`
- `wipe_import_artifacts_complete=true`

when the printed two-phase script retained the matching import / wipe-import
status artifacts.

Follow-up owned separately: opt-in cutover-artifact require-gate on `export-tree-status` — see [CLI export-tree-status cutover-artifact gate contract freeze](2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md).

Follow-up owned separately: additive import-outcome / replace-mode aggregates on `export-tree-status` — see [CLI export-tree-status import-outcome aggregates contract freeze](2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md).

Follow-up owned separately: opt-in import-outcome require-gate on retained outcome aggregates — see [CLI export-tree-status import-outcome require-gate contract freeze](2026-09-04-cli-export-tree-status-import-outcome-require-gate-contract-freeze.md).

Follow-up owned separately: additive wipe-import outcome aggregates on `export-tree-status` — see [CLI export-tree-status wipe-import outcome aggregates contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-aggregates-contract-freeze.md).

## What this is not yet

- upsert / merge / cascade-delete inside tip-`0002` roster replace
- production DB engine selection as a stock default
- automatic / scheduled execution of printed synthesize / import / status
  commands
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming import/wipe completeness bits prove live DB state beyond retained
  import-result contracts

## Likely files to change

- `internal/migratecli/export_tree_status.go`
- `internal/migratecli/export_tree_status_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-04-cli-export-tree-status.md` (pointer)
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- empty present tree → all new complete bits false / counts zero
- quarantine+wipe complete tree without import artifacts →
  `import_result_artifacts_complete=false`,
  `wipe_import_artifacts_complete=false`
- tree with valid import-result + import-result-status for every tip kind →
  `import_result_artifacts_complete=true`
- wipe kinds with valid wipe-import-result + wipe-import-result-status →
  `wipe_import_artifacts_complete=true`
- hermetic two-phase after snapshot asserts the new complete bits

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ExportTreeStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- `export-tree-status` reports `import_result_artifacts_complete` and
  `wipe_import_artifacts_complete` from retained import / wipe-import evidence.
- Hermetic two-phase SQLite after-tree snapshot asserts both completeness bits.
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.

## Exit criteria

- `export-tree-status` documents the new aggregate bits beside quarantine /
  two-phase wipe completeness
- hermetic two-phase SQLite proof asserts after-tree import/wipe completeness
- Track E / migration-contract mark the import-result completeness follow-up done
- untagged `go test ./internal/migratecli` stays green without SQLite
- upsert / auto-run / stock production driver / cascade-delete remain explicitly
  deferred

## Anti-goals / ordering constraints

- Do not open a database from the status command.
- Do not embed DSN values in status stdout.
- Do not register a production driver in stock binaries.
- Do not invent cascade delete inside roster replace.
- Do not auto-run printed scripts from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/persistence`.
