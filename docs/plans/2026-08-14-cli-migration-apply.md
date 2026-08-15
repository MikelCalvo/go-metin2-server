# CLI Migration Apply Boundary — 2026-08-14

## Objective

Add the first mutating migration command surface as a CLI-only operator tool while keeping shipped daemon ops endpoints read-only and avoiding any claim that gameplay stores are DB-backed.

The project already had a validated migration catalog, offline ledger snapshots, metadata-only dry-run planning, and a package-level transactional up/down apply primitive. This slice wires those pieces into `metin2-migrate apply` with explicit operator inputs and guardrails.

## Contract frozen by this slice

`metin2-migrate apply` now accepts:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest>
```

Behavior:

- requires both `--driver` and `--dsn`;
- requires a strict offline `go-metin2-schema-migrations-ledger-v1` snapshot from a file path or stdin, including the generated `metin2-migrate empty-ledger-snapshot` artifact for version-zero initialization;
- caps ledger snapshot input at 64 KiB before decoding, planning, opening the database, or applying SQL;
- accepts explicit numeric targets, rollback target `0`, and `latest` for the embedded catalog tip;
- reuses `db/migrations.ApplyToVersion(...)`, so up and down steps run inside one transaction and transaction-local ledger verification still applies;
- writes only metadata-only `ApplyResult` JSON to stdout (`previous_version`, `current_version`, `latest_version`, and applied plan-step metadata);
- redacts the supplied DSN from apply errors before printing stderr.

Exit-code policy stays aligned with the existing CLI:

- `0` = success;
- `1` = validation/runtime failure such as invalid ledger snapshot, driver/database failure, migration SQL failure, or ledger verification failure;
- `2` = usage error such as missing driver/DSN/snapshot/target flags or malformed numeric targets.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or another daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default database engine;
- production backup/restore orchestration around a mutating run;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

The daemon-local migration endpoints remain read-only: catalog, status, explicit target plan, ledger snapshot export, and offline ledger-snapshot planning.

## Why this order

A CLI-only command is the smallest useful mutation boundary after the package-level primitive. It gives operators and CI/local QA a real executable path without widening the daemon ops surface or silently migrating databases at service startup.

Requiring an offline ledger snapshot keeps the apply run tied to an explicit preflight artifact. Operators can export or assemble the snapshot, inspect a dry-run plan, and then pass the exact metadata boundary into the mutating command.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `apply` executes an up target against a registered `database/sql` driver;
- rollback target `0` uses an offline ledger snapshot and executes the down path;
- missing `--ledger-snapshot` fails as usage before opening the DB;
- missing driver/DSN fails as usage;
- oversized ledger snapshots fail before opening the DB;
- apply errors redact the supplied DSN;
- stdout remains metadata-only and omits executable SQL and DSNs.

Validation for this slice:

- `go test ./internal/migratecli ./db/migrations -count=1`;
- `go test ./internal/migratecli -run 'TestRunApply' -count=1`;
- `go test ./... -count=1 -timeout=120s`;
- `go vet ./...`;
- `gofmt -l .`;
- `git diff --check`.

## Follow-up options

1. Add a build-tagged driver-backed integration harness after selecting a concrete DB engine for production migration tests.
2. Add backup/restore preflight orchestration around `metin2-migrate apply` before recommending it for real deployments.
3. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
