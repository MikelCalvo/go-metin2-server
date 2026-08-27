# SQLite schema_migrations driver-backed harness — 2026-08-25

## Objective

Add a build-tagged, test-only real-SQL harness that opens a temporary SQLite
database, applies the embedded migration catalog through the existing
programmatic apply primitive, and proves live `schema_migrations` round-trips
through `ReadSQLLedgerEntries`, `LedgerSnapshotFromSQLLedger`, and
`PlanUpToLatestFromSQLLedger`.

This closes migration-contract follow-up #3 and the deferred driver-backed
integration gate from the DB driver startup-preflight plan — without selecting a
production engine, shipping a stock release driver, importing quarantined
exports into SQL, or folding destructive purge into scheduled helpers.

## Why now

- Track E item 1 still waits on a ready repository/backfill contract; MemoryStore
  seams and tip-`0015` quarantine/export are green, but no real-engine ledger
  proof exists.
- Docs repeatedly defer SQL import/backfill "until a driver-backed harness
  exists." Fake `database/sql/driver` stubs already cover unit paths; the missing
  cohesive step is real ledger I/O.
- HEAD already owns confirmation-gated `artifact-gc-aside-purge`, hermetic
  backup/export drills, and print-only retention samples. The next unlock for
  durable DB ops is the harness gate, not another retention printer.

## Contract frozen by this slice

1. Tests live under `//go:build sqlite_harness` so default `go test ./...` stays
   free of engine selection and does not require the SQLite dependency at
   compile time for stock CI.
2. The harness links pure-Go SQLite (`modernc.org/sqlite`) only under that tag.
   Stock `gamed` / `authd` / `metin2-migrate` release builds still ship with no
   registered production driver.
3. Temp-file DSN only (`file:<tmpdir>/schema-migrations-harness.sqlite?...`).
   No embedded production DSN, passwords, or host credentials.
4. Empty DB → `ApplyUpToLatest` with an empty caller ledger → catalog tip
   (`0015_character_safebox_money`) is applied in one transaction when the
   catalog SQL is SQLite-compatible; if tip apply is blocked by an engine quirk,
   the harness still must prove at least `0001_bootstrap_schema_migrations`
   apply + ledger read + snapshot + empty pending plan (and document any tip
   blocker honestly).
5. After a successful apply boundary:
   - `ReadSQLLedgerEntries` returns version/name/`up_sha256` matching the
     catalog prefix that was applied;
   - `LedgerSnapshotFromSQLLedger` encodes the same metadata-only snapshot
     shape (`go-metin2-schema-migrations-ledger-v1`);
   - `PlanUpToLatestFromSQLLedger` reports no pending ups at that boundary.
6. Optional narrow proof: rollback to version `0` then re-apply `0001` only,
   confirming down/up ledger mutations against the real engine.
7. Foreign keys are enabled for the harness connection (`PRAGMA foreign_keys=ON`)
   so catalog FK constraints are exercised.
8. Docs mark migration-contract follow-up #3 and the driver-startup-preflight
   harness follow-up done; SQL import/backfill, production engine selection,
   automatic / scheduled purge execution, and FreeBSD port enable defaults
   remain deferred. ~~Folding `artifact-gc-aside-purge` into
   `contrib/lab-retention-gc`~~ Done — see
   [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).

## What this is not yet

- SQL import/backfill from quarantined `0010` / `0014` / `0015` exports
- production DB engine selection as a stock default / bundled release driver
- new catalog tip `0016+`
- SQL-backed runtime repositories
- ~~folding `artifact-gc-aside-purge` into `contrib/lab-retention-gc`~~ Done — see
  [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md)
- automatic/scheduled purge or GC deletion daemons
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing, multi-host orchestration

## Likely files to change

- `db/migrations/sqlite_harness_test.go` (new, `//go:build sqlite_harness`)
- `go.mod` / `go.sum` (SQLite test dependency retained via
  `go mod tidy -tags=sqlite_harness`)
- `docs/plans/2026-08-09-db-migration-contract.md` (follow-up #3)
- `docs/plans/2026-08-13-db-driver-startup-preflight.md` (follow-up #1)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief harness mention)
- `docs/workflow/migration-apply-runbook.md` (optional one-line pointer)
- this plan

## TDD and validation

Focused coverage under the build tag:

- empty SQLite DB apply to catalog tip (or documented `0001` floor) succeeds
- ledger rows match catalog `version` / `name` / `up_sha256`
- ledger snapshot JSON shape is metadata-only
- plan-from-SQL-ledger at the applied boundary has empty pending ups
- optional rollback-to-zero + re-apply `0001`
- stdout/stderr of the harness never embeds a production DSN/password

Validation for this slice:

```bash
go test -tags=sqlite_harness ./db/migrations -run 'SQLiteHarness' -count=1
go test ./db/migrations ./internal/migratecli -count=1   # stock path stays green
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- build-tagged harness green against a real SQLite ledger
- migration-contract follow-up #3 marked done
- Track E docs name the harness as the prerequisite before SQL import/backfill
- default untagged `go test ./...` still does not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent INSERT/backfill policy for quarantined exports this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
