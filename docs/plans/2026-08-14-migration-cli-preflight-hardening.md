# Migration CLI Preflight Hardening — 2026-08-14

## Objective

Harden the read-only `metin2-migrate plan` operator surface before any mutating migration command exists.

The first CLI slice could already print the embedded catalog and dry-run a plan from an offline ledger snapshot. This follow-up keeps the CLI read-only while making it safer and friendlier for production preflight runbooks: operators can target the embedded catalog tip without manually discovering the numeric version first, and oversized ledger snapshots fail closed before planning.

## Contract frozen by this slice

`metin2-migrate plan` now accepts:

- `--target-version <N>` for an explicit catalog target, including rollback target `0`, and
- `--target-version latest` as an alias for the embedded catalog's latest validated migration version.

Ledger snapshot input is bounded before JSON decode and before dry-run planning:

- snapshots may be read from a file path or stdin with `--ledger-snapshot <path|->`,
- the accepted input body is capped at `64 KiB`, matching the daemon-local offline ledger-snapshot planning endpoint,
- bodies over the cap fail with exit code `1`, produce no plan on stdout, and do not attempt migration planning,
- the CLI remains metadata-only and never prints executable SQL.

Exit-code policy remains unchanged:

- `0` = success,
- `1` = validation/runtime failure such as an invalid or oversized ledger snapshot,
- `2` = usage error such as an unknown command, missing flags, or malformed numeric target.

## What this is not yet

This slice deliberately does not add:

- migration apply/rollback CLI commands,
- DB connection opening from the CLI,
- daemon startup auto-migration,
- `/local/db/migrations/apply`,
- a production database driver dependency or DB engine selection,
- backup/restore orchestration around mutating migration runs,
- DB-backed account/character/item/quest/login-ticket repositories.

The shipped daemon migration endpoints and the CLI migration surface remain read-only.

## Why this order

A future production migration command should be built on a boring, bounded preflight path. The `latest` alias removes one operator footgun from runbooks without changing mutation semantics, while the snapshot cap prevents accidental huge stdin/file input from being treated as an ordinary planning request.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `plan --target-version latest` resolves to the embedded catalog tip,
- oversized ledger snapshots fail closed before stdout output,
- existing explicit target-version and invalid-snapshot behavior remains intact.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRun' -count=1`,
- `go test ./internal/migratecli ./db/migrations -count=1`,
- `go test ./... -count=1 -timeout=120s`,
- `go vet ./...`,
- `gofmt -l .`,
- `git diff --check`.

## Follow-up status — CLI apply boundary

A later same-day slice added the first explicit CLI-only `metin2-migrate apply` command. The command still requires an operator-supplied driver/DSN, a strict offline ledger snapshot, and a target version; daemon-local migration endpoints remain read-only.

## Follow-up options

1. Add a driver-backed integration harness once the project selects a concrete DB engine.
2. Add backup/restore preflight orchestration around mutating migration commands.
3. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
