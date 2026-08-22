# Ops Docs Ground-Item Lab Topology / Tip Sync — 2026-08-22

## Objective

Close the remaining operator-facing documentation contradictions left after
[ops docs ground-item / 0013 tip sync](2026-08-22-ops-docs-ground-item-0013-tip-sync.md):
lab topology still omitted the durable ground-item FileStore path, Track E and
migration-contract prose still treated ground-handle BackupTo/RestoreFrom and
process-restart rematerialize as deferred, and the CLI quarantine plan still
tipped static-actor content at migration `12`.

No runtime behavior, SQL import, remote admin, or README churn is added.

## Contract frozen by this slice

1. `docs/workflow/lab-deployment-topology.md` lists
   `/var/metin2/data/ground-items/ground-items.json`, the matching
   `METIN2_GAMED_GROUND_ITEM_STORE_PATH` export, and a `ground-items/` child under
   `/var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/`, matching the seventh store
   already owned by `metin2-migrate backup-restore-drill`.
2. Track E item 3 in `docs/plans/2026-08-08-playable-vertical-roadmap.md` marks
   ground-handle BackupTo/RestoreFrom + drill coverage done (seventh manifested
   store), not follow-on.
3. `docs/plans/2026-08-09-db-migration-contract.md` distinguishes the schema-only
   `0010` projection from the already-landed durable ground-item FileStore
   rematerialize + backup/restore path, tips static-actor content exit criteria
   at `0013_static_actor_combat_profile_state`, and records `0013` among the
   quarantine preflights already owned.
4. `docs/plans/2026-08-19-cli-export-quarantine.md` tips
   `static-actor-content-state` at `0013_static_actor_combat_profile_state` and
   strikes the stale “keep ground-item restart durability deferred” follow-up.
5. Historical slice plans that still listed ground-item restart durability as
   deferred (`2026-08-22-combat-profile-content-state-migration.md`,
   `2026-08-22-persistence-file-store-dedicated-parents.md`) mark that follow-up
   done and point at the landed rematerialize / backup/restore plans.

## What this is not yet

- SQL import/backfill from quarantined exports
- DB driver selection / driver-backed harness
- durable safebox persistence / password load
- automatic artifact GC deletion
- remote admin authentication
- README churn beyond what these operator docs already require

## TDD and validation

Docs sync only; no new Go tests required.

Validation for this slice:

- `git diff --check`
- spot-check that lab topology includes the ground-item store path and backup
  child directory
- confirm Track E / migration-contract / quarantine tip wording no longer claims
  ground handles are in-memory-only or that BackupTo/RestoreFrom remains deferred
- confirm static-actor content tip wording uses migration `13` /
  `static_actor_combat_profile_state` where it describes the current tip

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep durable safebox persistence / password load deferred.
3. Keep automatic artifact GC deletion deferred.
4. Optional later: systemd/unit samples that only print (never auto-run) retention
   / GC triage scripts.
