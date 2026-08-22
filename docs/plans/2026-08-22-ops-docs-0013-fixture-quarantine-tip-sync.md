# Ops Docs 0013 Fixture Quarantine Tip Sync — 2026-08-22

## Objective

Close the remaining operator-facing contradictions left after
`0013_static_actor_combat_profile_state` and the green content-bundle quarantine
proofs landed: historical NPC / PvE fixture plans and Track E still describe the
static-actor export/quarantine tip as migration `12`, and a couple of recent ops
follow-ups still claim pending ground-item restart durability is deferred.

No runtime behavior, SQL import, remote admin, or README churn is added.

## Why now

- `TestExampleBootstrapNPCServiceBundleExportsAndQuarantinesStaticActorPvEMigrationShape`
  and
  `TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles`
  already tip at `0013` / `static_actor_combat_profile_state`.
- `docs/plans/2026-08-22-npc-service-bundle-0012-export-quarantine.md` and
  `docs/plans/2026-08-22-pve-vertical-authoring-0012-export-combat-profile-gap.md`
  still narrate the tip as `0012` and treat portable `combat_profiles[]` as a
  current gap.
- Track E item 1 in `docs/plans/2026-08-08-playable-vertical-roadmap.md` still
  lists quarantine/preflight only through `0011`, omitting the landed `0012` /
  `0013` static-actor tip.
- Recent ops follow-ups (`cli-artifact-retention-gc-printer`,
  `lab-stale-lock-recovery-policy`) still say ground-item restart durability is
  deferred even though FileStore rematerialize + backup/restore already landed.

Those contradictions are production-ops hazards for export/quarantine and
reconnect/restart runbooks.

## Contract frozen by this slice

1. The NPC service fixture plan tips the export/quarantine proof at
   `0013_static_actor_combat_profile_state` (built-in `practice_mob` still
   yields empty portable `combat_profiles[]` / death-reward child rows) and
   marks the authoring-form combat-profile follow-up done via the green `0013`
   PvE vertical proof.
2. The PvE vertical authoring combat-profile gap plan is reframed as the
   historical `0012` fail-closed proof that motivated `0013`, with the current
   green command
   `TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles`.
3. `docs/plans/2026-08-21-static-actor-pve-interaction-state-migration.md`
   follow-up item 6 no longer claims the next tip still needs combat-profile
   rows; it points at the landed `0013` migration plan.
4. Track E item 1 lists quarantine/preflight through
   `0002`/`0003`/`0004`/`0007`/`0008`/`0009`/`0010`/`0011`/`0012`/`0013`.
5. Recent ops plans that still deferred ground-item restart durability mark that
   follow-up done and point at the landed rematerialize / backup/restore plans;
   SQL import/backfill from quarantined `0010` exports remains deferred.

## What this is not yet

- SQL import/backfill from quarantined exports
- DB driver selection / driver-backed harness
- durable safebox persistence / password load
- automatic artifact GC deletion
- systemd/unit samples that auto-run retention / GC printers
- remote admin authentication
- README churn beyond what these operator docs already require

## TDD and validation

Docs sync only; no new Go tests required.

Validation for this slice:

- `git diff --check`
- spot-check that NPC / PvE fixture plans and Track E quarantine wording tip at
  `0013` / `static_actor_combat_profile_state` where they describe the current
  tip
- confirm the PvE plan cites the green `...ExportsOnto0013AndQuarantines...`
  test name
- confirm recent ops follow-ups no longer claim ground-item restart durability
  is deferred

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep durable safebox persistence / password load deferred (items-lane
   contract freeze may land first).
3. Keep automatic artifact GC deletion deferred.
4. Optional later: systemd/unit samples that only print (never auto-run)
   retention / GC triage scripts.
