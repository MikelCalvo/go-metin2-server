# CLI Backup/Restore Drill Crash-Temp Preflight — 2026-08-20

## Objective

Extend the read-only `metin2-migrate backup-restore-drill` printer so the emitted shell script includes the file-store runbook's optional per-store `validate` + `crash-temps/cleanup` triage before backup, without changing the CLI's own no-execute / no-DB / no-file-write boundary.

The first drill printer already printed aggregate preflight, backup, backup/validate, aside-rename, and restore curls. Operators still had to improvise the store-level validate / crash-temp cleanup sequence that `docs/workflow/file-store-backup-restore-drill.md` documents as the safe triage path when `/local/persistence/status` reports only crash-temp residue.

## Contract frozen by this slice

1. Successful `backup-restore-drill` stdout still begins with the existing path variables and aggregate preflight curls:
   - `GET /healthz`
   - `GET /local/runtime-config`
   - `GET /local/persistence/status`
2. Immediately after that aggregate preflight, the script now prints a dedicated `store validate / crash-temp triage` section in runbook store order:
   - `POST /local/account-store/validate`
   - `POST /local/account-store/crash-temps/cleanup`
   - `POST /local/login-tickets/validate`
   - `POST /local/login-tickets/crash-temps/cleanup`
   - `POST /local/item-templates/validate`
   - `POST /local/item-templates/crash-temps/cleanup`
   - `POST /local/interaction-store/validate`
   - `POST /local/interaction-store/crash-temps/cleanup`
   - `POST /local/static-actor-store/validate`
   - `POST /local/static-actor-store/crash-temps/cleanup`
   - `POST /local/quest-state/validate`
   - `POST /local/quest-state/crash-temps/cleanup`
   - a second `GET /local/persistence/status` so operators can confirm residue cleared before backup
3. Static-actor triage continues to use the `/local/static-actor-store/...` prefix; backup/restore continue to use `/local/static-actors/...`.
4. Script comments state explicitly that:
   - the CLI printer itself does not execute backup/restore;
   - printed `crash-temps/cleanup` curls mutate only hidden crash-temp residue after validate;
   - cleanup alone is not enough preparation for restore because committed snapshots and active `*-backup-manifest.json` still make destinations non-empty.
5. Existing input validation, shared-parent rejection, ops-base-url / backup-base rules, and no-SQL / no-DSN output guarantees are unchanged.

## What this is not yet

- automatic backup/restore execution by the CLI
- daemon startup auto-restore
- ground-item / ground-gold restart durability
- SQL-backed repository implementation or import/backfill
- remote admin auth
- changing the validate / crash-temp endpoint contracts themselves

## TDD and validation

Focused coverage:

- `go test ./internal/migratecli -run 'BackupRestoreDrill' -count=1`
- asserts the triage section appears after aggregate status and before backup
- asserts static-actor-store validate precedes static-actors backup
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
2. Add deployment topology / artifact retention docs once production hosts are known.
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
