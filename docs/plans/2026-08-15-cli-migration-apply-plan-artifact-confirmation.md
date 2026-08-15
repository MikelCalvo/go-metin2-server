# CLI Migration Apply Plan Artifact Confirmation — 2026-08-15

## Objective

Let `metin2-migrate apply` consume the exact metadata-only artifact produced by `metin2-migrate plan-artifact`, so an operator can confirm an apply run against a previously reviewed dry-run plan without manually copying a checksum into `--plan-sha256`.

The repo already had strict ledger snapshots, metadata-only dry-run plans, a plan-artifact command, CLI-only apply, optional plan checksum confirmation, and metadata-only audit files. This slice closes the runbook handoff by allowing the reviewed artifact itself to be the apply confirmation input.

## Contract frozen by this slice

`metin2-migrate apply` now accepts either a raw plan checksum or a plan artifact path:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  --plan-artifact migration-plan-artifact.json
```

The new `--plan-artifact <path>` option:

- reads the strict `go-metin2-migration-plan-artifact-v1` JSON emitted by `metin2-migrate plan-artifact` with a 128 KiB input cap;
- rejects oversized artifacts, malformed JSON, unknown fields, trailing JSON, invalid UTF-8, unsupported formats, malformed checksums, and artifacts whose embedded checksum does not match their embedded `Plan` JSON;
- recomputes the current metadata-only plan from the supplied ledger snapshot and target version;
- requires both the embedded `Plan` and embedded `plan_sha256` to match that recomputed plan;
- fails closed before opening the configured database when the artifact does not match;
- is mutually exclusive with `--plan-sha256` to avoid ambiguous confirmation sources;
- when combined with `--audit-file`, records the artifact's confirmed checksum in `confirmed_plan_sha256`.

The intended safer runbook is now:

```bash
metin2-migrate ledger-snapshot --driver <driver> --dsn <dsn> > ledger.json
metin2-migrate plan-artifact --ledger-snapshot ledger.json --target-version latest > plan-artifact.json
# Review/retain plan-artifact.json here.
metin2-migrate apply \
  --driver <driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger.json \
  --target-version latest \
  --plan-artifact plan-artifact.json \
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

`--plan-artifact` is a confirmation guard and runbook convenience. It is not a lock, not a backup, not a concurrency-control primitive, and not a substitute for a tested pre-apply backup/restore workflow.

## Why this order

`--plan-sha256` already prevented applying a plan that differed from a reviewed dry-run, but still required an operator or script to extract and pass only a checksum. The artifact path is safer for runbooks because it lets the CLI validate the whole reviewed artifact shape, not just an unstructured checksum string, before any database open occurs.

Keeping this in `cmd/metin2-migrate` preserves the existing rule that daemon-local migration endpoints remain read-only.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `apply --plan-artifact <path>` accepts the current plan artifact and proceeds to mutation;
- audited apply records the artifact checksum in the metadata-only audit file;
- mismatched artifacts fail closed before opening the database;
- internally inconsistent and oversized plan artifacts fail closed before opening the database;
- `--plan-sha256` and `--plan-artifact` are mutually exclusive usage errors.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(AcceptsPlanArtifactBeforeMutation|WritesPlanArtifactSHA256IntoAuditFile|RejectsMismatchedPlanArtifactBeforeOpeningDatabase|RejectsInvalidPlanArtifactBeforeOpeningDatabase|RejectsPlanArtifactAndPlanSHA256TogetherAsUsageError|RejectsOversizedPlanArtifactBeforeOpeningDatabase)' -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Add a production migration runbook that sequences ledger export, plan artifact review, file-store backup validation, `apply --plan-artifact --audit-file`, and audit retention.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
