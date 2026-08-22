# PvE Vertical Authoring 0012 Export Combat-Profile Gap — 2026-08-22

## Objective

Prove that the checked-in authoring-form QA fixture
`docs/examples/bootstrap-pve-vertical-authoring-bundle.json` expands through
`regen_spawns` + gated `drop_tables` into a valid `0012` static-actor /
interaction export **when** the portable custom combat profile is supplied on
the static snapshot, and that quarantine of that export **fails closed** because
migration `0012` still has no combat-profile row tip.

## Why now

- `docs/plans/2026-08-22-npc-service-bundle-0012-export-quarantine.md` already
  freezes the runtime-canonical NPC service fixture onto `0012`. That fixture
  uses built-in `practice_mob`, so quarantine succeeds.
- The playable PvE vertical authoring fixture uses portable
  `qa_pve_vertical_practice_mob` (formula-first HP/damage). Static-store
  validation requires that profile row on the snapshot, but the `0012` export
  shape only carries the actor's `combat_profile` **name**, not the portable
  profile definition.
- Operators need an explicit fail-closed proof of that tip gap before any future
  `0013` combat-profile content-state migration, instead of a silent false
  green quarantine claim.

## Contract frozen by this slice

1. Canonicalizing the PvE vertical authoring fixture strips authoring-only
   `regen_spawns` / `drop_tables` and expands `practice.qa_pve_vertical_mob`
   with the owned kill-quest / combat reward scalars plus the portable
   `qa_pve_vertical_practice_mob` combat profile.
2. Projecting that canonical content into `staticstore.Snapshot` **including**
   `CombatProfiles` + `interactionstore.Snapshot` yields a successful
   `ExportStaticActorContentState` onto migration tip `12` /
   `static_actor_pve_interaction_state`.
3. That export preserves the owned PvE actor/interaction scalars:
   - 8 interaction definitions / 2 merchant catalog entries
   - 1 quest-flag reward item + 1 quest-flag consume item
   - 9 static actors including `QAPveVerticalMob`
   - 1 reward drop (`27001`)
   - `quest:first_steps_kill_turnin` reward/consume gold/experience/item tables
   - gated `npc:qa_warehouse`
   - `practice.qa_pve_vertical_mob` kill-quest credit + require gate + combat
     rewards, with `combat_profile = qa_pve_vertical_practice_mob`
4. `ValidateStaticActorContentStateExport` / `QuarantineStaticActorContentStateExport`
   for that export fail closed with
   `ErrInvalidStaticActorContentStateExport` wrapping the static-snapshot
   validation failure caused by the missing portable combat-profile row after
   export round-trip (export does not retain `combat_profiles[]`).
5. No SQL migration, import/backfill, or daemon-local mutating endpoint is added.

## Focused coverage

- `TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0012ButQuarantineFailsClosedWithoutCombatProfiles`

```bash
go test ./internal/contentbundle -run 'TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0012ButQuarantineFailsClosedWithoutCombatProfiles$' -count=1
```

## What this is not yet

- `0013` / combat-profile content-state SQL tip or export rows
- import/backfill execution from quarantined exports
- weighted/random loot tables or branching quest scripts
- durable safebox persistence / password load
- changing the authoring fixture to built-in `practice_mob` just to force a
  green quarantine (that would hide the formula-profile QA path)

## Follow-up

1. Persistence/content migration follow-up: add a combat-profile content-state tip
   (likely `0013`) that retains portable `combat_profiles[]` beside the actor
   name reference, then re-prove full export+quarantine for this authoring
   fixture.
2. Keep weighted/random loot and branching quest scripts deferred.
