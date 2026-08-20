# CLI Ledger Snapshot Status — 2026-08-20

## Objective

Add a read-only CLI helper for validating and inspecting a retained offline `schema_migrations` ledger snapshot before it is trusted by `plan`, `plan-artifact`, `apply-preflight`, or `apply`, without opening a database target, reserving local coordination files, or exposing executable SQL / DSNs.

The existing `metin2-migrate ledger-snapshot` and `empty-ledger-snapshot` commands write strict metadata-only `go-metin2-schema-migrations-ledger-v1` JSON for runbook handoff. Operators could already feed those artifacts into later planning commands, but there was no small status command to re-check a retained snapshot file by itself during release packaging, handoff review, or incident triage. This closes the remaining status-helper gap beside `plan-artifact-status`, `apply-preflight-status`, `apply-lock-status`, and `apply-audit-status`.

## Contract frozen by this slice

`metin2-migrate ledger-snapshot-status --ledger-snapshot <path>`:

- requires `--ledger-snapshot`; extra positional arguments are usage errors;
- performs no database open, SQL execution, migration apply, rollback, lock reservation, audit-file reservation, snapshot deletion, or daemon mutation;
- returns success with `present: false` when the snapshot path is absent;
- returns success with `present: true` plus checksum and catalog-relative status when the file is a valid `go-metin2-schema-migrations-ledger-v1` snapshot that still matches the embedded catalog;
- rejects symlink or non-regular snapshot paths, oversized snapshots over 64 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported format markers, malformed entry names/checksums, duplicate versions, or catalog name/checksum drift;
- never emits DSNs, executable SQL, runtime store rows, apply output, lock output, or audit output.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-schema-migrations-ledger-snapshot-status-v1",
  "present": true,
  "ledger_snapshot_sha256": "...",
  "current_version": 1,
  "latest_version": 11,
  "up_to_date": false,
  "snapshot": {
    "format": "go-metin2-schema-migrations-ledger-v1",
    "entries": [
      {
        "version": 1,
        "name": "bootstrap_schema_migrations",
        "up_sha256": "..."
      }
    ]
  }
}
```

`ledger_snapshot_sha256` is computed over the exact retained snapshot bytes so operators can correlate the inspected file with later `apply-preflight` / apply-audit ledger checksum fields.

## What this is not yet

This is not migration apply authorization or artifact retention policy. It deliberately does not add:

- stale-snapshot cleanup;
- database backup validation;
- DB process liveness checks;
- database advisory locks;
- daemon-local migration mutation endpoints;
- daemon startup auto-migration;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories;
- ground-item restart durability.

A present valid ledger snapshot only proves the retained JSON is internally consistent with the current embedded catalog. Operators must still compare it with plan artifacts, apply preflight, lock/audit evidence, deployment notes, and DB backup evidence before a mutation window.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- a missing snapshot produces `present: false` without opening a database;
- a valid snapshot produces `present: true` with checksum, catalog-relative current/latest status, and without DSN or SQL text;
- malformed/unknown-field snapshot JSON, catalog checksum drift, and symlink snapshot paths fail closed without opening a database;
- top-level CLI usage lists `ledger-snapshot-status`.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunLedgerSnapshotStatus|TestRunRejectsUnknownCommandMentionsLedgerSnapshotStatus' -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add deployment-specific artifact naming / retention policy once deployment topology is known.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
4. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
