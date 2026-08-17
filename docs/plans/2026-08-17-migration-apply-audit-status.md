# Migration Apply Audit Status — 2026-08-17

## Objective

Add a read-only CLI helper for validating and inspecting a retained migration apply audit artifact without opening a database target, deleting the audit, applying migrations, or exposing DSNs / executable SQL.

The previous audit-file slices made `metin2-migrate apply --audit-file <path>` write strict metadata-only `go-metin2-migration-apply-audit-v1` JSON after successful non-empty apply plans. Operators could retain that artifact, but there was no safe command to re-check the file shape and correlation fields during incident review or release evidence collection. This slice keeps mutation CLI-only and adds an inspection-only audit-status boundary.

## Contract frozen by this slice

`metin2-migrate apply-audit-status --audit-file <path>`:

- requires `--audit-file`; extra positional arguments are usage errors;
- performs no database open, SQL execution, migration apply, rollback, audit deletion, lock reservation, or daemon mutation;
- returns success with `present: false` when the audit path is absent;
- returns success with `present: true` and the decoded `audit` object when the file is a valid `go-metin2-migration-apply-audit-v1` artifact;
- rejects symlink or non-regular audit paths, oversized audit files over 128 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported format markers, invalid timestamps, missing driver metadata, invalid target/result metadata, malformed checksum fields, plan checksum drift, or non-contiguous applied-step sequences;
- never emits DSNs, executable SQL, runtime store rows, or apply/rollback output beyond the existing metadata-only `ApplyResult`.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-migration-apply-audit-status-v1",
  "present": true,
  "audit": {
    "format": "go-metin2-migration-apply-audit-v1",
    "applied_at": "2026-08-17T00:00:00Z",
    "driver": "example-driver",
    "dsn_configured": true,
    "target_version": 11,
    "target_latest": true,
    "plan_sha256": "...",
    "confirmed_plan_sha256": "...",
    "ledger_snapshot_sha256": "...",
    "result": {
      "previous_version": 10,
      "current_version": 11,
      "latest_version": 11,
      "applied": [
        {
          "version": 11,
          "name": "example_migration",
          "direction": "up",
          "path": "0011_example_migration.up.sql",
          "sha256": "..."
        }
      ]
    }
  }
}
```

## What this is not yet

This is not audit retention policy or database recovery automation. It deliberately does not add:

- stale-lock or stale-audit deletion;
- database backup validation;
- process liveness checks;
- database advisory locks;
- daemon-local migration mutation endpoints;
- daemon startup auto-migration;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

A present valid audit only proves the retained JSON is internally consistent with the metadata-only apply result and plan checksum. Operators must still compare it with retained ledger snapshots, plan artifacts, preflight output, deployment notes, and DB backup evidence.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- a missing audit produces `present: false` without opening a database;
- a valid audit produces `present: true` with the exact correlation fields and without DSN or SQL text;
- malformed/unknown-field audit JSON and symlink audit paths fail closed without opening a database.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunApplyAuditStatus -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add deployment-specific audit retention and artifact naming policy once deployment topology is known.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Add import/quarantine tooling for schema-shaped exports before extracting DB-backed repositories.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
