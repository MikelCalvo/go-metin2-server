# CLI Synthesize Wipe-Export Status — 2026-09-03

## Objective

Add a read-only CLI helper for validating and inspecting a retained
`synthesize-wipe-export` artifact (`wipe-quarantine.json`) without opening a
database target, re-running SQL import, or exposing DSNs / executable SQL.

The two-phase wipe→roster→reimport drill already synthesizes bare wipe-scope
exports into `$EXPORT_TREE/<kind>/wipe-quarantine.json` before scoped-replace
wipe imports. Operators can retain those files during a lab cutover, but there
is no small status command to re-check a retained wipe export during incident
review or release evidence collection. This closes the remaining status-helper
gap beside `import-export-status`, `plan-artifact-status`,
`ledger-snapshot-status`, `apply-preflight-status`, `apply-lock-status`, and
`apply-audit-status`.

## Why now

- Track E / migration-contract tips still name operator runbook hardening beyond
  the SQLite harness (and still defer upsert / stock production driver /
  cascade-delete / auto-run).
- Two-phase wipe→roster→reimport GREEN just landed
  ([import-export-drill two-phase wipe→roster→reimport](2026-09-03-import-export-drill-two-phase-wipe-roster-reimport.md)).
- Wipe-scope exports are the newest retained mutation-prep artifacts without a
  matching `*-status` inspector; wipe-import results already reuse
  `import-export-status`.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work.

## Contract frozen by this slice

```bash
metin2-migrate synthesize-wipe-export-status --kind <kind> --wipe-export <path>
```

Rules:

1. Requires `--kind` and `--wipe-export`; extra positional arguments are usage
   errors.
2. `--kind` must be one of the six character-FK wipe kinds already owned by
   `synthesize-wipe-export` / two-phase drill phase 1:
   - `character-item-state`
   - `character-point-state`
   - `character-myshop-unit-prices`
   - `character-quest-state`
   - `character-safebox-state`
   - `bootstrap-ground-item-state`
3. Performs no database open, SQL execution, import mutation, lock reservation,
   artifact deletion, or daemon mutation.
4. Returns success with `present: false` when the wipe-export path is absent.
5. Returns success with `present: true` plus checksum and decoded wipe export
   when the file is a valid bare wipe-scope export for the requested kind:
   - regular non-symlink file,
   - UTF-8,
   - size capped at 1 MiB (same bound as quarantine/synthesize input),
   - strict JSON decode with unknown fields / trailing JSON rejected,
   - bare migration-shaped export only (wrapped `{"summary":...,"export":...}`
     rejected — retained `wipe-quarantine.json` is always bare),
   - tip `migration_version` / `migration_name` match the kind,
   - quarantine contract succeeds,
   - derived wipe scope is non-empty (`character_ids` or `vids`),
   - all tip row arrays are empty (inventory/equipment/quickslots/points/unit
     prices/flags/safebox cells/money/ground items as applicable).
6. Never emits DSNs, executable SQL, runtime store rows, or import mutation
   output beyond the retained metadata-only wipe export.
7. On contract failure, exit `1` with a short stderr reason and **no** stdout
   status JSON (except the intentional `present: false` success path).
8. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists
   `synthesize-wipe-export-status` and the supported wipe kinds.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-synthesize-wipe-export-status-v1",
  "present": true,
  "kind": "character-item-state",
  "wipe_export_sha256": "...",
  "scope_key": "character_ids",
  "scope_count": 1,
  "scope_ids": [11],
  "export": {
    "migration_version": 3,
    "migration_name": "character_item_state",
    "character_ids": [11],
    "inventory_items": [],
    "equipment_items": [],
    "quickslots": []
  }
}
```

For `bootstrap-ground-item-state`, `scope_key` is `vids`.
`wipe_export_sha256` is computed over the exact retained file bytes so operators
can correlate the inspected file with lab notes / drill trees.

### Drill wiring

`metin2-migrate import-export-drill --i-confirm-print-two-phase-wipe-roster-reimport`
prints, after each phase-1 synthesize redirect, a matching status command:

```sh
metin2-migrate synthesize-wipe-export-status --kind <kind> --wipe-export "$EXPORT_TREE/<kind>/wipe-quarantine.json" > "$EXPORT_TREE/<kind>/wipe-quarantine-status.json"
```

The printer remains confirmation-gated and still does not execute synthesize,
import, or status itself.

## What this is not yet

- upsert / merge / cascade-delete inside tip-`0002` roster replace
- production DB engine selection as a stock default
- automatic / scheduled execution of printed synthesize / import / status
  commands
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming a present valid wipe-export proves live DB row absence beyond the
  retained wipe-scope artifact (operators still compare wipe-import-result
  status, quarantine exports, ledger evidence, and DB backups)

## Likely files to change

- `internal/migratecli/synthesize_wipe_export_status.go` (new)
- `internal/migratecli/synthesize_wipe_export_status_test.go` (new)
- `internal/migratecli/import_export_drill.go` (status lines after synthesize)
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `internal/migratecli/migratecli.go` (command switch + usage)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-03-import-export-drill-two-phase-wipe-roster-reimport.md`
  (pointer)
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing wipe-export → `present: false`, no DB open
- valid bare character-item wipe → `present: true` + checksum + scope_ids
- valid bare ground VID wipe → `scope_key=vids`
- empty scope / non-empty row arrays / wrapped quarantine / wrong kind /
  wrong migration identity / unknown fields / symlink / oversized → exit `1`,
  no stdout
- usage / unknown-command mention `synthesize-wipe-export-status`
- two-phase `import-export-drill` stdout includes the status redirects after
  each synthesize
- hermetic two-phase SQLite proof retains valid
  `wipe-quarantine-status.json` beside each wipe artifact

Validation for this slice:

```bash
go test ./internal/migratecli -run 'SynthesizeWipeExportStatus|ImportExportDrillPrintsTwoPhase|RejectsUnknownCommandMentionsSynthesizeWipeExportStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Exit criteria

- `metin2-migrate synthesize-wipe-export-status` is documented beside
  `synthesize-wipe-export` / other `*-status` helpers
- two-phase drill printer emits wipe-export status redirects for every wipe kind
- hermetic two-phase SQLite proof asserts retained wipe-quarantine-status
  artifacts
- Track E / migration-contract mark the wipe-export status helper done
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
