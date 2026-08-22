# PvE Vertical Authoring 0012 Export Combat-Profile Gap — 2026-08-22

## Objective

Originally: prove that the checked-in authoring-form QA fixture
`docs/examples/bootstrap-pve-vertical-authoring-bundle.json` expands through
`regen_spawns` + gated `drop_tables` into a valid `0012` static-actor /
interaction export **when** the portable custom combat profile is supplied on
the static snapshot, and that quarantine of that export **fails closed** because
migration `0012` still had no combat-profile row tip.

Current tip status (after
[combat-profile content-state migration](2026-08-22-combat-profile-content-state-migration.md)):
the same fixture now exports and quarantines cleanly onto
`0013_static_actor_combat_profile_state` with portable `combat_profiles[]`
retained. Keep this plan as the historical fail-closed proof that motivated
`0013`, plus the landed green `0013` command operators should run today.

## Why the historical gap mattered

- `docs/plans/2026-08-22-npc-service-bundle-0012-export-quarantine.md` freezes the
  runtime-canonical NPC service fixture onto the current static-actor tip. That
  fixture uses built-in `practice_mob`, so quarantine succeeds with empty
  portable combat-profile rows.
- The playable PvE vertical authoring fixture uses portable
  `qa_pve_vertical_practice_mob` (formula-first HP/damage). Static-store
  validation requires that profile row on the snapshot; migration `0012` only
  carried the actor's `combat_profile` **name**, not the portable profile
  definition.
- Operators needed an explicit fail-closed proof of that tip gap before `0013`,
  instead of a silent false green quarantine claim.

## Historical contract frozen at the `0012` tip

1. Canonicalizing the PvE vertical authoring fixture strips authoring-only
   `regen_spawns` / `drop_tables` and expands `practice.qa_pve_vertical_mob`
   with the owned kill-quest / combat reward scalars plus the portable
   `qa_pve_vertical_practice_mob` combat profile.
2. Projecting that canonical content into `staticstore.Snapshot` **including**
   `CombatProfiles` + `interactionstore.Snapshot` yielded a successful
   `ExportStaticActorContentState` onto migration tip `12` /
   `static_actor_pve_interaction_state` (name reference only).
3. That export preserved the owned PvE actor/interaction scalars:
   - 8 interaction definitions / 2 merchant catalog entries
   - 1 quest-flag reward item + 1 quest-flag consume item
   - 9 static actors including `QAPveVerticalMob`
   - 1 reward drop (`27001`)
   - `quest:first_steps_kill_turnin` reward/consume gold/experience/item tables
   - gated `npc:qa_warehouse`
   - `practice.qa_pve_vertical_mob` kill-quest credit + require gate + combat
     rewards, with `combat_profile = qa_pve_vertical_practice_mob`
4. At tip `12`, `ValidateStaticActorContentStateExport` /
   `QuarantineStaticActorContentStateExport` failed closed with
   `ErrInvalidStaticActorContentStateExport` wrapping the static-snapshot
   validation failure caused by the missing portable combat-profile row after
   export round-trip (export did not retain `combat_profiles[]`).
5. No SQL import/backfill or daemon-local mutating endpoint was added by that
   proof.

## Current tip (`0013`) proof

After `0013_static_actor_combat_profile_state`, the same fixture exports and
quarantines with portable profile rows retained:

- `TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles`

```bash
go test ./internal/contentbundle -run 'TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles$' -count=1
```

That green proof reports one portable combat profile
(`qa_pve_vertical_practice_mob`) plus matching death-reward drop child rows
beside the same PvE actor/interaction scalars above.

## What this is not yet

- import/backfill execution from quarantined exports
- weighted/random loot tables or branching quest scripts
- durable safebox persistence / password load
- changing the authoring fixture to built-in `practice_mob` just to force a
  green quarantine (that would hide the formula-profile QA path)

## Follow-up

1. ~~Persistence/content migration follow-up: add a combat-profile content-state tip
   (likely `0013`) that retains portable `combat_profiles[]` beside the actor
   name reference, then re-prove full export+quarantine for this authoring
   fixture.~~ Done: see
   [combat-profile content-state migration](2026-08-22-combat-profile-content-state-migration.md)
   and
   `TestExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles`.
2. Keep weighted/random loot and branching quest scripts deferred.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
