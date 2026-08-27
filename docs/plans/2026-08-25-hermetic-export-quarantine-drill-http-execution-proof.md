# Hermetic Export-Quarantine Drill HTTP Execution Proof — 2026-08-25

## Objective

Prove the already-shipped `metin2-migrate export-quarantine-drill` printer emits a
portable `/bin/sh` script that actually drives the loopback export retain →
offline `quarantine-export` handoff across every tip-`0015` migration-shaped
export kind against drained `gamed` / `authd` ops muxes — without inventing
automatic CLI execution, SQL import/backfill, remote admin, or changing the
printer's read-only contract.

The printer and the hermetic HTTP → offline CLI (direct `quarantine-export`
invocation) proofs already landed. Operators still lacked a hermetic end-to-end
`/bin/sh` execution proof that the printed curl ordering, retention-tree
layout, dual-daemon identity retain, and `metin2-migrate quarantine-export`
PATH wiring succeed together.

## Why now

- [CLI export → offline quarantine drill printer](2026-08-25-cli-export-quarantine-drill-printer.md)
  explicitly deferred “hermetic `/bin/sh` execution proof against drained
  loopback ops muxes”.
- Printer stdout-shape tests and the offline CLI proof do not exercise the
  combined printed script against HTTP + a real `metin2-migrate` binary on
  `PATH`.
- The PvE reconnect / migration-window vertical needs confidence that the
  operator runbook script retains all nine tip-`0015` kinds (including seeded
  roster + safebox money) under `/var/metin2/exports`-shaped trees.

## Contract frozen by this slice

1. Focused `internal/minimal` coverage materializes dedicated parent FileStores,
   seeds a drained runtime with:
   - one account/character roster row
   - one durable safebox snapshot including warehouse `money` (tip `0015`)
2. The test serves:
   - `RegisterGamedFileStorePersistenceOps(ops.NewPprofMux("gamed"), runtime)`
     then `RegisterGamedMigrationQuarantineExportOps(...)` on a loopback
     `httptest.Server` (runtime-config + migration catalog + tip-`0015` exports)
   - a second loopback mux via `ops.NewPprofMux("authd")` for dual-daemon
     build-info retain
3. The test builds `./cmd/metin2-migrate` into a temp `PATH` directory so the
   printed `metin2-migrate quarantine-export ...` lines resolve.
4. The test prints `export-quarantine-drill` from retained build-info JSON,
   pointing `--ops-base-url` / `--authd-ops-base-url` / `--export-base` at the
   hermetic servers and temp export root (missing daemon logs stay non-fatal).
5. The printed script is executed under `/bin/sh` (same helper shape as the
   backup-restore drill HTTP proof), with `PATH` including the temp migrate
   binary plus host `curl`.
6. After execution:
   - a lab retention tree exists under `--export-base` ending in the stamped
     12-char commit suffix
   - correlation files exist: `gamed-build-info.json`, `authd-build-info.json`,
     `runtime-config.json`, `migration-catalog.json`, `notes.md`
   - every tip-`0015` kind subdirectory contains both `export.json` and
     `quarantine.json`
   - roster quarantine retains the seeded login/name; safebox quarantine
     retains tip `0015_character_safebox_money` and the seeded warehouse money
7. Printed / executed stdout still omit SQL / DSN markers.
8. The printer remains read-only: this slice does not auto-run the drill from
   CLI, does not fold the printer into `contrib/lab-retention-gc`, and does not
   add SQL import/backfill.
9. Docs mark the printer plan's hermetic `/bin/sh` follow-up done.

## What this is not yet

- automatic / scheduled execution of the printed export-quarantine script
- ~~folding `export-quarantine-drill` into `contrib/lab-retention-gc` print-only
  samples~~ Done: see
  [contrib export-quarantine drill print helper](2026-08-25-contrib-export-quarantine-drill-print-helper.md)
- SQL import/backfill from quarantined exports
- ~~`rm` of aside-renamed trees~~ Done for confirmation-gated print-only `artifact-gc-aside-purge` — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution remains deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).
- FreeBSD port / `pkg` enable defaults
- remote log shipping / metrics exporters
- remote admin authentication
- changing export schemas, quarantine validators, or migration tip `0015`

## Likely files to change

- `internal/minimal/export_quarantine_drill_http_test.go` (new)
- `docs/plans/2026-08-25-cli-export-quarantine-drill-printer.md`
- `docs/plans/2026-08-25-hermetic-export-quarantine-offline-cli-proof.md`
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md`
- this plan

## TDD and validation

Focused coverage in `internal/minimal`:

- hermetic dual loopback servers + printed-script `/bin/sh` execution
- all nine tip-`0015` kind trees retain `export.json` + `quarantine.json`
- seeded roster + tip-`0015` safebox money round-trip through the printed path
- correlation files present; SQL/DSN markers absent from printer + quarantine
  outputs

Validation for this slice:

- `go test ./internal/minimal -run 'ExportQuarantineDrillHTTP' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- hermetic printed-script execution proof is green
- printer / offline-proof plans mark the deferred `/bin/sh` follow-up done
- docs point operators at this proof beside the printer

## Anti-goals / ordering constraints

- Do not auto-run the printed script from CLI.
- Do not widen registration helpers or change endpoint paths/bodies.
- Do not add SQL import/backfill.
- Do not push `origin/main`; push only `origin/lane/persistence`.
