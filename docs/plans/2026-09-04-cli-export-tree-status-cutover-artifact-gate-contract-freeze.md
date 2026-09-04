# CLI export-tree-status cutover-artifact gate contract freeze — 2026-09-04

## Objective

Freeze opt-in fail-closed **require** flags on the already-landed read-only
`metin2-migrate export-tree-status` inspector so operators can gate cutover
review on retained completeness aggregates without hand-parsing JSON.

This freeze does **not** invent upsert / merge / tip-`0002` cascade-delete, a
stock production driver, automatic / scheduled script execution, or any claim
that retained completeness bits prove live DB row state beyond the existing
import-result contracts.

## Why docs-first

Track E tip chain through import-result completeness aggregates is Done
([CLI export-tree status import-result completeness](2026-09-04-cli-export-tree-status-import-result-completeness.md)).

`export-tree-status` already reports:

- `quarantine_complete`
- `two_phase_wipe_artifacts_complete`
- `import_result_artifacts_complete`
- `wipe_import_artifacts_complete`

Missing child artifacts remain exit `0` by design. That is correct for
inspection, but operators still reassemble stop/go by hand (or via `jq`) when
reviewing a retained export tree after insert-only, scoped-replace, or two-phase
wipe→roster→reimport drills.

Opening RED without freezing:

- exact flag names,
- which aggregate each flag gates,
- absent-tree (`present: false`) behavior under require flags,
- whether stdout JSON is suppressed on require failure,
- which printed `import-export-drill` after-status redirects adopt the flags,

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
2. No silent upgrade of inspection into a cutover gate.

### B. Opt-in require flags (exact names frozen)

```bash
metin2-migrate export-tree-status --export-tree <absolute-path> \
  [--require-quarantine-complete] \
  [--require-two-phase-wipe-artifacts-complete] \
  [--require-import-result-artifacts-complete] \
  [--require-wipe-import-artifacts-complete]
```

Rules:

1. Each flag is independently opt-in (boolean; default false).
2. Flags may be combined freely.
3. Usage text lists every require flag beside `--export-tree`.
4. Unknown flags / unexpected args still exit `2`.
5. Still absolute `--export-tree` only; still opens no database; still emits no
   DSNs / executable SQL / live row payloads.

### C. Require failure semantics

When any selected require flag is set:

1. If the export-tree path is absent (`present: false`), exit `1` with a short
   stderr reason that names the absent tree / failed gate, and **no** stdout
   status JSON.
2. If the tree is present but the matching aggregate is false, exit `1` with a
   short stderr reason that names the failed aggregate flag, and **no** stdout
   status JSON.
3. Mapping:
   - `--require-quarantine-complete` → `quarantine_complete`
   - `--require-two-phase-wipe-artifacts-complete` → `two_phase_wipe_artifacts_complete`
   - `--require-import-result-artifacts-complete` → `import_result_artifacts_complete`
   - `--require-wipe-import-artifacts-complete` → `wipe_import_artifacts_complete`
4. When every selected require flag's aggregate is true on a present tree, exit
   `0` and emit the same `go-metin2-export-tree-status-v1` JSON as today
   (aggregates already true; no new status fields required by this freeze).
5. Invalid present child artifacts still fail closed **before** aggregate
   gating (unchanged exit `1` / no stdout JSON path).

### D. Drill printer wiring (same GREEN as status flags)

`metin2-migrate import-export-drill` already retains ungated before/after
`export-tree-status` snapshots. GREEN must upgrade **only the after-status**
redirect:

1. **Before-status** stays ungated:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-before.json"
   ```
2. **After-status for insert-only and scoped-replace printers** adds:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-import-result-artifacts-complete \
     > "$EXPORT_TREE/export-tree-status-after.json"
   ```
3. **After-status for two-phase wipe→roster→reimport** adds all four:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-two-phase-wipe-artifacts-complete \
     --require-import-result-artifacts-complete \
     --require-wipe-import-artifacts-complete \
     > "$EXPORT_TREE/export-tree-status-after.json"
   ```
4. Printer remains confirmation-gated and still does not execute status /
   import itself. Contrib `lab-retention-gc` forwarding of the printer stays
   unchanged in this freeze (no new env vars required here).

### E. Explicit non-goals

- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed synthesize / import / status scripts
- opening a database from `export-tree-status`
- claiming completeness / gate bits prove live DB row presence beyond retained
  import-result contracts
- changing default (ungated) missing-artifact exit `0` behavior
- outcome/success aggregates beyond artifact presence (row-count / replace-mode
  proofs)
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
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- incomplete present tree + each require flag alone → exit `1`, empty stdout,
  stderr names the failed aggregate
- complete tree + matching require flags → exit `0`, status JSON unchanged aside
  from still-true aggregates
- require flags with absent tree → exit `1`, empty stdout
- no require flags → existing missing-artifact exit `0` preserved
- invalid child artifact still fail-closed before aggregate gating
- usage lists new flags; unknown flag → exit `2`
- insert-only / scoped-replace after-status prints quarantine + import-result
  requires; two-phase after-status prints all four; before-status stays ungated
- hermetic two-phase after path stays green under the gated after-status line

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'ExportTreeStatus|ImportExportDrillPrints' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```


Follow-up owned separately and now GREEN: additive import-outcome / replace-mode aggregates on `export-tree-status` — see [CLI export-tree-status import-outcome aggregates contract freeze](2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md).

## Status

GREEN on `lane/persistence`.

- Opt-in `export-tree-status` require flags fail closed on absent/incomplete
  trees and succeed on complete trees without changing default ungated
  inspection.
- `import-export-drill` after-status redirects gate quarantine+import-result
  (and all four bits for two-phase); before-status stays ungated.
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
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
