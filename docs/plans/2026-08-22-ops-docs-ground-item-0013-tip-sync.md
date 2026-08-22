# Ops Docs Ground-Item / 0013 Tip Sync — 2026-08-22

## Objective

Bring operator-facing persistence docs and a couple of stale export-helper
comments back in line with the already-landed ground-item FileStore rematerialize
+ backup/restore drill coverage and the `0013_static_actor_combat_profile_state`
static-actor content export/quarantine tip — without inventing SQL import,
remote admin, or new runtime behavior.

After `feat(persistence): rematerialize pending ground items across restart`,
`feat(persistence): backup/restore durable ground-item FileStore`, and
`feat(db): tip static-actor content export at combat-profile 0013`, several
operator docs still claimed:

- pending ground handles were in-memory-only / not restart-durable
- the backup/restore drill accepted `ground_item_store_path` but did not cover
  BackupTo/RestoreFrom
- static-actor content export/quarantine still tipped at migration `12` /
  `static_actor_pve_interaction_state`
- combat-profile tables were still "deliberately out of scope" for the catalog

Those contradictions are production-ops hazards for reconnect/restart and
export/quarantine runbooks, so this slice closes the documentation contract
gap only.

## Contract frozen by this slice

1. `docs/development.md` states that pending ground item/gold handles
   rematerialize from the dedicated ground-item FileStore across `gamed`
   restart, with the focused proof
   `TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart`,
   and points operators at the backup/restore drill for
   `/local/ground-item-store/*`.
2. `docs/development.md` lists `0013_static_actor_combat_profile_state` in the
   embedded catalog tip narrative and describes the portable
   `static_actor_combat_profiles` /
   `static_actor_combat_profile_death_reward_drops` schema boundary.
3. Quarantine tip wording for static-actor content uses migration version `13`
   / `static_actor_combat_profile_state` (retained `12` artifacts fail closed).
4. `docs/debugging-and-profiling.md` documents the static-actor content-state
   export/quarantine endpoints against the `0013` tip, including
   `combat_profiles` / `combat_profile_death_reward_drops` and quarantine
   summary fields for those collections.
5. Ground-item `0010` export wording no longer claims handles are not durable
   across restart; it distinguishes the live migration-shaped projection from
   the durable FileStore restart path.
6. Runtime-config persistence wording no longer claims the backup/restore drill
   omits ground-handle BackupTo/RestoreFrom.
7. `docs/workflow/file-store-backup-restore-drill.md` anti-goals no longer deny
   ground-item restart durability (they keep denying DB-backed repositories and
   daemon-local apply).
8. Stale `internal/staticstore` export-helper comments that still said "0012
   tip" are corrected to the current `0013` tip (comment-only; no behavior
   change).

## What this is not yet

- SQL import/backfill from quarantined exports
- DB driver selection / driver-backed harness
- durable safebox persistence / password load
- rebinding process-local `OwnerID` when an exclusive ground-item owner rejoins
- automatic artifact GC deletion
- remote admin authentication
- README churn beyond what these operator docs already require

## TDD and validation

Docs/comment sync only; no new Go tests required.

Validation for this slice:

- `git diff --check`
- spot-check that `StaticActorContentStateMigrationVersion` remains `13` /
  `static_actor_combat_profile_state`
- confirm no remaining operator-facing claims that ground handles are
  in-memory-only or that the drill omits ground-item backup/restore

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Optional later: rebind process-local `OwnerID` when the exclusive owner
   rejoins the shared world.
3. Keep durable safebox persistence / password load deferred.
4. Keep automatic artifact GC deletion deferred.
