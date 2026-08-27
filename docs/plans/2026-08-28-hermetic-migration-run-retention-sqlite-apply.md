# Hermetic Migration-Run Retention SQLite Apply Proof — 2026-08-28

## Objective

Make the already-shipped `metin2-migrate migration-run-retention` printer emit a
portable `/bin/sh` script that operators can actually execute end-to-end against a
build-tagged SQLite database for both forward apply (`--target-version latest`)
and rollback apply (`--allow-rollback --target-version 0`) — without inventing
automatic CLI execution, registering a stock production driver, upsert policy,
or a daemon mutation route.

Today the printer always emits an unconditional
`metin2-migrate apply-lock-aside ...` line after the mutating apply block. After
a successful apply the lock file is removed, so `apply-lock-aside` fails closed
and `set -eu` aborts the printed script before post-apply retention completes.
That makes the runbook script non-executable even though the comments already
say the aside step is optional and only for leftover-lock recovery.

## Why now

- Track E / migration-contract tips still name production-engine selection /
  operator runbook hardening beyond the SQLite harness as the remaining gap
  after seeded hermetic `import-export-drill`.
- Printer stdout-shape tests and fake-driver apply CLI coverage do not exercise
  the combined printed PATH wiring + `DRIVER`/`DSN` env indirection +
  plan/preflight/apply/audit retention against a real migrate binary + SQLite.
- The PvE durable-state / migration-window vertical needs confidence that the
  operator runbook script reaches catalog tip (and can roll back to zero) without
  hand-editing the printed lock-triage footer.

## Contract frozen by this slice

1. Keep the existing printer command surface and forward/rollback artifact names.
2. Treat a missing `schema_migrations` relation as an empty applied ledger in
   `ReadSQLLedgerEntries` / `LedgerSnapshotFromSQLLedger` (and therefore in
   CLI `ledger-snapshot` / `status` and daemon-configured ledger reads):
   - portable fail-open only for clearly missing-table diagnostics that name
     `schema_migrations` (SQLite `no such table`, Postgres/MySQL
     `does not exist` / `doesn't exist`);
   - all other query/scan/iteration failures stay fail-closed;
   - this matches apply's already-owned version-zero pre-read skip and makes
     first-time printed `ledger-snapshot` plus post-rollback-to-zero `status`
     executable without hand-editing the runbook script.
3. Change only the optional stale-lock triage footer so a successful apply script
   stays green under `set -eu`:
   - print `apply-lock-status` only when `"$RUN/$LOCK_FILE"` still exists;
   - do **not** auto-run `apply-lock-aside` in the printed script;
   - print the aside command as an operator-run hint / commented example that
     still names `--i-confirm-lab-aside-rename` and the retained JSON path;
   - when no leftover lock exists, print a short non-fatal note that this is
     expected after successful apply.
4. Add build-tagged coverage under `internal/migratecli`
   (`//go:build sqlite_harness`) that:
   - builds `./cmd/metin2-migrate` with `-tags=sqlite_harness`;
   - stubs `curl` on `PATH` for the printed dual-daemon identity /
     runtime-config / persistence-status / daemon-migrations-status retains;
   - prints forward `migration-run-retention` against an absolute temp
     `--migration-runs-base` and executes the printed script under `/bin/sh`
     with `PATH` including the tagged binary + curl stub, `DRIVER=sqlite`, and
     `DSN` pointing at a temp empty SQLite file;
   - asserts the retained run tree contains catalog / ledger-snapshot /
     plan-artifact / apply-preflight / migration-apply-audit /
     post-apply-status JSON and that the live ledger is at catalog tip;
   - prints rollback `migration-run-retention --allow-rollback --target-version 0`
     against a tip-applied SQLite DB and executes it, asserting
     `post-rollback-status.json` / live ledger at version `0` and rollback
     artifact names (`rollback-plan-artifact.json`,
     `rollback-apply-preflight.json`, `migration-rollback-audit.json`,
     `post-rollback-status.json`, default `migration-rollback.lock` path).
5. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency
   and continues to own printer stdout-shape coverage, including the new
   conditional / commented aside footer markers.
6. Focused `db/migrations` coverage (build-tagged SQLite harness) proves missing
   `schema_migrations` reads as an empty ledger snapshot / status plan, while
   genuine query failures stay fail-closed.
7. Docs mark the hermetic printed-script apply/rollback follow-up done on the
   migration-run-retention / rollback-retention / Track E / migration-contract
   tips; upsert / auto-run / stock production driver remain explicitly deferred.

## What this is not yet

- automatic / scheduled execution of the printed apply / rollback script from
  CLI / contrib / cron / periodic
- flipping any timer / cron / `periodic` enable flag to YES by default
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- FreeBSD port / `pkg` enable defaults
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- inventing distributed advisory locks beyond the local lock file

## Likely files to change

- `db/migrations/ledger.go` (missing `schema_migrations` → empty ledger)
- `db/migrations/ledger_test.go` and/or `db/migrations/sqlite_harness_test.go`
- `internal/migratecli/migration_run_retention.go`
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go` (new;
  build-tagged)
- `docs/plans/2026-08-21-cli-migration-run-retention.md`
- `docs/plans/2026-08-21-cli-migration-rollback-run-retention.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- this plan

## TDD and validation

Focused coverage:

- missing `schema_migrations` returns an empty ledger / empty snapshot
- untagged printer tests expect the conditional leftover-lock status block and
  the commented / hinted aside-rename command (no unconditional live
  `apply-lock-aside` invocation under `set -eu`)
- forward path still omits `--allow-rollback` and keeps forward artifact names
- rollback path still prints rollback artifact names + `--allow-rollback`
- hermetic tagged binary + printed-script `/bin/sh` forward apply reaches tip
- hermetic tagged binary + printed-script `/bin/sh` rollback reaches version `0`
- printer stdout never embeds a concrete DSN value

Validation for this slice:

```bash
go test ./db/migrations -run 'Ledger|MissingSchema' -count=1
go test -tags=sqlite_harness ./db/migrations -run 'SQLiteHarness' -count=1
go test ./internal/migratecli -run 'MigrationRunRetention|RejectsUnknownCommand' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l db/migrations/ledger.go internal/migratecli/migration_run_retention.go internal/migratecli/migration_run_retention_test.go internal/migratecli/migration_run_retention_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- missing `schema_migrations` reads as empty ledger for snapshot/status
- successful printed apply/rollback scripts no longer die on missing lock aside
- hermetic printed-script SQLite forward + rollback proofs are green under
  `-tags=sqlite_harness`
- prior deferred runbook-hardening follow-ups mark this proof done
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed apply/rollback from CLI / contrib / cron.
- Do not embed DSN values in printer stdout.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
