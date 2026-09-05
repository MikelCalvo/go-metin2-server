# CLI export-tree-status import-outcome aggregates contract freeze — 2026-09-04

## Objective

Freeze additive **import-outcome** aggregates on the already-landed read-only
`metin2-migrate export-tree-status` inspector so operators can review retained
`import-result.json` replace-mode + primary row-count evidence without hand-parsing
every tip-kind result file.

This freeze does **not** invent upsert / merge / tip-`0002` cascade-delete, a
stock production driver, automatic / scheduled script execution, live DB row
proofs beyond retained import-result contracts, or new require-gate flags.

## Why docs-first

Track E tip chain through cutover-artifact require-gates is Done
([CLI export-tree-status cutover-artifact gate contract freeze](2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md)).

`export-tree-status` already reports artifact presence / completeness:

- `quarantine_complete`
- `two_phase_wipe_artifacts_complete`
- `import_result_artifacts_complete`
- `wipe_import_artifacts_complete`

and already validates each present `import-result.json` through the same
strict tip-kind decode path used by `import-export-status`. But the tree-level
JSON still drops the decoded outcome after checksum capture (`_ = result`), so
operators must reopen every tip-kind result (or its status wrapper) to answer:

1. Did every tip kind run scoped-replace (`replaced: true`)?
2. How many primary rows did each tip kind insert?

Opening RED without freezing:

- exact additive field names,
- which tip-kind count becomes the primary `row_count`,
- how omitted / false `replaced` maps into aggregates,
- whether wipe-import outcomes are included or deferred,
- whether new require-gate flags are in or out of this GREEN,

would invent operator-facing outcome semantics mid-implementation. Freeze first;
GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default artifact-presence behavior stays unchanged

1. Existing presence / completeness aggregates and opt-in require-gates stay as
   landed by the cutover-artifact gate GREEN.
2. Absent tree still returns `present: false` with exit `0` when ungated.
3. Present but invalid child artifacts still fail closed with exit `1` and no
   stdout status JSON **before** any outcome aggregation.
4. Outcome aggregates are additive only; they do not change default ungated
   missing-artifact exit `0` behavior.

### B. Additive per-kind outcome fields (exact names frozen)

When a tip kind's `import-result.json` is present and valid, that kind entry
gains:

```json
{
  "import_result_outcome": {
    "replaced": false,
    "row_count": 0
  }
}
```

Rules:

1. Field name is exactly `import_result_outcome`.
2. `replaced` is the boolean already owned by each tip-kind `Import*Result`
   (`replaced,omitempty`). Missing / omitted `replaced` means `false`.
3. `row_count` is the tip-kind primary inserted-row count, mapped exactly as:
   - `account-character-roster` → `character_count`
   - `character-item-state` → `inventory_item_count + equipment_item_count + quickslot_count`
   - `character-point-state` → `point_row_count`
   - `character-myshop-unit-prices` → `price_row_count`
   - `character-quest-state` → `flag_count`
   - `character-safebox-state` → `item_count`
   - `auth-login-ticket-handoff` → `ticket_count`
   - `item-template-state` → `template_count`
   - `static-actor-content-state` → `static_actor_count`
   - `bootstrap-ground-item-state` → `ground_item_count`
4. When `import-result.json` is absent, omit `import_result_outcome` entirely
   (do not invent zeros that look like a successful empty import).
5. Existing per-kind artifact fields (`import_result`, `import_result_status`,
   wipe companions) stay unchanged.

### C. Additive tree-level aggregates (exact names frozen)

Successful present `go-metin2-export-tree-status-v1` output gains:

```json
{
  "import_result_replaced_count": 0,
  "import_result_row_count_total": 0,
  "import_result_outcomes_complete": false,
  "import_result_all_replaced": false
}
```

Rules:

1. `import_result_replaced_count` counts tip kinds whose present valid
   `import_result_outcome.replaced` is `true`.
2. `import_result_row_count_total` sums `import_result_outcome.row_count` across
   tip kinds that expose an outcome object.
3. `import_result_outcomes_complete` is true only when every tip kind has a
   present valid `import_result_outcome` (same coverage bar as
   `import_result_artifacts_complete`, but derived from decoded outcomes rather
   than artifact presence alone). In practice GREEN should make this equivalent
   to `import_result_artifacts_complete` because outcome extraction reuses the
   already-validated decode path; keep the dedicated bit so operators can gate
   on outcome projection explicitly later.
4. `import_result_all_replaced` is true only when
   `import_result_outcomes_complete` is true **and**
   `import_result_replaced_count == kind_count`.
5. Empty/absent trees omit these aggregates or leave them at zero/false with
   `present: false` (match existing aggregate style for incomplete trees).
6. No new `--require-*` flags in this GREEN. Require-gate adoption for
   `import_result_all_replaced` / `import_result_outcomes_complete` is a later
   opt-in freeze if operators need fail-closed exit semantics.

### D. Wipe-import outcomes stay deferred

1. This freeze covers tip-kind `import-result.json` outcomes only.
2. `wipe-import-result.json` / wipe-import status companions stay presence-only
   under the already-landed wipe-import completeness bits.
3. Do not add `wipe_import_result_outcome` or wipe-side replaced/row-count
   aggregates in this GREEN.

### E. Explicit non-goals

- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed synthesize / import / status scripts
- opening a database from `export-tree-status`
- claiming outcome aggregates prove live DB row presence beyond retained
  import-result contracts
- new require-gate flags for outcome aggregates
- wipe-import outcome projection
- changing default ungated missing-artifact exit `0` behavior
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/export_tree_status.go`
- `internal/migratecli/export_tree_status_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go` (optional
  after-tree assertions once fixtures expose outcomes)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-04-cli-export-tree-status.md` (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-import-result-completeness.md`
  (pointer)
- `docs/plans/2026-09-04-cli-export-tree-status-cutover-artifact-gate-contract-freeze.md`
  (pointer)
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- present tree with valid empty insert-only import results →
  `import_result_outcomes_complete=true`,
  `import_result_replaced_count=0`,
  `import_result_all_replaced=false`,
  `import_result_row_count_total=0`,
  every kind exposes `import_result_outcome.replaced=false`
- one tip kind with `replaced: true` and non-zero primary counts →
  replaced count increments and `row_count` uses the frozen mapping
- mixed replaced / insert-only tree → `import_result_all_replaced=false`
- all tip kinds replaced → `import_result_all_replaced=true`
- absent import-result for a tip kind → that kind omits
  `import_result_outcome` and `import_result_outcomes_complete=false`
- invalid import-result still fail-closed before outcome aggregation
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

- Exact additive outcome field names + primary `row_count` mapping are frozen and implemented.
- `export-tree-status` now projects per-kind `import_result_outcome` plus tree-level replaced/row-count aggregates from retained tip-kind `import-result.json` evidence.
- Wipe-import outcomes were deferred here; follow-up is now GREEN — see [CLI export-tree-status wipe-import outcome aggregates contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-aggregates-contract-freeze.md).
- Follow-up owned separately and now GREEN: opt-in import-outcome require-gate + drill after-status wiring — see [CLI export-tree-status import-outcome require-gate contract freeze](2026-09-04-cli-export-tree-status-import-outcome-require-gate-contract-freeze.md).
- Follow-up owned separately and now GREEN: opt-in wipe-import outcome require-gate + two-phase after-status wiring — see [CLI export-tree-status wipe-import outcome require-gate contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-require-gate-contract-freeze.md).

## Exit criteria for this freeze

- this plan exists and names exact outcome fields + row-count mapping +
  tree-level aggregates
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not change default ungated exit `0` missing-child behavior in GREEN.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
