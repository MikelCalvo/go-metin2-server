# CLI Migration Apply Preflight — 2026-08-16

## Objective

Add a read-only CLI command that validates the exact offline inputs an operator intends to pass to `metin2-migrate apply` before opening any database target, reserving a lock file, or creating an audit artifact.

The project already had strict ledger snapshots, metadata-only dry-run plans, plan artifacts, rollback confirmation guards, optional local locks, optional audit files, and a CLI-only mutating `apply` command. This slice adds a final no-side-effect check for production runbooks: confirm that the ledger snapshot, target version, rollback acknowledgement, and reviewed plan artifact still match before the mutating step.

## Contract frozen by this slice

`metin2-migrate apply-preflight` accepts:

```bash
metin2-migrate apply-preflight \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  [--plan-sha256 <hex> | --plan-artifact <path>] \
  [--allow-rollback]
```

On success it writes a metadata-only JSON document with format marker `go-metin2-migration-apply-preflight-v1`:

- `target_version` — resolved numeric target, so `latest` becomes the embedded catalog tip;
- `target_latest` — whether the operator requested `latest`;
- `ledger_snapshot_sha256` — checksum over the exact offline ledger-snapshot bytes supplied to the preflight command;
- `plan_sha256` — checksum over the exact indented dry-run plan JSON;
- `plan` — the same metadata-only `db/migrations.Plan` shape emitted by `plan` / `plan-artifact`.

The command:

- reads the strict offline `go-metin2-schema-migrations-ledger-v1` snapshot from a file or stdin;
- caps ledger-snapshot input at the existing 64 KiB boundary before decoding or planning;
- recomputes the exact dry-run plan for the supplied target;
- validates either `--plan-sha256` or a strict `go-metin2-migration-plan-artifact-v1` file when provided;
- rejects rollback/down plans unless `--allow-rollback` is present;
- rejects rollback/down plans unless a plan checksum or plan artifact is also present;
- never accepts a driver or DSN;
- never opens a database;
- never creates a lock file;
- never creates an audit file;
- never emits executable SQL.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- backup/restore orchestration inside the CLI;
- lock-file stale inspection or recovery;
- DB advisory locking;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

`apply-preflight` is a runbook guard, not a backup, not a lock, and not a substitute for the final `apply` validations. The mutating `apply` command still re-validates the ledger snapshot, target, rollback acknowledgement, plan confirmation, optional lock, optional audit file, and transaction-local database ledger before committing.

## Why this order

The previous safe runbook already had all core pieces, but its final validation step was implicit: operators could create a plan artifact and later call `apply`, but there was no read-only command that exercised the exact apply confirmation path without also requiring a DB target and risking lock/audit side effects.

A dedicated `apply-preflight` command gives scripts and humans a cheap final check that can run after ledger export and plan review, and before backup validation plus the real mutating apply. It keeps daemon ops surfaces read-only and avoids choosing a production DB engine.

A conservative runbook shape is now:

```bash
metin2-migrate ledger-snapshot --driver <driver> --dsn <dsn> > ledger-snapshot.json
metin2-migrate plan-artifact --ledger-snapshot ledger-snapshot.json --target-version latest > migration-plan-artifact.json
metin2-migrate apply-preflight --ledger-snapshot ledger-snapshot.json --target-version latest --plan-artifact migration-plan-artifact.json
# validate/back up file stores and DB using deployment-specific tooling here
metin2-migrate apply --driver <driver> --dsn <dsn> --ledger-snapshot ledger-snapshot.json --target-version latest --plan-artifact migration-plan-artifact.json --lock-file migration-apply.lock --audit-file migration-apply-audit.json
```

Rollback drills still require the explicit direction acknowledgement:

```bash
metin2-migrate plan-artifact --ledger-snapshot ledger-snapshot.json --target-version 0 > rollback-plan-artifact.json
metin2-migrate apply-preflight --ledger-snapshot ledger-snapshot.json --target-version 0 --plan-artifact rollback-plan-artifact.json --allow-rollback
metin2-migrate apply --driver <driver> --dsn <dsn> --ledger-snapshot ledger-snapshot.json --target-version 0 --plan-artifact rollback-plan-artifact.json --allow-rollback --lock-file migration-rollback.lock --audit-file migration-rollback-audit.json
```

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `apply-preflight` validates a reviewed plan artifact and writes metadata-only JSON without opening a database;
- successful preflight output carries both the reviewed plan checksum and the exact ledger-snapshot checksum for runbook/audit correlation;
- rollback preflight without `--allow-rollback` fails before any DB event;
- confirmed rollback preflight with `--allow-rollback` and a matching plan artifact succeeds without opening a database.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyPreflight' -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Write the production migration runbook that sequences ledger export, plan artifact review, `apply-preflight`, store backup validation, DB backup validation, `apply --plan-artifact --lock-file --audit-file`, and audit retention.
2. Add stale lock inspection/recovery only after deployment topology and operator policy are known.
3. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
