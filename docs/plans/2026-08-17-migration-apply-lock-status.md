# Migration Apply Lock Status — 2026-08-17

## Objective

Add a read-only CLI helper for inspecting an existing local migration apply lock file without deleting it, opening a database target, or exposing DSNs / executable SQL.

The prior lock-metadata slice made `metin2-migrate apply --lock-file <path>` reserve strict metadata-only JSON. Operators still had to inspect that JSON manually when a later run failed because the lock already existed. This slice freezes a small inspection command that can be used safely in runbooks before any manual stale-lock decision.

## Contract frozen by this slice

`metin2-migrate apply-lock-status --lock-file <path>`:

- requires `--lock-file`; extra positional arguments are usage errors;
- performs no database open, SQL execution, migration apply, rollback, audit-file reservation, or lock deletion;
- returns success with `present: false` when the lock path is absent;
- returns success with `present: true` and the decoded `lock` object when the file is a valid `go-metin2-migration-apply-lock-v1` artifact;
- rejects symlink or non-regular lock paths, oversized lock files over 16 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported format markers, invalid timestamps, non-positive PIDs, missing driver metadata, invalid target metadata, or malformed checksum fields;
- never emits DSNs, executable SQL, runtime store rows, or apply output.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-migration-apply-lock-status-v1",
  "present": true,
  "lock": {
    "format": "go-metin2-migration-apply-lock-v1",
    "created_at": "2026-08-17T00:00:00Z",
    "pid": 1234,
    "driver": "example-driver",
    "dsn_configured": true,
    "target_version": 11,
    "target_latest": true,
    "plan_sha256": "...",
    "confirmed_plan_sha256": "...",
    "ledger_snapshot_sha256": "..."
  }
}
```

## What this is not yet

This is not stale-lock recovery policy. It deliberately does not add:

- automatic lock expiry or deletion;
- process liveness checks;
- hostname / deployment identity checks;
- database advisory locks;
- daemon-local migration mutation endpoints;
- daemon startup auto-migration;
- backup/restore orchestration inside the CLI.

A present lock still means an operator must inspect deployment notes, retained preflight/audit artifacts, and local process ownership before deciding whether removal is safe.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- a missing lock produces `present: false` without opening a database;
- a valid metadata-only lock produces `present: true` with the exact correlation fields and without DSN or SQL text;
- malformed/unknown-field lock JSON and symlink lock paths fail closed without opening a database.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus|TestRunApply(WritesMetadataOnlyLockFileBeforeOpeningDatabase|RejectsExistingLockFile|RemovesLockFile|RemovesReservedLockFile)' -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. ~~Add deployment-specific stale-lock policy only after the project defines host/process ownership and retention expectations.~~ Partial: holder PID liveness reporting landed in [migration apply lock holder liveness](2026-08-20-migration-apply-lock-holder-liveness.md); automatic removal remains deferred.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
