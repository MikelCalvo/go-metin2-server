# CLI export-tree-status import-outcome require-gate contract freeze — 2026-09-04

## Objective

Freeze opt-in fail-closed **require** flags on the already-landed
`metin2-migrate export-tree-status` import-outcome aggregates so operators can
gate cutover review on retained replace-mode / primary row-count evidence
without hand-parsing JSON.

This freeze does **not** invent wipe-import outcome projection, upsert / merge /
tip-`0002` cascade-delete, a stock production driver, automatic / scheduled
script execution, or any claim that outcome bits prove live DB row state beyond
the existing import-result contracts.

## Why docs-first

Track E tip chain through import-outcome aggregates is Done
([CLI export-tree-status import-outcome aggregates contract freeze](2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md)).

`export-tree-status` already reports:

- per-kind `import_result_outcome.{replaced,row_count}`
- `import_result_replaced_count`
- `import_result_row_count_total`
- `import_result_outcomes_complete`
- `import_result_all_replaced`

and already owns opt-in artifact-presence require-gates
([CLI export-tree-status cutover-artifact gate contract freeze](2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md)).

Missing children remain exit `0` by design for ungated inspection. That is
correct for browsing a retained tree, but operators still reassemble stop/go by
hand (or via `jq`) when they need fail-closed exit semantics on outcome
projection after insert-only, scoped-replace, or two-phase wipe→roster→reimport
drills.

Opening RED without freezing:

- exact flag names,
- which aggregate each flag gates,
- absent-tree (`present: false`) behavior under require flags,
- whether stdout JSON is suppressed on require failure,
- which printed `import-export-drill` after-status redirects adopt which flags,

would invent operator-facing exit semantics mid-implementation. Freeze first;
GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default inspector behavior stays ungated

1. With **no** require flags, `export-tree-status` keeps today's semantics:
   - absent tree → exit `0` with `present: false`,
   - present tree with missing children → exit `0` and report `present: false`
     child entries / false aggregates,
   - present but invalid child artifact → exit `1`, short stderr, **no** stdout
     status JSON.
2. Existing artifact-presence require flags stay unchanged.
3. No silent upgrade of inspection into a cutover gate.

### B. Opt-in outcome require flags (exact names frozen)

```bash
metin2-migrate export-tree-status --export-tree <absolute-path> \
  [--require-quarantine-complete] \
  [--require-two-phase-wipe-artifacts-complete] \
  [--require-import-result-artifacts-complete] \
  [--require-wipe-import-artifacts-complete] \
  [--require-import-result-outcomes-complete] \
  [--require-import-result-all-replaced]
```

Rules:

1. Each new flag is independently opt-in (boolean; default false).
2. New flags may be combined freely with the four existing artifact require flags.
3. Usage text lists every require flag beside `--export-tree`.
4. Unknown flags / unexpected args still exit `2`.
5. Still absolute `--export-tree` only; still opens no database; still emits no
   DSNs / executable SQL / live row payloads.
6. No new status JSON fields are required by this freeze.

### C. Require failure semantics

When any selected new outcome require flag is set:

1. If the export-tree path is absent (`present: false`), exit `1` with a short
   stderr reason that names the absent tree / failed gate, and **no** stdout
   status JSON.
2. If the tree is present but the matching aggregate is false, exit `1` with a
   short stderr reason that names the failed aggregate flag, and **no** stdout
   status JSON.
3. Mapping:
   - `--require-import-result-outcomes-complete` → `import_result_outcomes_complete`
   - `--require-import-result-all-replaced` → `import_result_all_replaced`
4. When every selected require flag's aggregate is true on a present tree, exit
   `0` and emit the same `go-metin2-export-tree-status-v1` JSON as today
   (aggregates already true; no new status fields required by this freeze).
5. Invalid present child artifacts still fail closed **before** aggregate
   gating (unchanged exit `1` / no stdout JSON path).

### D. Drill printer wiring (same GREEN as status flags)

`metin2-migrate import-export-drill` already retains ungated before-status and
artifact-gated after-status `export-tree-status` snapshots. GREEN must upgrade
**only the after-status** redirects:

1. **Before-status** stays ungated:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-before.json"
   ```
2. **After-status for insert-only printers** adds outcomes-complete only:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-import-result-artifacts-complete \
     --require-import-result-outcomes-complete \
     > "$EXPORT_TREE/export-tree-status-after.json"
   ```
   Do **not** add `--require-import-result-all-replaced` here: insert-only
   imports keep `replaced=false`, so `import_result_all_replaced` stays false by
   contract.
3. **After-status for scoped-replace printers** also adds outcomes-complete only:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-import-result-artifacts-complete \
     --require-import-result-outcomes-complete \
     > "$EXPORT_TREE/export-tree-status-after.json"
   ```
   Do **not** add `--require-import-result-all-replaced` here either: the owned
   omit-roster / partial re-backfill path can leave one or more tip kinds
   non-replaced while still producing complete outcome projection.
4. **After-status for two-phase wipe→roster→reimport** adds both outcome
   requires beside the four existing artifact requires:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-two-phase-wipe-artifacts-complete \
     --require-import-result-artifacts-complete \
     --require-wipe-import-artifacts-complete \
     --require-import-result-outcomes-complete \
     --require-import-result-all-replaced \
     > "$EXPORT_TREE/export-tree-status-after.json"
   ```
5. Printer remains confirmation-gated and still does not execute status /
   import itself. Contrib `lab-retention-gc` forwarding of the printer stays
   unchanged in this freeze (no new env vars required here).

### E. Explicit non-goals

- wipe-import outcome projection (`wipe_import_result_outcome` / wipe-side
  replaced/row-count aggregates)
- wipe-import outcome require flags
- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed synthesize / import / status scripts
- opening a database from `export-tree-status`
- claiming outcome / gate bits prove live DB row presence beyond retained
  import-result contracts
- changing default (ungated) missing-artifact exit `0` behavior
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/export_tree_status.go`
- `internal/migratecli/export_tree_status_test.go`
- `internal/migratecli/import_export_drill.go`
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-04-cli-export-tree-status.md` (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-result-completeness.md`
  (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md`
  (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md`
  (pointer)
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- incomplete present tree + each new require flag alone → exit `1`, empty
  stdout, stderr names the failed aggregate
- complete insert-only tree + `--require-import-result-outcomes-complete` →
  exit `0`; same tree + `--require-import-result-all-replaced` → exit `1`
- complete all-replaced tree + both new require flags → exit `0`, status JSON
  unchanged aside from still-true aggregates
- require flags with absent tree → exit `1`, empty stdout
- no require flags → existing missing-artifact exit `0` preserved
- invalid child artifact still fail-closed before aggregate gating
- usage lists new flags; unknown flag → exit `2`
- insert-only after-status prints quarantine + import-result artifact requires +
  outcomes-complete (not all-replaced)
- scoped-replace after-status also prints outcomes-complete only (not
  all-replaced), so omit-roster / partial re-backfill remains valid
- two-phase after-status prints all four artifact requires + both outcome
  requires; before-status stays ungated
- hermetic scoped-replace / two-phase after paths stay green under the gated
  after-status lines

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'ExportTreeStatus|ImportExportDrillPrints' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScript' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- Opt-in `--require-import-result-outcomes-complete` /
  `--require-import-result-all-replaced` fail closed with exit `1` and no stdout
  JSON when the matching aggregate is false or the tree is absent.
- Insert-only and scoped-replace after-status add outcomes-complete only
  (scoped-replace still supports omit-roster / partial re-backfill); two-phase
  after-status adds both outcome requires; before-status stays ungated.
- Wipe-import outcome projection and wipe outcome require flags remain deferred.
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.

## Exit criteria for this freeze

- this plan exists and names exact flags + failure semantics + drill after-status
  wiring
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not change default ungated exit `0` missing-child behavior in GREEN.
- Do not project wipe-import outcomes in this GREEN.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
