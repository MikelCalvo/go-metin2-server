# Migration Apply Runbook

This runbook freezes the current production-safe ordering for the project-owned SQL migration tooling. It is an operator workflow for `cmd/metin2-migrate`, not daemon startup policy and not a DB-backed runtime claim.

## Scope and guardrails

Use this workflow when you need to apply the embedded `db/migrations` catalog to an operator-managed `database/sql` target.

The current boundary is deliberately narrow:

- mutating migration execution is CLI-only through `metin2-migrate apply`;
- shipped daemon ops endpoints remain read-only (`catalog`, `status`, `ledger-snapshot`, and offline plan helpers);
- stock CLI/daemon binaries do not register or select a production DB engine/driver (a build-tagged SQLite harness proves live ledger apply/read/snapshot under `go test -tags=sqlite_harness ./db/migrations -run SQLiteHarness`, programmatic `0002` roster SQL import under `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessRosterImport`, programmatic `0003` item-state SQL import (including additive `0024` presence-aware instance sockets) under `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessItemStateImport`, programmatic `0004` quest-state SQL import under `go test -tags=sqlite_harness ./internal/queststate -run SQLiteHarnessQuestStateImport`, programmatic `0011` point-state SQL import under `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessPointStateImport`, programmatic `0023` myshop unit-prices SQL import under `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessMyShopUnitPricesImport`, programmatic `0015` safebox-state SQL import (including additive `0025` presence-aware safebox cell instance sockets) under `go test -tags=sqlite_harness ./internal/safeboxstore -run SQLiteHarnessSafeboxStateImport`, programmatic `0010` ground-item-state SQL import (including additive `0026` presence-aware instance sockets) under `go test -tags=sqlite_harness ./internal/worldruntime -run SQLiteHarnessGroundItemStateImport`, and programmatic `0009` item-template-state SQL import under `go test -tags=sqlite_harness ./internal/itemstore -run SQLiteHarnessItemTemplateStateImport`, programmatic `0007` auth-login-ticket-handoff SQL import under `go test -tags=sqlite_harness ./internal/loginticket -run SQLiteHarnessAuthLoginTicketHandoffImport`, and programmatic tip-`0013` static-actor content-state SQL import under `go test -tags=sqlite_harness ./internal/staticstore -run SQLiteHarnessStaticActorContentStateImport`; see [SQLite schema_migrations driver-backed harness](../plans/2026-08-25-sqlite-schema-migrations-driver-harness.md), [account/character roster SQL import/backfill](../plans/2026-08-25-account-character-roster-sql-import-backfill.md), [character item-state SQL import/backfill](../plans/2026-08-26-character-item-state-sql-import-backfill.md), [seeded item instance-sockets tip sync](../plans/2026-08-30-seeded-item-instance-sockets-import-export-drill.md), [character quest-state SQL import/backfill](../plans/2026-08-26-character-quest-state-sql-import-backfill.md), [character point-state SQL import/backfill](../plans/2026-08-26-character-point-state-sql-import-backfill.md), [character myshop unit-prices migration](../plans/2026-08-29-character-myshop-unit-prices-migration.md), [character safebox-state SQL import/backfill](../plans/2026-08-26-character-safebox-state-sql-import-backfill.md), [seeded safebox cell instance-sockets tip sync](../plans/2026-08-30-seeded-safebox-cell-instance-sockets-import-export-drill.md), [bootstrap ground-item-state SQL import/backfill](../plans/2026-08-26-bootstrap-ground-item-state-sql-import-backfill.md), [item-template-state SQL import/backfill](../plans/2026-08-26-item-template-state-sql-import-backfill.md), and [auth-login-ticket-handoff SQL import/backfill](../plans/2026-08-26-auth-login-ticket-handoff-sql-import-backfill.md), [static-actor content-state SQL import/backfill](../plans/2026-08-27-static-actor-content-state-sql-import-backfill.md));
- account, character, item, quest, content, login-ticket, and world runtime stores are still bootstrap/file-backed unless a later repository slice says otherwise;
- executable SQL and DSNs must not be copied into logs, tickets, or audit summaries.

## Required artifacts

Keep these files together for each migration run:

- `ledger-snapshot.json` — strict `go-metin2-schema-migrations-ledger-v1` input exported from the target before mutation;
- `ledger-snapshot-status.json` — optional `go-metin2-schema-migrations-ledger-snapshot-status-v1` output from re-validating a retained ledger snapshot before planning, preflight, or apply;
- `migration-plan-artifact.json` — strict `go-metin2-migration-plan-artifact-v1` output reviewed before mutation;
- `plan-artifact-status.json` — optional `go-metin2-migration-plan-artifact-status-v1` output from re-validating a retained plan artifact before handoff review, preflight, or apply;
- `apply-preflight.json` — strict `go-metin2-migration-apply-preflight-v1` output generated immediately before backup validation and mutation;
- `apply-preflight-status.json` — optional `go-metin2-migration-apply-preflight-status-v1` output from re-validating a retained preflight artifact before handoff review, release evidence collection, or incident triage;
- `migration-apply.lock` — an operator-chosen local lock path that should not already exist; while reserved, it contains metadata-only `go-metin2-migration-apply-lock-v1` JSON with the local PID, hostname, stamped build identity, target version, plan checksum, and ledger-snapshot checksum, but never the DSN or executable SQL;
- `apply-lock-status.json` — optional `go-metin2-migration-apply-lock-status-v1` output from inspecting an existing lock before any stale-lock decision;
- `apply-lock-aside.json` — optional `go-metin2-migration-apply-lock-aside-v1` output from confirmation-gated lab aside-rename when recovering a leftover lock;
- `migration-apply-audit.json` — exclusive metadata-only audit output written after a successful non-empty apply;
- `apply-audit-status.json` — optional `go-metin2-migration-apply-audit-status-v1` output from re-validating a retained apply audit during release evidence review or incident triage;
- deployment-specific DB backup evidence, kept outside this repo.

`apply-preflight.json` reports both:

- `ledger_snapshot_sha256` — checksum over the exact offline ledger snapshot bytes;
- `plan_sha256` — checksum over the exact reviewed dry-run plan bytes.

Those two checksums let an operator correlate the preflight with the plan artifact and the later apply audit without storing executable SQL or DSNs in the audit trail. `migration-apply-audit.json` records `plan_sha256` for the exact plan applied, `ledger_snapshot_sha256` for the exact ledger snapshot supplied to `apply`, and `confirmed_plan_sha256` when the run used `--plan-sha256`, `--plan-artifact`, or `--apply-preflight`.

## Forward apply workflow

From a clean checkout of the release you intend to run:

Optionally print a path-aware retention script that creates the lab
`/var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/` tree documented in
[lab deployment topology](lab-deployment-topology.md):

```bash
metin2-migrate version \
  | metin2-migrate migration-run-retention --build-info - \
  > migration-run-retention.sh
```

The printer never opens a database, never embeds a DSN, and never executes
apply itself. Hermetic `/bin/sh` proofs cover forward apply-to-tip, rollback-to-zero, and intermediate targets empty→`7` / tip→`8` under `go test -tags=sqlite_harness ./internal/migratecli -run MigrationRunRetentionSQLite` — see [hermetic migration-run-retention SQLite apply](../plans/2026-08-28-hermetic-migration-run-retention-sqlite-apply.md) and [intermediate-target twin](../plans/2026-08-28-hermetic-migration-run-retention-intermediate-target-sqlite.md). It retains both-daemon build-info, optional
`/var/log/metin2/{gamed,authd}.log` copies when present (`--gamed-log-path` /
`--authd-log-path`; missing files stay non-fatal), runtime-config, persistence
status before/after mutation, and a `notes.md` stub beside the migration
metadata artifacts. Export `DRIVER` / `DSN` before running any printed
DB-touching commands, then retain the redirected artifacts under `$RUN`.

```bash
metin2-migrate catalog > migration-catalog.json
metin2-migrate ledger-snapshot \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  > ledger-snapshot.json
metin2-migrate ledger-snapshot-status \
  --ledger-snapshot ledger-snapshot.json \
  > ledger-snapshot-status.json
metin2-migrate plan-artifact \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  > migration-plan-artifact.json
metin2-migrate plan-artifact-status \
  --plan-artifact migration-plan-artifact.json \
  > plan-artifact-status.json
metin2-migrate apply-preflight \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  --plan-artifact migration-plan-artifact.json \
  > apply-preflight.json
metin2-migrate apply-preflight-status \
  --apply-preflight apply-preflight.json \
  > apply-preflight-status.json
```

Before applying, verify and retain deployment-specific backups outside the migration CLI. At minimum, validate that you can restore the target database from your backup system. For this bootstrap server, also validate any file-backed stores that matter to the deployment before running DB migrations:

- account snapshots (`/local/account-store/validate`, `/local/account-store/backup/validate` when using a store backup);
- login-ticket snapshots (`/local/login-tickets/validate`, `/local/login-tickets/backup/validate` when using a store backup);
- item-template snapshots (`/local/item-templates/validate`, `/local/item-templates/backup/validate` when using a store backup);
- interaction definitions (`/local/interaction-store/validate`, `/local/interaction-store/backup/validate` when using a store backup);
- static actors (`/local/static-actor-store/validate`, `/local/static-actors/backup/validate` when using a store backup);
- quest state (`/local/quest-state/validate`, `/local/quest-state/backup/validate` when using a store backup);
- pending ground item/gold handles (`/local/ground-item-store/validate`, `/local/ground-item-store/backup/validate` when using a store backup);
- durable safebox cells + warehouse gold (`/local/safebox-store/validate`, `/local/safebox-store/backup/validate` when using a store backup; tip `0015_character_safebox_money`).

When the window needs a combined multi-store file backup or a drained-session restore drill across the eight manifested stores, follow the detailed [file-store backup/restore drill](file-store-backup-restore-drill.md) instead of improvising per-store ordering.

Then run the mutating apply with an exclusive local lock and audit file:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger-snapshot.json \
  --target-version latest \
  --apply-preflight apply-preflight.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

After success:

1. confirm the lock file was removed;
2. retain `migration-apply-audit.json` with the release artifacts;
3. optionally re-validate the retained audit with `metin2-migrate apply-audit-status --audit-file migration-apply-audit.json > apply-audit-status.json`;
4. run `metin2-migrate status --driver <driver> --dsn <dsn> --target-version latest` and confirm `up_to_date: true`;
5. if a daemon is running against that target, confirm `GET /local/db/migrations/status` reports the same metadata-only boundary.

## Rollback drill workflow

Rollback/down plans require an explicit direction acknowledgement and reviewed-plan confirmation. Optionally print a path-aware retention script for the same lab `/var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/` tree with rollback artifact names:

```bash
metin2-migrate version \
  | metin2-migrate migration-run-retention \
      --build-info - \
      --target-version <rollback-version> \
      --allow-rollback \
  > migration-rollback-run-retention.sh
```

`--allow-rollback` requires an explicit non-`latest` `--target-version`. When `--lock-file` is omitted the printed default becomes `migration-rollback.lock`. The printer still retains both-daemon build-info, optional `/var/log/metin2/{gamed,authd}.log` copies when present, runtime-config, persistence status before/after mutation, and a `notes.md` stub beside the rollback metadata artifacts. It never opens a database, never embeds a DSN, and never executes rollback itself.

Preview first:

```bash
metin2-migrate ledger-snapshot \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  > ledger-snapshot.json
metin2-migrate ledger-snapshot-status \
  --ledger-snapshot ledger-snapshot.json \
  > ledger-snapshot-status.json
metin2-migrate plan-artifact \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  > rollback-plan-artifact.json
metin2-migrate plan-artifact-status \
  --plan-artifact rollback-plan-artifact.json \
  > rollback-plan-artifact-status.json
metin2-migrate apply-preflight \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  --plan-artifact rollback-plan-artifact.json \
  --allow-rollback \
  > rollback-apply-preflight.json
metin2-migrate apply-preflight-status \
  --apply-preflight rollback-apply-preflight.json \
  > rollback-apply-preflight-status.json
```

Only after backup restoreability is proven, execute the rollback:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  --apply-preflight rollback-apply-preflight.json \
  --allow-rollback \
  --lock-file migration-rollback.lock \
  --audit-file migration-rollback-audit.json
```

Rollback to zero is allowed by the current primitive, but it drops the `schema_migrations` ledger. Treat it as a destructive drill unless the deployment has an explicit database restore plan.

## Failure handling

- If `ledger-snapshot`, `ledger-snapshot-status`, `status`, or daemon-local migration status fails, stop before planning and inspect the configured driver/DSN and `schema_migrations` metadata. A missing `schema_migrations` relation (first-time target or after rollback-to-zero) is treated as an empty applied ledger by `ledger-snapshot` / `status` so operators can plan from version zero without hand-editing the runbook script; other query failures stay fail-closed. `ledger-snapshot-status` is read-only: it validates the non-symlink regular offline ledger snapshot shape, returns `present: false` for an absent path, verifies catalog name/checksum consistency, reports the exact snapshot checksum plus catalog-relative current/latest status, and never opens the DB target, reserves lock/audit files, or exposes executable SQL / DSNs.
- If `plan-artifact`, `plan-artifact-status`, `apply-preflight`, or `apply-preflight-status` fails, do not run `apply`; regenerate or re-review the ledger snapshot, plan, and preflight artifact. `plan-artifact-status` is read-only: it validates the non-symlink regular plan artifact shape, returns `present: false` for an absent path, verifies the embedded plan checksum and contiguous pending-step sequence, and never opens the DB target, reserves lock/audit files, or exposes executable SQL / DSNs. `apply-preflight-status` is also read-only: it validates the non-symlink regular preflight artifact shape, returns `present: false` for an absent path, verifies the preflight plan checksum plus target/plan endpoint consistency, and never opens the DB target, reserves lock/audit files, deletes the preflight artifact, or exposes executable SQL / DSNs. Passing `apply --apply-preflight <path>` revalidates the retained preflight and requires its ledger checksum, resolved target, plan checksum, and embedded plan to match the supplied `apply` ledger snapshot and target before any database open; prefer that handoff when a migration window retains `apply-preflight.json` as the reviewed final pre-mutation evidence.
- If `--lock-file` already exists, assume another operator or interrupted run owns the migration window. Do not delete it blindly; first run `metin2-migrate apply-lock-status --lock-file <path> > apply-lock-status.json`, then inspect process ownership, the metadata-only lock JSON (`pid`, `hostname`, `build_version`, `build_commit`, `build_date`, `target_version`, `plan_sha256`, `ledger_snapshot_sha256`), retained preflight/audit artifacts, and deployment notes. The status helper is read-only: it validates the non-symlink regular lock file shape, returns `present: false` for an absent path, and never removes the lock, opens the DB target, or exposes the DSN / executable SQL. When the lock is present, status also reports advisory local holder liveness as `holder_pid_alive` / `holder_pid_check=local_signal_0` (signal-0 existence probe against `lock.pid`), advisory hostname locality as `holder_hostname_local` / `holder_hostname_check=local_os_hostname` (exact trimmed compare of `lock.hostname` to the inspecting host's `os.Hostname()`), advisory build-identity match as `holder_build_matches` / `holder_build_check=local_buildinfo_current` (exact trimmed compare of stamped `build_version` / `build_commit` / `build_date` to `buildinfo.Current()` on the inspecting binary), advisory wall-clock age as `lock_age_seconds` / `lock_age_check=local_wall_clock` (non-negative whole-second floor of age since `lock.created_at`; future-dated locks clamp to `0`), and advisory lab recovery triage as `manual_clear_candidate` / `manual_clear_check=lab_stale_lock_policy_v1` (`true` only when PID is absent, hostname is local, build identity matches, and age is at least 3600 seconds). Treat `holder_pid_alive=false`, `holder_hostname_local=false`, `holder_build_matches=false`, a large `lock_age_seconds`, or `manual_clear_candidate=true` as triage evidence only — PID reuse, container namespaces, copied lock files, unstamped `dev` binaries, clock skew, and cross-host leftover locks still require operator judgment, and the helper never auto-deletes the lock. On the single-host lab topology, follow [lab stale-lock recovery](lab-stale-lock-recovery.md) for the confirmation-gated `metin2-migrate apply-lock-aside --lock-file <path> --i-confirm-lab-aside-rename` recovery steps after a true candidate (or the documented manual `mv` fallback).
- If a retained `migration-apply-audit.json` must be checked later, run `metin2-migrate apply-audit-status --audit-file <path> > apply-audit-status.json` before trusting it in release evidence. The helper is read-only: it validates the non-symlink regular audit file shape, returns `present: false` for an absent path, verifies metadata/result/checksum consistency, and never removes the audit, opens the DB target, or exposes the DSN / executable SQL.
- If `apply` fails after reserving the lock or audit path, the CLI attempts to remove the reserved lock/audit files and roll back the SQL transaction. Keep stderr, the original ledger snapshot, the failed plan artifact, and the failed apply-preflight artifact for diagnosis.
- If the database reports an unknown or drifted ledger row, do not edit `schema_migrations` by hand. Compare `migration-catalog.json`, the deployed binary version, and the target database backup.

## Anti-goals

Do not use this runbook to justify:

- daemon-local `/local/db/migrations/apply` or rollback endpoints;
- daemon startup auto-migration;
- committing DSNs or secrets to git;
- DB-backed runtime claims for account, character, item, quest, content, login-ticket, or world state;
- stale-lock or stale-audit auto-removal without following [lab stale-lock recovery](lab-stale-lock-recovery.md) on the single-host lab topology (and never from the CLI/daemons themselves);
- treating `apply-lock-status` or `manual_clear_candidate=true` as authorization to delete an existing lock; status is inspection-only, and lab recovery uses confirmation-gated `apply-lock-aside` (aside-rename, never `rm`) after operator judgment.
- treating `apply-audit-status` as proof that a database is currently migrated; it validates a retained metadata artifact only.
- treating `ledger-snapshot-status` as proof that a live database still matches the retained snapshot; it validates a retained offline artifact against the embedded catalog only.
- treating `apply --apply-preflight` as a substitute for deployment-specific DB/file-store backup validation or transaction-local ledger verification.

## Related: quarantined export SQL import

After schema migrations are applied, operators can backfill retained migration-shaped exports with `metin2-migrate import-export --kind <kind> --export <path> --driver <driver> --dsn <dsn> --i-confirm-sql-import`. This reuses the programmatic `Import*` seams (insert-only, no upsert) and still does not register a stock production driver. See [CLI import-export](../plans/2026-08-27-cli-import-export.md).

For a retained `export-quarantine-drill` tree, print the reviewable import walk without embedding a DSN via:

```bash
export METIN2_IMPORT_DSN='<operator-supplied-dsn>'
metin2-migrate import-export-drill \
  --export-tree /var/metin2/exports/YYYYMMDDTHHMMSSZ-<commit12> \
  --driver <database/sql-driver> \
  --i-confirm-print-sql-import-drill
```

The printer never executes the imports, never opens a database, and never embeds the DSN value; each printed `import-export` line still requires `--i-confirm-sql-import` at execution time. The tree-owned print-only helper under `contrib/lab-retention-gc/` can optionally dump the same script as `import-export-drill.sh` when `METIN2_IMPORT_EXPORT_TREE` and `METIN2_IMPORT_DRIVER` are set — see [contrib import-export drill print helper](../plans/2026-08-27-contrib-import-export-drill-print-helper.md) and [lab retention / GC print-only unit samples](lab-retention-gc-unit-samples.md). Hermetic `/bin/sh` execution against build-tagged SQLite is owned by [hermetic import-export drill SQLite execution proof](../plans/2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md), including the seeded non-empty retained-tree twin in [hermetic import-export drill SQLite seeded tree](../plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md). See also [CLI import-export drill printer](../plans/2026-08-27-cli-import-export-drill.md).
