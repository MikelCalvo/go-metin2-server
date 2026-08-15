# CLI Empty Ledger Snapshot Boundary — 2026-08-15

## Objective

Add a tiny `metin2-migrate empty-ledger-snapshot` helper so operators and runbooks can produce the strict "database has no applied migrations yet" artifact without hand-writing JSON and without opening a database.

The existing CLI already supports `catalog`, `status`, `ledger-snapshot`, `plan`, and CLI-only `apply`. Those commands all depend on the same `go-metin2-schema-migrations-ledger-v1` snapshot shape when working from an offline ledger. This slice closes the bootstrap-from-empty gap by making the version-zero snapshot a generated project-owned artifact.

## Contract frozen by this slice

`metin2-migrate empty-ledger-snapshot` accepts no arguments:

```bash
metin2-migrate empty-ledger-snapshot > empty-ledger.json
```

Behavior:

- writes strict `go-metin2-schema-migrations-ledger-v1` JSON to stdout;
- emits an explicit empty `entries: []` array;
- performs no database open, query, transaction begin, SQL exec, migration apply, rollback, daemon call, or filesystem mutation except the operator's own stdout redirection;
- does not expose executable SQL text, runtime store data, DSNs, or apply results;
- rejects any extra argument as usage exit `2`.

The generated artifact is accepted by the existing offline planner:

```bash
metin2-migrate empty-ledger-snapshot \
  | metin2-migrate plan --ledger-snapshot - --target-version latest
```

It is also valid input for `metin2-migrate apply --ledger-snapshot <path|->`, subject to the existing apply guardrails: explicit `--driver`, `--dsn`, target version, bounded snapshot input, catalog/ledger validation, transaction-local ledger verification, and metadata-only output.

## What this is not yet

This slice deliberately does not add:

- a production database driver or default DB engine;
- daemon startup auto-migration;
- daemon-local migration mutation endpoints;
- backup/restore orchestration around mutating migration runs;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

## Why this order

For first-time database initialization, operators need the same explicit preflight artifact as every other apply path. Hand-written `{ "entries": [] }` snapshots are easy to mistype and can drift from the strict decoder. Generating the artifact through the CLI keeps version-zero plans and apply runs tied to the project-owned snapshot format while preserving the rule that mutating migration execution is CLI-only and explicitly requested.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `empty-ledger-snapshot` writes the strict snapshot format with explicit `entries: []`;
- stdout remains metadata-only and omits executable SQL / DSN text;
- the generated output can be piped directly into `metin2-migrate plan --ledger-snapshot - --target-version latest`;
- unexpected positional arguments fail as usage without stdout.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunEmptyLedgerSnapshot -count=1`;
- `go test ./internal/migratecli ./db/migrations -count=1`;
- `go test ./... -count=1 -timeout=120s`;
- `go vet ./...`;
- `gofmt -l .`;
- `git diff --check`.

## Follow-up options

1. Add a production migration runbook that sequences `empty-ledger-snapshot -> plan -> backup/preflight -> apply` for brand-new databases and `status -> ledger-snapshot -> plan -> backup/preflight -> apply` for existing targets.
2. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
3. Add a build-tagged driver-backed integration harness after selecting a concrete DB engine.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
