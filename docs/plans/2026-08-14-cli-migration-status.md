# CLI Migration Status Boundary — 2026-08-14

## Objective

Add a read-only `metin2-migrate status` command so operators can inspect a database target's current `schema_migrations` ledger and the resulting dry-run migration plan without using a running `gamed` loopback endpoint and without creating an intermediate ledger-snapshot file.

The repo already had daemon-local read-only migration status, strict CLI ledger-snapshot export, offline snapshot planning, and CLI-only apply. This slice adds the missing direct CLI preflight: `status` reads the live ledger metadata and prints the same metadata-only `Plan` shape used everywhere else.

## Contract frozen by this slice

`metin2-migrate status` accepts:

```bash
metin2-migrate status \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  [--target-version <version|latest>]
```

Behavior:

- requires both `--driver` and `--dsn`;
- defaults `--target-version` to `latest`;
- accepts explicit numeric targets, rollback target `0`, and `latest` for the embedded catalog tip;
- opens the operator-supplied `database/sql` target only long enough to query `schema_migrations` metadata;
- reads only `version`, `name`, and `up_sha256` through the existing strict ledger-snapshot helper;
- prints the metadata-only `db/migrations.Plan` JSON shape to stdout;
- never emits executable SQL text, runtime store payloads, apply results, or the supplied DSN;
- redacts the supplied DSN from runtime errors before printing stderr;
- performs no transaction begin, SQL exec, migration apply, rollback, or daemon mutation.

Exit-code policy stays aligned with the rest of the CLI:

- `0` = success;
- `1` = runtime/validation failure such as unknown driver, query failure, row scan failure, invalid ledger metadata, catalog drift, or target outside the embedded catalog boundary;
- `2` = usage error such as missing driver/DSN, extra positional arguments, or malformed numeric targets.

## What this is not yet

This slice deliberately does not add:

- a bundled production database driver or default database engine;
- daemon startup auto-migration;
- daemon-local migration mutation endpoints;
- backup/restore orchestration around mutating migration runs;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

`status` is intentionally read-only. `metin2-migrate apply` remains the only shipped mutating migration command, and it still requires an explicit offline ledger snapshot instead of implicitly trusting a just-read status result.

## Why this order

The prior CLI workflow could already run `ledger-snapshot -> plan -> apply`, but a quick production preflight still required either daemon-local ops access or two CLI commands and a temporary snapshot file. Direct `status` gives operators a simple read-only check while preserving the stricter snapshot artifact requirement for mutation.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `status` reads `schema_migrations` through a registered test `database/sql` driver and writes a metadata-only plan;
- the command performs only an open/query/close path and does not begin a transaction, execute SQL, commit, or roll back;
- omitted `--target-version` defaults to `latest`;
- rollback target `0` returns down-plan metadata;
- missing driver/DSN fails as usage;
- runtime errors redact the supplied DSN.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunStatus' -count=1`;
- `go test ./internal/migratecli ./db/migrations -count=1`;
- `go test ./... -count=1 -timeout=120s`;
- `go vet ./...`;
- `gofmt -l .`;
- `git diff --check`.

## Follow-up options

1. Add a build-tagged driver-backed integration harness after selecting a concrete DB engine.
2. Add a migration runbook that sequences `status -> ledger-snapshot -> plan -> backup/preflight -> apply` for production-style operator drills.
3. Add advisory locking or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
