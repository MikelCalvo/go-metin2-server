# CLI export-quarantine-drill export-tree-status contract freeze — 2026-09-08

## Objective

Freeze a small, print-only **post-quarantine tree-status retain** for
`metin2-migrate export-quarantine-drill`. After the generated script has
retained every tip-kind `export.json` and written matching
`quarantine-export` output, it must inspect the new `$BASE` tree with the
already-owned read-only inspectors:

```bash
metin2-migrate export-tree-status --export-tree "$BASE" --require-quarantine-complete
metin2-migrate export-tree-status-status --export-tree-status "$BASE/export-tree-status.json" --require-quarantine-complete
```

That gives operators one fail-closed evidence artifact that the retained
export/quarantine tree is complete **before** they hand it to
`import-export-drill` or archive it. It does not execute HTTP, quarantine,
or SQL import from the printer, open a database, register a production
driver, or change the existing `export-tree-status` command.

## Why this follow-up is needed

Linked-driver discovery and the later CLI/printer driver preflights are
GREEN:

- [CLI sql-drivers contract freeze](2026-09-07-cli-sql-drivers-contract-freeze.md)
- [CLI direct SQL-driver preflight](2026-09-07-cli-direct-sql-driver-preflight-contract-freeze.md)
- [CLI import-export-drill driver preflight](2026-09-08-cli-import-export-drill-driver-preflight-contract-freeze.md)
- [CLI migration-run-retention driver preflight before DSN](2026-09-08-cli-migration-run-retention-driver-preflight-before-dsn-contract-freeze.md)

Sibling printers already retain matching tree-level inspectors:

- `import-export-drill` retains ungated
  `$EXPORT_TREE/export-tree-status-before.json` and gated after-status
  (quarantine + import-result, plus wipe flags in two-phase mode);
- `backup-restore-drill` retains gated
  `$BASE/backup-tree-status.json` after backup/validate;
- `export-quarantine-drill` already retains
  `$BASE/migration-catalog-status.json`.

The export/quarantine printer still stops after the last
`quarantine-export` line. An operator reviewing
`/var/metin2/exports/YYYYMMDDTHHMMSSZ-<commit12>/` must walk every
tip-kind directory by hand to learn whether quarantine is complete.
`export-tree-status --require-quarantine-complete` already answers that
question; the printer just does not emit it.

This is the remaining Track E operator-runbook gap on the PvE
login → map → reward → reconnect/restart → local-ops path: a retained
roster / item / point / quest / safebox / ground / ticket / template /
static-content / myshop export tree should fail closed when any
`quarantine.json` is missing **before** anyone pastes that tree into
`import-export-drill`.

Opening RED without freezing filenames, require-flag subset, placement,
partial-run redirect behavior, and hermetic proof bounds would invent
those operator-facing stop/go semantics mid-implementation. Freeze first;
GREEN stays follow-on.

## Contract to freeze before GREEN

### A. Printer-only scope

Keep the command and all existing flags unchanged:

```bash
metin2-migrate export-quarantine-drill \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--authd-ops-base-url http://127.0.0.1:6061] \
  [--export-base /var/metin2/exports] \
  [--gamed-log-path /var/log/metin2/gamed.log] \
  [--authd-log-path /var/log/metin2/authd.log]
```

The printer continues to accept an opaque build-info snapshot and print a
`set -eu` script. At generation time it must not call
`export-tree-status`, walk `$BASE`, open or ping a database, execute
HTTP, or write retention files. That preserves use of the printer on a
build host distinct from the later execution host.

No new printer flag, env var, contrib helper, or inner
`go-metin2-export-tree-status-v1` field is introduced.

### B. Required printed ordering

After the last `quarantine-export` line (all ten tip kinds already
printed by `exportQuarantineDrillKinds`), the script must emit:

```sh
echo '== retain export-tree-status after quarantine =='
metin2-migrate export-tree-status \
  --export-tree "$BASE" \
  --require-quarantine-complete \
  > "$BASE/export-tree-status.json"
metin2-migrate export-tree-status-status \
  --export-tree-status "$BASE/export-tree-status.json" \
  --require-quarantine-complete \
  > "$BASE/export-tree-status-status.json"
```

Rules:

1. Placement is **after** every kind's `curl` export retain and
   `quarantine-export` redirect, and **after** the existing catalog /
   catalog-status / notes retains. Do not insert tree-status between
   kinds.
2. `--export-tree` is `"$BASE"` (the timestamped export tree created
   earlier as `"${EXPORTS_BASE}/${TS}-${COMMIT12}"`). That path is
   already absolute because `--export-base` is an absolute cleaned path.
3. Filenames are `$BASE/export-tree-status.json` and
   `$BASE/export-tree-status-status.json`. Do **not** use
   `export-tree-status-before.json` / `export-tree-status-after.json`:
   this printer is not a mutation window and has no pre-quarantine tree
   to inspect.
4. The only require flag on both commands is
   `--require-quarantine-complete`. Do **not** print
   `--require-two-phase-wipe-artifacts-complete`,
   `--require-import-result-artifacts-complete`,
   `--require-wipe-import-artifacts-complete`,
   `--require-import-result-outcomes-complete`,
   `--require-import-result-all-replaced`,
   `--require-wipe-import-result-outcomes-complete`, or
   `--require-wipe-import-result-all-replaced`. Those aggregates stay
   false on a successful export/quarantine tree because this printer
   never writes import-result or wipe artifacts.
5. The command text must remain `metin2-migrate export-tree-status` /
   `export-tree-status-status` (PATH-selected CLI), not a loopback curl.
   The same CLI binary that just wrote `quarantine.json` is the one that
   re-validates the tree.

### C. Success behavior

For a drained loopback export of every tip kind:

- each `$BASE/<kind>/export.json` and `$BASE/<kind>/quarantine.json`
  remain as today;
- `$BASE/export-tree-status.json` is a present
  `go-metin2-export-tree-status-v1` envelope with
  `quarantine_complete: true` and `kind_count` equal to
  `len(exportQuarantineKinds)` (currently 10, including
  `character-myshop-unit-prices`);
- missing import/wipe children stay ungated `present: false` inside the
  kind entries;
- `$BASE/export-tree-status-status.json` is a present
  `go-metin2-export-tree-status-status-v1` envelope whose inner status
  checksum matches the exact tree-status file bytes;
- existing catalog-status, identity, runtime-config, notes, and optional
  daemon-log retains are unchanged.

A passing `--require-quarantine-complete` proves only that every tip kind
has a present valid `quarantine.json` according to the inspecting CLI.
It does **not** prove live FileStore health, drained sessions, target DSN
reachability, schema compatibility, or that a later SQL import is safe.

### D. Failure behavior and partial-run boundary

The printed script remains `set -eu`. Failures keep this order:

1. Existing identity / catalog / per-kind export or quarantine failures
   still stop before tree-status runs.
2. A present tree whose `quarantine_complete` is false (missing kind
   directory, missing `quarantine.json`, or a kind the inspector does
   not count) reaches `export-tree-status --require-quarantine-complete`
   and exits `1` with the existing inspector stderr. No later
   `export-tree-status-status` line runs.
3. A valid tree-status file whose companion inspect fails is a
   follow-on inspector error, not a reason to skip writing
   `export-tree-status.json`.

Shell redirection happens before the inspector subprocess runs.
Therefore a `--require-quarantine-complete` failure can leave an empty
or incomplete `$BASE/export-tree-status.json` next to the earlier
per-kind files. That is an expected **failed partial run tree**, not a
valid tree-status artifact. The script's nonzero exit is authoritative;
an operator must discard/restart the partial tree after correcting the
missing kind, quarantine input, or CLI binary. GREEN must not silently
add cleanup, retry, automatic artifact removal, or a new status schema
merely to make that partial failure look successful.

`export-quarantine-drill` itself stays print-only: rendering the script
still does not inspect a tree, open a database, contact an ops endpoint,
or require a DSN.

### E. Contrib / hermetic / sibling-printer boundaries

- Contrib `lab-retention-gc` forwarding stays unchanged (no new env
  vars). `export-quarantine-drill.sh` dumps this printer, so GREEN
  output is inherited automatically.
- `import-export-drill` before/after `export-tree-status-*.json` wiring
  is **not** changed. Operators may still run that printer against the
  same `$BASE` later; the new `$BASE/export-tree-status.json` is
  export/quarantine evidence, not an import-result snapshot.
- `backup-restore-drill`, `migration-run-retention`, direct SQL command
  gates, and loopback `/local/db/drivers` are not modified.
- Do not reorder `exportQuarantineDrillKinds` onto `exportQuarantineKinds`
  in this slice. Both lists already cover the same ten kinds;
  `quarantine_complete` does not depend on walk order. Kind-order
  alignment stays a separate follow-up if operators need it.

## TDD plan for GREEN

1. Untagged `export-quarantine-drill` printer tests assert the exact
   gated `export-tree-status` / `export-tree-status-status` pair after
   the last `quarantine-export` line and before EOF. Existing tests
   continue to assert no SQL / DSN text.
2. Ordering markers prove catalog-status → notes → first kind export →
   last kind quarantine → tree-status → tree-status-status.
3. Printer stdout must **not** contain import-result or wipe require
   flags, `export-tree-status-before.json`, or
   `export-tree-status-after.json`.
4. Hermetic `TestExportQuarantineDrillHTTPExecutesAgainstDrainedGamedOps`
   keeps today's dual-mux `/bin/sh` proof and additionally asserts:
   - `$BASE/export-tree-status.json` and
     `$BASE/export-tree-status-status.json` exist as regular files;
   - decoded tree-status `quarantine_complete` is true and `kind_count`
     equals `len(exportQuarantineKinds)` (full ten-kind set, including
     `character-myshop-unit-prices`, which the current assertion loop
     does not name);
   - decoded tree-status-status `present` is true and its checksum
     matches the tree-status file bytes;
   - import-result / wipe children stay `present: false`;
   - printer + retained JSON still omit SQL / DSN markers.
5. Existing `export-tree-status`, `export-tree-status-status`,
   `import-export-drill`, and catalog-status tests remain green. Their
   independent contracts are not changed by this renderer-only
   follow-up.

Expected validation after GREEN:

```bash
go test ./internal/migratecli -run 'ExportQuarantineDrill|ExportTreeStatus|RejectsUnknownCommandMentionsExportQuarantineDrill' -count=1
go test ./internal/minimal -run 'ExportQuarantineDrillHTTP' -count=1
gofmt -l internal/migratecli/export_quarantine_drill.go internal/migratecli/export_quarantine_drill_test.go internal/minimal/export_quarantine_drill_http_test.go
git diff --check
```

## Likely files for GREEN

- `internal/migratecli/export_quarantine_drill.go`
- `internal/migratecli/export_quarantine_drill_test.go`
- `internal/minimal/export_quarantine_drill_http_test.go`
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-25-cli-export-quarantine-drill-printer.md`
- `docs/plans/2026-08-25-hermetic-export-quarantine-drill-http-execution-proof.md`
- this plan (flip freeze → Done)

## Status

Frozen on `lane/persistence`. GREEN is follow-on and must not claim
`$BASE/export-tree-status.json` exists until the printer actually emits
the redirects.

## Exit criteria for this freeze

- exact command, placement, filenames, and require-flag subset are
  explicit;
- generation-time versus script-execution-time behavior is distinguished;
- successful retained-artifact behavior is distinguished from a failed
  partial preflight redirect;
- hermetic HTTP proof is required to decode `quarantine_complete` over
  the full ten-kind set;
- no production Go code, production driver registration, target DSN,
  daemon mutation surface, or import-result require flag is introduced
  by this freeze.

## Anti-goals / ordering constraints

- Do not implement GREEN before this freeze is committed.
- Do not change `import-export-drill`, `backup-restore-drill`,
  `migration-run-retention`, or the `export-tree-status` inspector
  contract in this slice.
- Do not print import-result or wipe require flags on an
  export/quarantine tree.
- Do not treat `quarantine_complete` as proof of live DB rows, FileStore
  restoreability, or a safe SQL import.
- Do not select/register a production driver or auto-run the printed
  script.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
