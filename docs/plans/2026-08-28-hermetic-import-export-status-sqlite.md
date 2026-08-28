# Hermetic Import-Export Status SQLite Proof — 2026-08-28

## Objective

Extend the already-shipped hermetic `import-export-drill` SQLite proofs so the
printed `/bin/sh` script is also proven to write valid
`import-result-status.json` artifacts via the matching
`import-export-status` redirects — without inventing upsert policy, registering
a stock production driver, auto-running the printer from CLI/contrib/cron, or
opening a second database from the status command.

## Why now

- [CLI import-export status](2026-08-28-cli-import-export-status.md) already
  owns the offline inspector and wires status redirects into the print-only
  drill.
- The empty and seeded hermetic drill proofs still only assert
  `import-result.json` markers after `/bin/sh` execution; they do not prove the
  redirected status files land as present/valid evidence beside each tip kind.
- Release/incident runbooks that retain import trees need confidence that the
  printed status redirects survive PATH wiring end-to-end the same way import
  results already do.

## Contract frozen by this slice

1. Keep the existing empty-payload and seeded hermetic import proofs intact for
   `import-result.json` markers and SELECT round-trips.
2. After successful `/bin/sh` execution of the printed drill script, every tip
   kind subdirectory also contains `import-result-status.json`.
3. Each status file must decode as `go-metin2-import-export-status-v1` with:
   - `present: true`
   - `kind` equal to the tip-kind directory name
   - `import_result_sha256` equal to the SHA-256 of the sibling
     `import-result.json` bytes
   - nested `result` retaining the same tip-kind count markers already asserted
     on `import-result.json` (empty tree stays zero; seeded tree stays non-zero)
4. Status bodies still omit concrete DSN embedding and executable SQL fragments.
5. The status command path remains metadata-only: hermetic proof does not claim
   status re-opens the DB or re-validates live rows beyond the retained result.
6. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency.
7. Docs mark this status-redirect hermetic follow-up done on Track E /
   migration-contract tips; upsert / auto-run / stock driver remain explicitly
   deferred.

## What this is not yet

- automatic / scheduled execution of the printed import / status script
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming a present status file proves live DB contents beyond retained
  metadata

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-28-cli-import-export-status.md` (pointer)
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- this plan

## TDD and validation

Focused coverage under `//go:build sqlite_harness`:

- empty hermetic drill writes present status files for every tip kind
- seeded hermetic drill writes present status files with non-zero nested markers
- checksum matches sibling `import-result.json`
- status bodies never embed a concrete DSN / executable SQL

Validation for this slice:

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

Also keep untagged package green:

```bash
go test ./internal/migratecli -run 'ImportExportStatus|ImportExportDrill' -count=1
```

## Exit criteria

- hermetic printed-script SQLite proofs assert `import-result-status.json`
- prior deferred status-redirect hermetic follow-up marked done
- empty and seeded import-result proofs remain green
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed import/status commands from CLI / contrib / cron.
- Do not embed DSN values in printer stdout, status files, or contrib notes.
- Do not invent upsert / merge policy.
- Do not register a production driver in stock binaries.
- Do not push `origin/main`; push only `origin/lane/persistence`.
