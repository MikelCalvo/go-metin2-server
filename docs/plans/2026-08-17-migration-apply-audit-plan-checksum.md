# Migration Apply Audit Plan Checksum — 2026-08-17

## Objective

Tighten the CLI-only migration apply audit artifact so each successful audited mutation names both the exact offline ledger snapshot and the exact dry-run plan it applied.

The previous runbook/preflight path emitted `plan_sha256` from `apply-preflight`, and `metin2-migrate apply --audit-file` recorded the ledger snapshot checksum plus any optional `confirmed_plan_sha256`. An unconfirmed forward apply still lacked the computed plan checksum in the durable audit file, which made post-run correlation weaker when operators retained only the apply audit and not the preflight output.

## Contract frozen by this slice

`metin2-migrate apply --audit-file <path>` continues to be the only mutating migration CLI surface and still requires:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  [--plan-sha256 <hex> | --plan-artifact <path>] \
  [--lock-file <path>] \
  [--audit-file <path>] \
  [--allow-rollback]
```

When an audit file is written, the strict metadata-only `go-metin2-migration-apply-audit-v1` JSON now includes:

- `ledger_snapshot_sha256` — checksum over the exact ledger-snapshot bytes supplied to `apply`;
- `plan_sha256` — checksum over the exact dry-run plan recomputed from that ledger snapshot and resolved target;
- optional `confirmed_plan_sha256` when the operator supplied `--plan-sha256` or a matching `--plan-artifact`;
- the existing resolved target, driver name, DSN-present boolean, timestamp, and metadata-only `ApplyResult`.

This lets an operator compare:

1. the reviewed `migration-plan-artifact.json` checksum;
2. the immediate `apply-preflight.json` `plan_sha256` / `ledger_snapshot_sha256` pair;
3. the post-run `migration-apply-audit.json` `plan_sha256` / `ledger_snapshot_sha256` pair.

All three artifacts remain free of executable SQL, DSNs, runtime store rows, and daemon mutation endpoints.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply` or `/local/db/migrations/rollback`;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- stale-lock recovery policy;
- database advisory locks;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves an audited apply without explicit plan confirmation still records the computed `plan_sha256`, leaves `confirmed_plan_sha256` empty, keeps output metadata-only, and does not leak DSNs or executable SQL.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunApplyWritesComputedPlanSHA256IntoAuditFile -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add stale-lock inspection/recovery only after deployment topology and operator policy are known.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add import/quarantine tooling for schema-shaped exports before extracting DB-backed repositories.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
