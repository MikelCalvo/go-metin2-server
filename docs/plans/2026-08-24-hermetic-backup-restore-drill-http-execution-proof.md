# Hermetic Backup/Restore Drill HTTP Execution Proof — 2026-08-24

## Objective

Prove the already-shipped `metin2-migrate backup-restore-drill` printer emits a
portable `/bin/sh` script that actually drives the loopback `/local/*` validate /
crash-temp cleanup / backup / backup-validate / aside-rename / restore sequence
against a drained `gamed` ops mux — without inventing remote admin, SQL import,
automatic GC deletion, or changing the printer's read-only contract.

The printer and per-store FileStore / runtime backup primitives already landed.
Operators still lacked a hermetic end-to-end HTTP execution proof that the
printed curl ordering and path wiring succeed against real loopback handlers.

## Why now

- Multiple Track E follow-ups still list "hermetic end-to-end HTTP drill against
  a live drained `gamed`" as the remaining ops gap after daemon-log retention
  correlation and contrib helper forwarding.
- Printer stdout-shape tests and per-store runtime backup tests do not exercise
  the combined printed script against HTTP.
- The PvE reconnect/restart vertical needs confidence that the operator runbook
  script can empty dedicated parents and rematerialize manifested backups when
  `live_selected_character_count` is `0`.

## Contract frozen by this slice

1. Focused `internal/minimal` coverage materializes dedicated parent FileStores
   for all eight bootstrap stores, seeds a drained runtime (no selected
   characters), and serves the gamed persistence ops surface on a loopback
   `httptest.Server`.
2. A second loopback mux serves authd `/local/build-info` (and `/healthz`) so
   the printed dual-daemon identity retain steps succeed.
3. The test prints `backup-restore-drill` from retained runtime-config +
   build-info JSON, pointing `--ops-base-url` / `--authd-ops-base-url` /
   `--backup-base` at the hermetic servers and temp backup root.
4. The printed script is executed under `/bin/sh` (same helper shape as the
   artifact-retention-gc execution proof).
5. After execution:
   - a lab retention tree exists under `--backup-base` with the eight store
     subdirs and backup manifests;
   - live store parents were aside-renamed (`*.aside-*`);
   - restored active stores validate and `/local/persistence/status` reports
     `ok: true` with `live_selected_character_count: 0`.
6. Seeded non-empty account + safebox snapshots round-trip through backup →
   aside → restore (empty sibling stores still prove empty-manifest paths).
7. The printer remains read-only: this slice does not auto-run the drill from
   CLI, does not add remote admin, and does not embed DSNs / SQL.
8. Docs mark the recurring "hermetic HTTP drill" follow-up done on the recent
   ops plans that still listed it open.

## What this is not yet

- automatic / scheduled execution of printed backup / apply / GC scripts
- ~~`rm` of aside-renamed trees~~ Done for confirmation-gated print-only `artifact-gc-aside-purge` — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
- FreeBSD port / `pkg` enable defaults
- remote log shipping / SIEM / metrics exporters
- SQL import/backfill or a driver-backed harness
- inventing a daemon `/local/...` log-download endpoint
- extracting a shared production `BuildGamedOpsHandler` helper (test-local
  wiring is enough for this proof)

## TDD and validation

Focused coverage in `internal/minimal`:

- hermetic dual loopback servers + printed-script execution
- non-empty account/safebox round-trip after restore
- persistence status after restore is healthy and drained
- printer stdout still omits SQL / DSN markers (existing migratecli coverage
  remains the shape gate)

Validation for this slice:

- `go test ./internal/minimal -run 'BackupRestoreDrillHTTP' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
3. Keep FreeBSD port / `pkg` enable defaults deferred.
4. Keep remote log shipping / metrics exporters deferred.
5. Keep SQL import/backfill deferred until a driver-backed harness exists.
6. ~~Optional later: extract a shared gamed ops registration helper so `cmd/gamed`
   and the hermetic drill proof stop duplicating endpoint wiring.~~ Done: see
   [gamed file-store persistence ops registration helper](2026-08-24-gamed-file-store-persistence-ops-registration-helper.md).
