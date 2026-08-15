# CLI Migration Plan Artifact — 2026-08-15

## Objective

Add a read-only migration CLI command that emits both the dry-run plan and the exact checksum expected by `metin2-migrate apply --plan-sha256`.

The project already had strict ledger snapshots, `metin2-migrate plan`, CLI-only `metin2-migrate apply`, optional apply audit files, and a plan-checksum confirmation guard. This slice removes the remaining runbook footgun where operators had to separately compute the checksum over the exact indented plan JSON bytes before applying.

## Contract frozen by this slice

`metin2-migrate plan-artifact` accepts the same inputs as `plan`:

```bash
metin2-migrate plan-artifact \
  --ledger-snapshot <path|-> \
  --target-version <version|latest>
```

The command:

- reads the strict `go-metin2-schema-migrations-ledger-v1` snapshot from a path or stdin;
- caps ledger snapshot input at 64 KiB before planning;
- accepts explicit numeric targets, rollback target `0`, and `latest` for the embedded catalog tip;
- computes the same dry-run `db/migrations.Plan` that `metin2-migrate plan` prints;
- computes `plan_sha256` over the exact indented JSON bytes emitted by `metin2-migrate plan`, including the trailing newline;
- writes one metadata-only JSON artifact to stdout with format marker `go-metin2-migration-plan-artifact-v1`;
- never opens a database, applies migrations, rolls migrations back, or emits executable SQL or DSNs.

The output shape is:

```json
{
  "format": "go-metin2-migration-plan-artifact-v1",
  "plan_sha256": "<64-hex-sha256-of-plan-json>",
  "plan": {
    "current_version": 9,
    "latest_version": 9,
    "up_to_date": true,
    "pending": []
  }
}
```

The intended runbook shape is now:

```bash
metin2-migrate ledger-snapshot --driver <driver> --dsn <dsn> > ledger.json
metin2-migrate plan-artifact --ledger-snapshot ledger.json --target-version latest > plan-artifact.json
PLAN_SHA256=$(jq -r .plan_sha256 plan-artifact.json)
metin2-migrate apply \
  --driver <driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger.json \
  --target-version latest \
  --plan-sha256 "$PLAN_SHA256" \
  --audit-file migration-apply-audit.json
```

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- backup/restore orchestration around migration apply;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

`plan-artifact` is a convenience and safety command for explicit operator runbooks. It is still read-only and is not a lock, backup, or concurrency-control mechanism.

## Why this order

The checksum guard added to `apply` is useful only when operators can reproduce the exact checked byte stream reliably. Asking humans or scripts to remember that the checksum must cover the indented `Plan` JSON with a trailing newline creates an avoidable production-ops error mode.

A read-only artifact command keeps mutation in the existing CLI-only `apply` path while making the preflight-to-apply handoff deterministic, scriptable, and testable.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `plan-artifact` writes the expected format marker, embedded plan, and checksum over the exact `plan` JSON bytes;
- the emitted `plan_sha256` can feed `apply --plan-sha256` successfully;
- oversized ledger snapshots fail closed before planning or artifact output;
- the artifact remains metadata-only and does not expose SQL or DSN text.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunPlanArtifact' -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Add a production migration runbook that sequences ledger export, plan artifact, backup validation, `apply --plan-sha256`, and audit-file retention.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
