# Migration Apply Preflight Status — 2026-08-17

## Objective

Add a read-only CLI helper for validating and inspecting a retained migration apply preflight artifact without opening a database target, deleting the artifact, applying migrations, reserving a lock, or exposing DSNs / executable SQL.

The previous runbook path made `metin2-migrate apply-preflight` produce strict metadata-only `go-metin2-migration-apply-preflight-v1` JSON immediately before a migration window. Operators could retain that artifact, but there was no small status command to re-check its envelope, checksums, target metadata, and plan continuity during release evidence review or incident triage.

## Contract frozen by this slice

`metin2-migrate apply-preflight-status --apply-preflight <path>`:

- requires `--apply-preflight`; extra positional arguments are usage errors;
- performs no database open, SQL execution, migration apply, rollback, lock reservation, audit reservation, file deletion, or daemon mutation;
- returns success with `present: false` when the preflight path is absent;
- returns success with `present: true` and the decoded `preflight` object when the file is a valid `go-metin2-migration-apply-preflight-v1` artifact;
- rejects symlink or non-regular preflight paths, oversized preflight files over 128 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported format markers, invalid target metadata, malformed checksum fields, plan checksum drift, target/plan endpoint drift, invalid plan version metadata, malformed pending-step checksums, invalid step directions, or non-contiguous pending-step sequences;
- never emits DSNs, executable SQL, runtime store rows, migration apply output, lock output, or audit output.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-migration-apply-preflight-status-v1",
  "present": true,
  "preflight": {
    "format": "go-metin2-migration-apply-preflight-v1",
    "target_version": 11,
    "target_latest": true,
    "ledger_snapshot_sha256": "...",
    "plan_sha256": "...",
    "plan": {
      "current_version": 10,
      "latest_version": 11,
      "up_to_date": false,
      "pending": [
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

This is not database recovery automation or an authorization decision for mutation. It deliberately does not add:

- stale preflight cleanup;
- database backup validation;
- process liveness checks;
- database advisory locks;
- daemon-local migration mutation endpoints;
- daemon startup auto-migration;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

A present valid preflight only proves the retained JSON is internally consistent and metadata-only. Operators must still compare it with the retained ledger snapshot, plan artifact, lock/audit artifacts, deployment notes, and database backup evidence before treating a migration window as complete.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- a missing preflight produces `present: false` without opening a database;
- a valid preflight produces `present: true` with the exact target, ledger checksum, plan checksum, and metadata-only plan shape;
- malformed/unknown-field preflight JSON, plan checksum drift, target drift, and symlink preflight paths fail closed without opening a database.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunApplyPreflightStatus -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add deployment-specific preflight/audit/lock artifact naming and retention policy once deployment topology is known.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Add import/quarantine tooling for schema-shaped exports before extracting DB-backed repositories.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
