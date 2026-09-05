# CLI export-tree-status wipe-import outcome aggregates contract freeze — 2026-09-05

## Objective

Freeze additive **wipe-import outcome** aggregates on the already-landed read-only
`metin2-migrate export-tree-status` inspector so operators can review retained
`wipe-import-result.json` replace-mode + primary row-count evidence for the
character-FK wipe kinds without hand-parsing every wipe-kind result file.

This freeze does **not** invent wipe-import outcome require-gate flags, upsert /
merge / tip-`0002` cascade-delete, a stock production driver, automatic /
scheduled script execution, live DB row proofs beyond retained wipe-import-result
contracts, or any change to tip-kind `import_result_outcome` semantics.

## Why docs-first

Track E tip chain through import-outcome require-gates is Done
([CLI export-tree-status import-outcome require-gate contract freeze](2026-09-04-cli-export-tree-status-import-outcome-require-gate-contract-freeze.md)).

`export-tree-status` already reports:

- tip-kind `import_result_outcome.{replaced,row_count}`
- tip-level `import_result_replaced_count` / `import_result_row_count_total` /
  `import_result_outcomes_complete` / `import_result_all_replaced`
- wipe-kind artifact presence via `wipe_import_result` /
  `wipe_import_result_status` plus `wipe_import_artifacts_complete`

and already validates each present `wipe-import-result.json` through the same
strict tip-kind decode path used by tip-kind import results
(`inspectExportTreeImportResultArtifact`). But after checksum capture the decoded
wipe outcome is discarded (`_ = ...`), so operators must reopen every wipe-kind
result to answer:

1. Did every wipe kind run scoped-replace (`replaced: true`)?
2. How many primary rows did each wipe kind report?

Opening RED without freezing:

- exact additive field names,
- which wipe-kind count becomes the primary `row_count`,
- how omitted / false `replaced` maps into wipe aggregates,
- whether tip-kind outcome fields stay untouched,
- whether new wipe-outcome require-gate flags are in or out of this GREEN,

would invent operator-facing wipe-outcome semantics mid-implementation. Freeze
first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default artifact / tip-outcome behavior stays unchanged

1. Existing tip-kind outcome aggregates and their opt-in require-gates stay as
   landed.
2. Existing wipe-import artifact presence / completeness aggregates and their
   opt-in require-gates stay as landed.
3. Absent tree still returns `present: false` with exit `0` when ungated.
4. Present but invalid child artifacts still fail closed with exit `1` and no
   stdout status JSON **before** any wipe-outcome aggregation.
5. Wipe-outcome aggregates are additive only; they do not change default ungated
   missing-artifact exit `0` behavior.

### B. Additive per-wipe-kind outcome fields (exact names frozen)

When a wipe kind's `wipe-import-result.json` is present and valid, that kind
entry gains:

```json
{
  "wipe_import_result_outcome": {
    "replaced": true,
    "row_count": 0
  }
}
```

Rules:

1. Field name is exactly `wipe_import_result_outcome`.
2. Shape reuses the already-frozen tip-kind outcome object:
   `{ "replaced": bool, "row_count": int }`.
3. `replaced` is the boolean already owned by each tip-kind `Import*Result`
   (`replaced,omitempty`). Missing / omitted `replaced` means `false`.
4. `row_count` uses the **same** primary inserted-row mapping already frozen for
   tip-kind outcomes, limited to the wipe-kind set:
   - `character-item-state` → `inventory_item_count + equipment_item_count + quickslot_count`
   - `character-point-state` → `point_row_count`
   - `character-myshop-unit-prices` → `price_row_count`
   - `character-quest-state` → `flag_count`
   - `character-safebox-state` → `item_count`
   - `bootstrap-ground-item-state` → `ground_item_count`
5. When `wipe-import-result.json` is absent, omit `wipe_import_result_outcome`
   entirely (do not invent zeros that look like a successful empty wipe-import).
6. Non-wipe tip kinds never expose `wipe_import_result_outcome`.
7. Existing per-kind wipe artifact fields (`wipe_import_result`,
   `wipe_import_result_status`, wipe-quarantine companions) stay unchanged.
8. Tip-kind `import_result_outcome` stays independent: a wipe kind may expose
   both tip-import and wipe-import outcomes when both result files are present.

### C. Additive tree-level wipe aggregates (exact names frozen)

Successful present `go-metin2-export-tree-status-v1` output gains:

```json
{
  "wipe_import_result_replaced_count": 0,
  "wipe_import_result_row_count_total": 0,
  "wipe_import_result_outcomes_complete": false,
  "wipe_import_result_all_replaced": false
}
```

Rules:

1. Coverage denominator is the wipe-kind set
   (`importExportDrillWipeKinds`, currently 6 kinds), **not** full tip
   `kind_count`.
2. `wipe_import_result_replaced_count` counts wipe kinds whose present valid
   `wipe_import_result_outcome.replaced` is `true`.
3. `wipe_import_result_row_count_total` sums `wipe_import_result_outcome.row_count`
   across wipe kinds that expose an outcome object.
4. `wipe_import_result_outcomes_complete` is true only when every wipe kind has
   a present valid `wipe_import_result_outcome`. In practice GREEN should make
   this equivalent to `wipe_import_artifacts_complete` because outcome extraction
   reuses the already-validated decode path; keep the dedicated bit so operators
   can gate on wipe-outcome projection explicitly later.
5. `wipe_import_result_all_replaced` is true only when
   `wipe_import_result_outcomes_complete` is true **and**
   `wipe_import_result_replaced_count == len(importExportDrillWipeKinds)`.
6. Empty/absent trees omit these aggregates or leave them at zero/false with
   `present: false` (match existing aggregate style for incomplete trees).
7. No new `--require-*` flags in this GREEN. Require-gate adoption for
   `wipe_import_result_outcomes_complete` / `wipe_import_result_all_replaced` is
   a later opt-in freeze if operators need fail-closed exit semantics on wipe
   outcomes.

### D. Drill / hermetic wiring

No printer flag changes in this GREEN. Hermetic two-phase SQLite proof may
assert that `export-tree-status-after.json` reports:

- `wipe_import_result_outcomes_complete=true`
- `wipe_import_result_all_replaced=true`

when the printed two-phase script retained valid wipe-import results for every
wipe kind (those wipe imports already run with `--i-confirm-scoped-replace`).

Do **not** add wipe-outcome require flags to after-status redirects in this GREEN.

### E. Explicit non-goals

- wipe-import outcome require flags
  (`--require-wipe-import-result-outcomes-complete` /
  `--require-wipe-import-result-all-replaced`)
- changing tip-kind `import_result_outcome` mapping or tip-outcome require gates
- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed synthesize / import / status scripts
- opening a database from `export-tree-status`
- claiming wipe-outcome aggregates prove live DB row presence beyond retained
  wipe-import-result contracts
- changing default ungated missing-artifact exit `0` behavior
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/export_tree_status.go`
- `internal/migratecli/export_tree_status_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go` (optional
  after-tree assertions once fixtures expose wipe outcomes)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-04-cli-export-tree-status.md` (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-result-completeness.md`
  (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-outcome-aggregates-contract-freeze.md`
  (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-outcome-require-gate-contract-freeze.md`
  (pointer)
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- present tree with valid empty wipe-import results for every wipe kind →
  `wipe_import_result_outcomes_complete=true`,
  `wipe_import_result_replaced_count=0`,
  `wipe_import_result_all_replaced=false`,
  `wipe_import_result_row_count_total=0`,
  every wipe kind exposes `wipe_import_result_outcome.replaced=false`
- one wipe kind with `replaced: true` and non-zero primary counts →
  replaced count increments and `row_count` uses the frozen mapping
- mixed replaced / insert-only wipe tree →
  `wipe_import_result_all_replaced=false`
- all wipe kinds replaced → `wipe_import_result_all_replaced=true`
- absent wipe-import-result for a wipe kind → that kind omits
  `wipe_import_result_outcome` and
  `wipe_import_result_outcomes_complete=false`
- non-wipe tip kinds never expose `wipe_import_result_outcome`
- tip-kind `import_result_outcome` aggregates remain unchanged beside the new
  wipe fields
- invalid wipe-import-result still fail-closed before wipe-outcome aggregation
- no new require flags in usage text

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'ExportTreeStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- Exact additive wipe-outcome field names + primary `row_count` mapping are frozen and implemented.
- `export-tree-status` now projects per-wipe-kind `wipe_import_result_outcome` plus tree-level wipe replaced/row-count aggregates from retained `wipe-import-result.json` evidence.
- Wipe-import outcome require flags are frozen next — see [CLI export-tree-status wipe-import outcome require-gate contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-require-gate-contract-freeze.md).
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.

## Exit criteria for this freeze

- this plan exists and names exact wipe-outcome fields + row-count mapping +
  tree-level aggregates
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not change default ungated exit `0` missing-child behavior in GREEN.
- Do not add wipe-outcome require flags in this GREEN.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
