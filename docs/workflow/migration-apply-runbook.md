# Migration Apply Runbook

This runbook freezes the current production-safe ordering for the project-owned SQL migration tooling. It is an operator workflow for `cmd/metin2-migrate`, not daemon startup policy and not a DB-backed runtime claim.

## Scope and guardrails

Use this workflow when you need to apply the embedded `db/migrations` catalog to an operator-managed `database/sql` target.

The current boundary is deliberately narrow:

- mutating migration execution is CLI-only through `metin2-migrate apply`;
- shipped daemon ops endpoints remain read-only (`catalog`, `status`, `ledger-snapshot`, and offline plan helpers);
- the CLI does not bundle or select a production DB engine/driver;
- account, character, item, quest, content, login-ticket, and world runtime stores are still bootstrap/file-backed unless a later repository slice says otherwise;
- executable SQL and DSNs must not be copied into logs, tickets, or audit summaries.

## Required artifacts

Keep these files together for each migration run:

- `ledger-snapshot.json` — strict `go-metin2-schema-migrations-ledger-v1` input exported from the target before mutation;
- `migration-plan-artifact.json` — strict `go-metin2-migration-plan-artifact-v1` output reviewed before mutation;
- `apply-preflight.json` — strict `go-metin2-migration-apply-preflight-v1` output generated immediately before backup validation and mutation;
- `migration-apply.lock` — an operator-chosen local lock path that should not already exist; while reserved, it contains metadata-only `go-metin2-migration-apply-lock-v1` JSON with the local PID, target version, plan checksum, and ledger-snapshot checksum, but never the DSN or executable SQL;
- `apply-lock-status.json` — optional `go-metin2-migration-apply-lock-status-v1` output from inspecting an existing lock before any manual stale-lock decision;
- `migration-apply-audit.json` — exclusive metadata-only audit output written after a successful non-empty apply;
- deployment-specific DB backup evidence, kept outside this repo.

`apply-preflight.json` reports both:

- `ledger_snapshot_sha256` — checksum over the exact offline ledger snapshot bytes;
- `plan_sha256` — checksum over the exact reviewed dry-run plan bytes.

Those two checksums let an operator correlate the preflight with the plan artifact and the later apply audit without storing executable SQL or DSNs in the audit trail. `migration-apply-audit.json` records `plan_sha256` for the exact plan applied, `ledger_snapshot_sha256` for the exact ledger snapshot supplied to `apply`, and `confirmed_plan_sha256` when the run used `--plan-sha256` or `--plan-artifact`.

## Forward apply workflow

From a clean checkout of the release you intend to run:

```bash
metin2-migrate catalog > migration-catalog.json
metin2-migrate ledger-snapshot \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  > ledger-snapshot.json
metin2-migrate plan-artifact \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  > migration-plan-artifact.json
metin2-migrate apply-preflight \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  --plan-artifact migration-plan-artifact.json \
  > apply-preflight.json
```

Before applying, verify and retain deployment-specific backups outside the migration CLI. At minimum, validate that you can restore the target database from your backup system. For this bootstrap server, also validate any file-backed stores that matter to the deployment before running DB migrations:

- account snapshots (`/local/account-store/validate`, `/local/account-store/backup/validate` when using a store backup);
- login-ticket snapshots (`/local/login-tickets/validate`);
- item-template snapshots (`/local/item-templates/validate`, `/local/item-templates/backup/validate` when using a store backup);
- static actors, interactions, and quest state when authored content is part of the deployment.

Then run the mutating apply with an exclusive local lock and audit file:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  --plan-artifact migration-plan-artifact.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

After success:

1. confirm the lock file was removed;
2. retain `migration-apply-audit.json` with the release artifacts;
3. run `metin2-migrate status --driver <driver> --dsn <dsn> --target-version latest` and confirm `up_to_date: true`;
4. if a daemon is running against that target, confirm `GET /local/db/migrations/status` reports the same metadata-only boundary.

## Rollback drill workflow

Rollback/down plans require an explicit direction acknowledgement and reviewed-plan confirmation. Preview first:

```bash
metin2-migrate ledger-snapshot \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  > ledger-snapshot.json
metin2-migrate plan-artifact \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  > rollback-plan-artifact.json
metin2-migrate apply-preflight \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  --plan-artifact rollback-plan-artifact.json \
  --allow-rollback \
  > rollback-apply-preflight.json
```

Only after backup restoreability is proven, execute the rollback:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  --plan-artifact rollback-plan-artifact.json \
  --allow-rollback \
  --lock-file migration-rollback.lock \
  --audit-file migration-rollback-audit.json
```

Rollback to zero is allowed by the current primitive, but it drops the `schema_migrations` ledger. Treat it as a destructive drill unless the deployment has an explicit database restore plan.

## Failure handling

- If `ledger-snapshot`, `status`, or daemon-local migration status fails, stop before planning and inspect the configured driver/DSN and `schema_migrations` metadata.
- If `plan-artifact` or `apply-preflight` fails, do not run `apply`; regenerate or re-review the ledger snapshot and plan.
- If `--lock-file` already exists, assume another operator or interrupted run owns the migration window. Do not delete it blindly; first run `metin2-migrate apply-lock-status --lock-file <path> > apply-lock-status.json`, then inspect process ownership, the metadata-only lock JSON (`pid`, `target_version`, `plan_sha256`, `ledger_snapshot_sha256`), retained preflight/audit artifacts, and deployment notes. The status helper is read-only: it validates the non-symlink regular lock file shape, returns `present: false` for an absent path, and never removes the lock, opens the DB target, or exposes the DSN / executable SQL.
- If `apply` fails after reserving the lock or audit path, the CLI attempts to remove the reserved lock/audit files and roll back the SQL transaction. Keep stderr, the original ledger snapshot, and the failed plan artifact for diagnosis.
- If the database reports an unknown or drifted ledger row, do not edit `schema_migrations` by hand. Compare `migration-catalog.json`, the deployed binary version, and the target database backup.

## Anti-goals

Do not use this runbook to justify:

- daemon-local `/local/db/migrations/apply` or rollback endpoints;
- daemon startup auto-migration;
- committing DSNs or secrets to git;
- DB-backed runtime claims for account, character, item, quest, content, login-ticket, or world state;
- stale-lock auto-removal without a deployment-specific policy.
- treating `apply-lock-status` as authorization to delete an existing lock; it is an inspection helper only.
