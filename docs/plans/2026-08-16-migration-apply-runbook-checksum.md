# Migration Apply Runbook Checksum — 2026-08-16

## Objective

Tighten the CLI-only migration apply preflight and document the production-safe apply/rollback runbook without widening daemon-local mutation surfaces or selecting a production DB engine.

The previous `apply-preflight` output proved that the reviewed plan still matched the supplied ledger snapshot and target, but it did not echo a checksum of the exact ledger-snapshot bytes used for that preflight. Operators could correlate the plan checksum with the plan artifact, but the final no-side-effect preflight artifact did not independently name the ledger snapshot it checked.

## Contract frozen by this slice

`metin2-migrate apply-preflight` still accepts:

```bash
metin2-migrate apply-preflight \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  [--plan-sha256 <hex> | --plan-artifact <path>] \
  [--allow-rollback]
```

On success it writes metadata-only JSON with format marker `go-metin2-migration-apply-preflight-v1` and now includes:

- `target_version`;
- `target_latest`;
- `ledger_snapshot_sha256` — checksum over the exact ledger-snapshot bytes read by this preflight;
- `plan_sha256` — checksum over the exact dry-run plan JSON;
- `plan` — metadata-only pending migration steps.

The command still:

- never accepts a driver or DSN;
- never opens a database;
- never reserves a lock file;
- never creates an audit file;
- never emits executable SQL;
- rejects rollback/down plans unless `--allow-rollback` plus plan confirmation is present.

## Runbook added

`docs/workflow/migration-apply-runbook.md` now captures the conservative operator sequence:

1. export/read the migration catalog;
2. export the strict ledger snapshot;
3. create and review a plan artifact;
4. run `apply-preflight` immediately before backup validation and mutation;
5. verify deployment-specific DB/file-store backups;
6. run `apply` with `--plan-artifact`, `--lock-file`, and `--audit-file`;
7. retain metadata-only audit/preflight artifacts.

The same doc freezes the rollback drill sequence with explicit `--allow-rollback` and a reviewed rollback plan artifact.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply` or `/local/db/migrations/rollback`;
- daemon startup auto-migration;
- a bundled DB driver or production DB engine selection;
- DB advisory locks;
- stale local lock removal policy;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves successful `apply-preflight` output carries `ledger_snapshot_sha256`, remains metadata-only, and still does not open a database target.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunApplyPreflightReportsLedgerSnapshotSHA256ForRunbookAudit -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add stale-lock inspection/recovery only after deployment topology and operator policy are known.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add import/quarantine tooling for schema-shaped exports before extracting DB-backed repositories.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
