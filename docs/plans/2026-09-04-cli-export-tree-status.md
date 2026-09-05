# CLI Export-Tree Status — 2026-09-04

## Objective

Add a read-only CLI helper that inspects a retained
`export-quarantine-drill` / import-export tree as one metadata-only evidence
artifact, without opening a database target, re-running SQL import, or exposing
DSNs / executable SQL.

Per-kind helpers already exist (`quarantine-export`, `import-export-status`,
`synthesize-wipe-export-status`). Lab cutover review still requires operators to
walk every tip-kind directory by hand and mentally assemble readiness. This
closes the remaining tree-level status-helper gap beside those single-artifact
inspectors.

## Why now

- Track E / migration-contract tips still name operator runbook hardening beyond
  the SQLite harness (and still defer upsert / stock production driver /
  cascade-delete / auto-run).
- Two-phase wipe→roster→reimport GREEN and `synthesize-wipe-export-status` just
  landed
  ([two-phase wipe→roster→reimport](2026-09-03-import-export-drill-two-phase-wipe-roster-reimport.md),
  [CLI synthesize wipe-export status](2026-09-03-cli-synthesize-wipe-export-status.md)).
- Retained export trees are the newest multi-artifact cutover evidence without a
  matching tree-level `*-status` inspector.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work.

## Contract frozen by this slice

```bash
metin2-migrate export-tree-status --export-tree <absolute-path>
```

Rules:

1. Requires `--export-tree`; extra positional arguments are usage errors.
2. `--export-tree` must be an absolute cleaned path (same absolute-path policy as
   `import-export-drill`).
3. Performs no database open, SQL execution, import mutation, lock reservation,
   artifact deletion, synthesize, quarantine rewrite, or daemon mutation.
4. Returns success with `present: false` when the export-tree path is absent.
5. When present:
   - path must resolve to a non-symlink directory,
   - inspects every tip kind in `exportQuarantineKinds` in fixed order,
   - for each kind reports presence/validity/checksum metadata for:
     - `quarantine.json` (required shape for import readiness),
     - `import-result.json` / `import-result-status.json` when present,
   - for each wipe kind in `importExportDrillWipeKinds` also reports:
     - `wipe-quarantine.json`,
     - `wipe-quarantine-status.json`,
     - `wipe-import-result.json` / `wipe-import-result-status.json` when present.
6. Missing child artifacts are reported as `present: false` inside the kind
   entry and do **not** fail the command by themselves.
7. A present but invalid child artifact (symlink, oversized, non-UTF-8, unknown
   fields, quarantine/import/wipe-status contract failure) fails closed with exit
   `1`, short stderr reason, and **no** stdout status JSON.
8. Never emits DSNs, executable SQL, runtime store rows, or import mutation
   output beyond retained metadata checksums / readiness bits.
9. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists
   `export-tree-status`.

Successful present output uses this metadata-only envelope:

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
  "import_result_present_count": 0,
  "kinds": [
    {
      "kind": "character-item-state",
      "wipe_kind": true,
      "quarantine": {
        "present": true,
        "path": "character-item-state/quarantine.json",
        "sha256": "..."
      },
      "wipe_quarantine": {
        "present": true,
        "path": "character-item-state/wipe-quarantine.json",
        "sha256": "...",
        "scope_key": "character_ids",
        "scope_count": 1
      },
      "wipe_quarantine_status": {
        "present": true,
        "path": "character-item-state/wipe-quarantine-status.json",
        "sha256": "..."
      },
      "import_result": {"present": false},
      "import_result_status": {"present": false},
      "wipe_import_result": {"present": false},
      "wipe_import_result_status": {"present": false}
    }
  ]
}
```

Aggregate bits:

- `quarantine_complete` is true only when every tip kind has a present valid
  `quarantine.json`.
- `two_phase_wipe_artifacts_complete` is true only when every wipe kind has a
  present valid `wipe-quarantine.json` **and** a present valid
  `wipe-quarantine-status.json`.
- Counts are derived from the same kind walk and never invent missing rows.

### Drill wiring

`metin2-migrate import-export-drill` prints, after the DSN env binding and before
kind mutations (all printer modes), a tree-status capture:

```sh
metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-before.json"
```

and after the final kind import/status redirects:

```sh
metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-after.json"
```

The printer remains confirmation-gated and still does not execute import, status,
or tree inspection itself.

Follow-up owned separately: additive import-result / wipe-import completeness
aggregates on `export-tree-status` —
see [CLI export-tree status import-result completeness](2026-09-04-cli-export-tree-status-import-result-completeness.md).

Follow-up owned separately: opt-in cutover-artifact require-gate on retained completeness aggregates — see [CLI export-tree-status cutover-artifact gate contract freeze](2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md).

Follow-up owned separately: additive import-outcome / replace-mode aggregates on `export-tree-status` — see [CLI export-tree-status import-outcome aggregates contract freeze](2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md).

Follow-up owned separately: opt-in import-outcome require-gate on retained outcome aggregates — see [CLI export-tree-status import-outcome require-gate contract freeze](2026-09-04-cli-export-tree-status-import-outcome-require-gate-contract-freeze.md).

Follow-up owned separately and now GREEN: additive wipe-import outcome aggregates on `export-tree-status` — see [CLI export-tree-status wipe-import outcome aggregates contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-aggregates-contract-freeze.md).

Follow-up owned separately and now GREEN: opt-in wipe-import outcome require-gate on retained wipe-outcome aggregates — see [CLI export-tree-status wipe-import outcome require-gate contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-require-gate-contract-freeze.md).

## What this is not yet

- upsert / merge / cascade-delete inside tip-`0002` roster replace
- production DB engine selection as a stock default
- automatic / scheduled execution of printed synthesize / import / status
  commands
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming `quarantine_complete` / `two_phase_wipe_artifacts_complete` prove live
  DB row presence/absence beyond retained artifacts (operators still compare
  import-result status, wipe-import-result status, ledger evidence, and DB
  backups)

## Likely files to change

- `internal/migratecli/export_tree_status.go` (new)
- `internal/migratecli/export_tree_status_test.go` (new)
- `internal/migratecli/import_export_drill.go` (before/after status redirects)
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `internal/migratecli/migratecli.go` (command switch + usage)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-03-cli-synthesize-wipe-export-status.md` (pointer)
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing export-tree → `present: false`, no DB open
- absolute empty tree directory → present kinds all `present: false`,
  `quarantine_complete=false`
- tree with valid quarantine.json for every tip kind →
  `quarantine_complete=true`
- wipe kinds with valid wipe-quarantine + wipe-quarantine-status →
  `two_phase_wipe_artifacts_complete=true`
- present invalid quarantine / wipe / import-result → exit `1`, no stdout
- relative export-tree / symlink tree / file-as-tree → fail closed
- usage / unknown-command mention `export-tree-status`
- import-export-drill printers (insert-only, scoped-replace, two-phase) emit
  before/after `export-tree-status` redirects
- hermetic two-phase SQLite proof retains valid before/after tree-status
  artifacts when applicable

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ExportTreeStatus|ImportExportDrillPrints|RejectsUnknownCommandMentionsExportTreeStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- `metin2-migrate export-tree-status` inspects retained export/import trees without opening a DB.
- `import-export-drill` printers emit before/after tree-status redirects.
- Hermetic two-phase SQLite proof retains valid before/after tree-status artifacts.

## Exit criteria

- `metin2-migrate export-tree-status` is documented beside the other `*-status`
  helpers
- import-export-drill printers emit before/after tree-status redirects
- hermetic two-phase SQLite proof asserts retained tree-status artifacts when
  the printed script is executed
- Track E / migration-contract mark the export-tree status helper done
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
