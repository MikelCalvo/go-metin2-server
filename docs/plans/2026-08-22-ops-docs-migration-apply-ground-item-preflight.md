# Ops Docs Migration-Apply Ground-Item Preflight — 2026-08-22

## Objective

Close the remaining operator-facing contradiction in the migration apply
runbook: after the durable ground-item FileStore rematerialize + backup/restore
drill landed as the seventh manifested store, the apply preflight still listed
only six file-backed validate / backup-validate surfaces and omitted
`/local/ground-item-store/*`.

No runtime behavior, SQL import, remote admin, or README churn is added.

## Why now

- `docs/workflow/file-store-backup-restore-drill.md`, lab topology, Track E, and
  `metin2-migrate backup-restore-drill` already treat pending ground handles as
  the seventh manifested store.
- `docs/workflow/migration-apply-runbook.md` still tells operators to validate
  only accounts, login tickets, item templates, interactions, static actors, and
  quest state before DB mutation.
- That omission is a production-ops hazard for reconnect/restart windows that
  also apply SQL: operators can leave live ground-item/gold state unvalidated
  while treating the file-store half of apply preflight as complete.

## Contract frozen by this slice

1. `docs/workflow/migration-apply-runbook.md` lists pending ground item/gold
   handles (`/local/ground-item-store/validate`,
   `/local/ground-item-store/backup/validate` when using a store backup) beside
   the six older bootstrap JSON stores in the pre-apply file-store validation
   checklist.
2. The same runbook still points combined multi-store backup/restore at
   [file-store backup/restore drill](../workflow/file-store-backup-restore-drill.md)
   instead of improvising per-store ordering.
3. Historical plan wording that claimed the apply runbook listed “all six”
   store surfaces (`docs/plans/2026-08-19-file-store-backup-restore-drill.md`)
   notes that the seventh ground-item surface is now owned by the apply
   runbook as well.

## What this is not yet

- SQL import/backfill from quarantined `0010` exports
- DB driver selection / driver-backed harness
- durable safebox persistence / password load
- automatic artifact GC deletion
- remote admin authentication
- README churn beyond what these operator docs already require

## TDD and validation

Docs sync only; no new Go tests required.

Validation for this slice:

- `git diff --check`
- spot-check that the apply-runbook preflight list includes
  `/local/ground-item-store/validate` and
  `/local/ground-item-store/backup/validate`
- confirm the combined-drill link remains the multi-store sequencing authority

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep durable safebox persistence / password load deferred.
3. Keep automatic artifact GC deletion deferred.
4. Optional later: systemd/unit samples that only print (never auto-run)
   retention / GC triage scripts.
