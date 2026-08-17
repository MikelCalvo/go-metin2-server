# Migration Plan Artifact Status — 2026-08-17

## Objective

Add a read-only CLI helper for validating and inspecting a retained migration plan artifact before it is trusted by `apply-preflight` or `apply`, without opening a database target, reserving local coordination files, or exposing executable SQL / DSNs.

The existing `metin2-migrate plan-artifact` command writes strict metadata-only `go-metin2-migration-plan-artifact-v1` JSON for runbook review. Operators could already pass that artifact directly to `apply-preflight` or `apply`, but there was no small status command to re-check an artifact file by itself during release packaging, handoff review, or incident triage.

## Contract frozen by this slice

`metin2-migrate plan-artifact-status --plan-artifact <path>`:

- requires `--plan-artifact`; extra positional arguments are usage errors;
- performs no database open, SQL execution, migration apply, rollback, lock reservation, audit-file reservation, artifact deletion, or daemon mutation;
- returns success with `present: false` when the artifact path is absent;
- returns success with `present: true` and the decoded `artifact` object when the file is a valid `go-metin2-migration-plan-artifact-v1` artifact;
- rejects symlink or non-regular artifact paths, oversized artifacts over 128 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported format markers, malformed plan checksums, plan checksum drift, invalid plan version metadata, malformed pending-step checksums, invalid step directions, or non-contiguous pending-step sequences;
- never emits DSNs, executable SQL, runtime store rows, apply output, lock output, or audit output.

Successful output uses this metadata-only envelope:

```json
{
  "format": "go-metin2-migration-plan-artifact-status-v1",
  "present": true,
  "artifact": {
    "format": "go-metin2-migration-plan-artifact-v1",
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

This is not migration apply authorization or artifact retention policy. It deliberately does not add:

- stale-artifact cleanup;
- database backup validation;
- DB process liveness checks;
- database advisory locks;
- daemon-local migration mutation endpoints;
- daemon startup auto-migration;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

A present valid plan artifact only proves the retained JSON is internally consistent and metadata-only. Operators must still compare it with the exact ledger snapshot, apply preflight, lock, audit, deployment notes, and DB backup evidence before a mutation window.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- a missing artifact produces `present: false` without opening a database;
- a valid artifact produces `present: true` with the exact plan checksum and metadata-only plan shape;
- malformed/unknown-field artifact JSON and symlink artifact paths fail closed without opening a database.

Validation for this slice:

- `go test ./internal/migratecli -run TestRunPlanArtifactStatus -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add deployment-specific artifact naming / retention policy once deployment topology is known.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
