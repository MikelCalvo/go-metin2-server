# Contrib Print Helper Hermetic Execution Proof — 2026-08-23

## Objective

Prove the tree-owned `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
helper actually dumps companion printer scripts under a hermetic `/bin/sh`
execution against a fake `metin2-migrate` stub — so reconnect/restart and
apply-window operators are not relying on static text markers alone.

The previous slice folded `migration-run-retention` / env-gated
`backup-restore-drill` into the helper and fail-closed on live-fetch markers.
This slice owns runtime behavior of the helper itself without enabling timers,
shelling printed triage scripts, or inventing SQL import.

## Contract frozen by this slice

1. Focused `internal/migratecli` coverage builds a temp PATH with a stub
   `metin2-migrate` that:
   - answers `version` with a fixed commit JSON;
   - answers `artifact-retention-gc` / `migration-run-retention` /
     `backup-restore-drill` by writing recognizable stdout markers;
   - fails closed if invoked with unexpected subcommands.
2. Running the real helper via `/bin/sh` with `METIN2_MIGRATE_BIN` /
   `METIN2_OPS_PRINTS_ROOT` pointing at the hermetic tree always creates
   `migration-run-retention.sh` plus the two artifact-retention-gc scripts and
   a `notes.md` that records the drill skip when `METIN2_RUNTIME_CONFIG` is
   unset.
3. When `METIN2_RUNTIME_CONFIG` points at a non-symlink regular retained
   runtime-config fixture, the helper also writes `backup-restore-drill.sh`
   and notes that it printed from that env var.
4. Symlink / missing `METIN2_RUNTIME_CONFIG` paths skip the drill printer and
   record the skip reason; the helper never invokes `curl`.
5. Automatic execution of printed triage scripts, `rm` of aside trees, FreeBSD
   port enable defaults, SQL import, and remote admin remain deferred.

## What this is not yet

- automatic / scheduled execution of printed aside-rename / backup / apply scripts
- `rm` of `.gc-aside-*` trees
- live scheduled `curl` of ops JSON
- FreeBSD port / `pkg` enable defaults
- SQL import/backfill or driver-backed harness
- ~~hermetic end-to-end HTTP drill against a live drained `gamed`~~ Done: see
  [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md)
- remote admin authentication

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabRetentionGCHelperHermetic' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Keep FreeBSD port / `pkg` enable defaults deferred.
5. ~~Optional later: FreeBSD `periodic(8)` weekly print-only fragment gated on
   `weekly_metin2_artifact_retention_gc_print_enable="NO"`.~~ Done: see
   [contrib FreeBSD periodic retention / GC print sample](2026-08-23-contrib-freebsd-periodic-retention-gc-print-sample.md).
6. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`
   (printer remains read-only; scheduled helper still prefers retained JSON).~~
   Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
7. ~~Optional later: systemd drop-in `.sample` that only documents
   `EnvironmentFile=` / `METIN2_RUNTIME_CONFIG` for a retained runtime-config
   path (still print-only; no live curl; no DSN).~~ Done: see
   [contrib runtime-config EnvironmentFile sample](2026-08-23-contrib-runtime-config-envfile-sample.md).
