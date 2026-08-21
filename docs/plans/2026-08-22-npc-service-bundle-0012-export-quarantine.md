# NPC Service Bundle 0012 Export Quarantine — 2026-08-22

## Objective

Prove that the checked-in PvE QA fixture `docs/examples/bootstrap-npc-service-bundle.json` projects cleanly onto the landed `0012_static_actor_pve_interaction_state` export / quarantine tip without inventing SQL import/backfill or a new NPC service kind.

## Why now

- Migration `0012` already widens the static-actor / interaction export tip for `quest_flag`, `open_safebox`, kill-quest credit, and quest-flag item tables.
- Focused unit fixtures already cover those fields, but the real NPC QA bundle was not yet asserted end-to-end against export + quarantine.
- Operators need confidence that the playable content fixture can target the migration-shaped contract before any future driver-backed backfill work.

## Contract frozen by this slice

1. Canonicalizing `docs/examples/bootstrap-npc-service-bundle.json` and projecting its interactable static actors + spawn groups into `staticstore.Snapshot` / `interactionstore.Snapshot` yields a valid `0012` export.
2. `ValidateStaticActorContentStateExport` / `QuarantineStaticActorContentStateExport` accept that export and report:
   - 8 interaction definitions
   - 2 merchant catalog entries
   - 1 quest-flag reward item + 1 quest-flag consume item
   - 9 static actors (8 NPC/service actors + `QARewardMob`)
   - 1 reward drop
   - kinds including `info`, `talk`, `warp`, `shop_preview`, `open_safebox`, and `quest_flag`
3. The export preserves the owned PvE scalars:
   - `quest:first_steps_kill_turnin` reward/consume gold/experience/item tables
   - gated `npc:qa_warehouse` size/quest gate
   - `practice.qa_reward_mob` kill-quest credit + require gate + combat rewards/drop
4. No SQL driver, INSERT/backfill, or daemon-local mutating migration endpoint is added.

## Focused coverage

- `TestExampleBootstrapNPCServiceBundleExportsAndQuarantinesStaticActorPvEMigrationShape`

```bash
go test ./internal/contentbundle -run 'TestExampleBootstrapNPCServiceBundleExportsAndQuarantinesStaticActorPvEMigrationShape$' -count=1
```

## What this is not yet

- SQL-backed static-actor / interaction repositories at runtime
- import/backfill execution from quarantined exports
- branching quest scripts / new NPC service kinds
- durable safebox persistence / password load
