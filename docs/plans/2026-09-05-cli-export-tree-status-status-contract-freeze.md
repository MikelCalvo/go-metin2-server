# CLI export-tree-status-status contract freeze — 2026-09-05

## Objective

Freeze a read-only `metin2-migrate export-tree-status-status` inspector for
retained `export-tree-status-before.json` / `export-tree-status-after.json`
artifacts so operators can re-check tree-level cutover evidence during incident
review or release packaging **without** walking the original export-tree or
opening a database.

This freeze does **not** invent upsert / merge / tip-`0002` cascade-delete, a
stock production driver, automatic / scheduled script execution, new inner
`go-metin2-export-tree-status-v1` fields, loopback ops mutation, or any claim
that a present valid tree-status file proves live DB row state beyond the
existing retained import / wipe-import contracts.

## Why docs-first

Track E tip chain through wipe-import outcome require-gates is Done
([CLI export-tree-status wipe-import outcome require-gate contract freeze](2026-09-05-cli-export-tree-status-wipe-import-outcome-require-gate-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`plan-artifact-status`, `ledger-snapshot-status`,
`apply-preflight-status`, `apply-lock-status`, `apply-audit-status`,
`import-export-status`, `synthesize-wipe-export-status`).
`import-export-drill` already retains before/after `export-tree-status` JSON
beside the tree, but there is no small command to re-validate those files by
themselves after the original tree is archived, copied, or hand-edited.

Opening RED without freezing:

- exact command / flag names,
- outer status envelope + checksum field,
- size cap and fail-closed file policy,
- which inner aggregates must recompute from decoded kind entries,
- whether the original export-tree may be opened,
- which printed `import-export-drill` redirects adopt the new inspector,

would invent operator-facing incident-review semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate export-tree-status-status --export-tree-status <path> \
  [--require-quarantine-complete] \
  [--require-two-phase-wipe-artifacts-complete] \
  [--require-import-result-artifacts-complete] \
  [--require-wipe-import-artifacts-complete] \
  [--require-import-result-outcomes-complete] \
  [--require-import-result-all-replaced] \
  [--require-wipe-import-result-outcomes-complete] \
  [--require-wipe-import-result-all-replaced]
```

Rules:

1. Requires `--export-tree-status`; extra positional arguments are usage errors
   (exit `2`).
2. The path is a retained `export-tree-status` JSON file, **not** the
   export-tree directory. Relative paths are allowed (same file-inspector
   policy as `import-export-status` / `plan-artifact-status`). stdin (`-`) is
   **not** accepted.
3. Each require flag is independently opt-in (boolean; default false) and uses
   the **same names and aggregate mapping** as live `export-tree-status`.
4. Usage text lists the command beside the other `*-status` helpers and lists
   every require flag beside `--export-tree-status`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, import mutation, synthesize,
   quarantine rewrite, lock reservation, artifact deletion, daemon mutation, or
   filesystem walk of the original export-tree / sibling tip-kind files.
7. Never emits DSNs, executable SQL, runtime store rows, or live row payloads.

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `status`) when the
   path is absent.
2. Rejects symlink or non-regular paths, oversized files over **128 KiB**,
   invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON,
   or an unsupported `format` marker with exit `1`, a short stderr reason, and
   **no** stdout status JSON.
3. Input `format` must be `go-metin2-export-tree-status-v1`. A
   `go-metin2-export-tree-status-status-v1` file (this command's own output)
   is the wrong format and must fail closed.
4. `export_tree_status_sha256` is computed over the **exact retained file
   bytes** so operators can correlate the inspected file with lab notes /
   drill trees.

### C. Inner `present: false` retained snapshots

A valid live inspector snapshot for an absent tree is only:

```json
{
  "format": "go-metin2-export-tree-status-v1",
  "present": false
}
```

Rules:

1. That shape is success for ungated inspection.
2. Any selected require flag against inner `present: false` fails closed with
   exit `1`, a short stderr reason that names the failed gate / absent tree,
   and **no** stdout JSON — same mapping as live `export-tree-status`.
3. GREEN should reuse `enforceExportTreeStatusRequireGates` against the decoded
   inner status rather than duplicating the mapping.

### D. Inner `present: true` consistency (no tree walk)

When inner `present` is true, decode into the existing `exportTreeStatus`
shape and fail closed unless **all** of the following hold:

1. `export_tree` is a non-empty absolute cleaned path (same absolute-path
   policy the live inspector records).
2. `kind_count == len(kinds) == len(exportQuarantineKinds)`.
3. `kinds[i].kind` matches `exportQuarantineKinds[i]` in that fixed order.
4. `kinds[i].wipe_kind` is true iff the kind is in `importExportDrillWipeKinds`.
5. Non-wipe kinds omit wipe-only pointers (`wipe_quarantine`,
   `wipe_quarantine_status`, `wipe_import_result`, `wipe_import_result_status`,
   `wipe_import_result_outcome`).
6. Wipe kinds include those four artifact pointers (each may be
   `present: false`).
7. `import_result_outcome` is present iff `import_result.present` is true;
   `wipe_import_result_outcome` is present iff `wipe_import_result.present`
   is true.
8. Completeness / outcome aggregates **recompute** from the decoded kind
   entries and must match the reported values exactly:
   - `quarantine_present_count` / `quarantine_complete`
   - `wipe_quarantine_present_count` / `two_phase_wipe_artifacts_complete`
   - `import_result_present_count` / `import_result_status_present_count` /
     `import_result_artifacts_complete`
   - `import_result_replaced_count` / `import_result_row_count_total` /
     `import_result_outcomes_complete` / `import_result_all_replaced`
   - `wipe_import_result_present_count` /
     `wipe_import_result_status_present_count` /
     `wipe_import_artifacts_complete`
   - `wipe_import_result_replaced_count` /
     `wipe_import_result_row_count_total` /
     `wipe_import_result_outcomes_complete` /
     `wipe_import_result_all_replaced`
9. Recompute uses the same rules as live `inspectExportTreeStatus` (counts from
   `present` bits; outcome totals only from present outcome objects; complete /
   all-replaced bits require the full kind or wipe-kind set). Do **not** reopen
   quarantine / import-result files to re-hash them.
10. After consistency succeeds, apply any selected require flags to the decoded
    inner status with the same failure semantics as live `export-tree-status`
    (exit `1`, no stdout JSON).

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove the original tree still exists or that live DB
rows match.

### E. Successful outer envelope

```json
{
  "format": "go-metin2-export-tree-status-status-v1",
  "present": true,
  "export_tree_status_sha256": "...",
  "status": {
    "format": "go-metin2-export-tree-status-v1",
    "present": true,
    "export_tree": "/var/metin2/exports/20260905T120000Z-abcdef012345",
    "kind_count": 10,
    "quarantine_complete": true,
    "kinds": []
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-export-tree-status-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `status` is the decoded
retained `go-metin2-export-tree-status-v1` object (same field names as live
stdout).

### F. Drill printer wiring (same GREEN as the inspector)

`metin2-migrate import-export-drill` already retains ungated before-status and
gated after-status `export-tree-status` snapshots. GREEN must add a matching
`export-tree-status-status` redirect **immediately after** each of those
lines. Printer remains confirmation-gated and still does not execute status /
import itself.

1. **Before-status** stays ungated live capture, then ungated retained
   inspection:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-before.json"
   metin2-migrate export-tree-status-status --export-tree-status "$EXPORT_TREE/export-tree-status-before.json" > "$EXPORT_TREE/export-tree-status-before-status.json"
   ```
2. **After-status for insert-only and scoped-replace printers** keeps today's
   three live require flags, then inspects the retained after JSON with the
   **same three** flags:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-import-result-artifacts-complete \
     --require-import-result-outcomes-complete \
     > "$EXPORT_TREE/export-tree-status-after.json"
   metin2-migrate export-tree-status-status --export-tree-status "$EXPORT_TREE/export-tree-status-after.json" \
     --require-quarantine-complete \
     --require-import-result-artifacts-complete \
     --require-import-result-outcomes-complete \
     > "$EXPORT_TREE/export-tree-status-after-status.json"
   ```
   Do **not** add wipe-outcome / two-phase-wipe requires here.
3. **After-status for two-phase wipe→roster→reimport** keeps today's eight live
   require flags, then inspects the retained after JSON with the **same
   eight** flags:
   ```sh
   metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" \
     --require-quarantine-complete \
     --require-two-phase-wipe-artifacts-complete \
     --require-import-result-artifacts-complete \
     --require-wipe-import-artifacts-complete \
     --require-import-result-outcomes-complete \
     --require-import-result-all-replaced \
     --require-wipe-import-result-outcomes-complete \
     --require-wipe-import-result-all-replaced \
     > "$EXPORT_TREE/export-tree-status-after.json"
   metin2-migrate export-tree-status-status --export-tree-status "$EXPORT_TREE/export-tree-status-after.json" \
     --require-quarantine-complete \
     --require-two-phase-wipe-artifacts-complete \
     --require-import-result-artifacts-complete \
     --require-wipe-import-artifacts-complete \
     --require-import-result-outcomes-complete \
     --require-import-result-all-replaced \
     --require-wipe-import-result-outcomes-complete \
     --require-wipe-import-result-all-replaced \
     > "$EXPORT_TREE/export-tree-status-after-status.json"
   ```
4. Contrib `lab-retention-gc` forwarding of the printer stays unchanged in this
   freeze (no new env vars required here). Printed scripts still use `set -eu`,
   so a failed after-status-status inspect fails the drill.

### G. Explicit non-goals

- new inner `go-metin2-export-tree-status-v1` fields
- walking / hashing sibling tip-kind files from the original export-tree
- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed synthesize / import / status scripts
- opening a database from `export-tree-status-status`
- claiming status / gate bits prove live DB row presence beyond retained
  import-result / wipe-import-result contracts
- changing default (ungated) missing-file outer `present: false` exit `0`
- adding wipe-outcome requires to insert-only or scoped-replace after-status
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/export_tree_status_status.go` (new)
- `internal/migratecli/export_tree_status_status_test.go` (new)
- `internal/migratecli/export_tree_status.go` (shared consistency helper if needed)
- `internal/migratecli/import_export_drill.go` (before/after status-status redirects)
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `internal/migratecli/migratecli.go` (command switch + usage)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-09-04-cli-export-tree-status.md` (pointer)
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing export-tree-status path → outer `present: false`, no DB open
- valid inner `present: false` snapshot → outer `present: true` + checksum +
  inner status, no DB / tree open
- valid complete all-replaced two-phase after snapshot → outer `present: true`,
  inner aggregates unchanged
- kind-order / wipe-kind / aggregate-drift / missing wipe pointers on a wipe
  kind / outcome present without artifact → exit `1`, empty stdout
- require flags with absent path or inner `present: false` → exit `1`, empty
  stdout
- complete insert-only after snapshot + wipe-outcome require → exit `1`
- symlink / oversized / unknown-field / wrong format (including this command's
  own outer envelope) → exit `1`, empty stdout
- usage / unknown-command mention `export-tree-status-status`
- insert-only / scoped-replace / two-phase printers emit the matching
  before/after status-status redirects with the flags frozen above
- hermetic two-phase after path retains a valid
  `export-tree-status-after-status.json` whose checksum matches the after JSON

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'ExportTreeStatusStatus|ImportExportDrillPrints|RejectsUnknownCommandMentionsExportTreeStatusStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLiteHermeticPrintedScriptTwoPhase' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

Frozen on `lane/persistence` (docs/spec only). GREEN is follow-on.

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / consistency
  rules / drill status-status wiring
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not walk the original export-tree from the status-status command.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
