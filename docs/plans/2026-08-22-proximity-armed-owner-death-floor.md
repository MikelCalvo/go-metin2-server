# Proximity-Armed Owner Death Floor — 2026-08-22

## Objective

Prove the owned player-death floor vertical for delayed server-origin retaliation
that was armed only by proximity aggro (no `TARGET`, no `ATTACK`).

## Contract frozen by this slice

1. Walking into default/effective aggro radius may arm delayed retaliation without
   inventing selected-target ownership or an immediate retaliation piggyback.
2. When that proximity-armed delayed beat reaches owner HP `0`, emit:
   - self `PLAYER_POINT_CHANGE(value = 0)`
   - self `DEAD(owner_vid)`
   - self `TARGET(0, 0)` even though no prior selection existed
3. Persist bootstrap HP `0`; release aggro-lite engagement; stop further delayed
   beats; omit owner-floor `DAMAGE_INFO`.
4. Visible live peers receive one queued `DEAD(owner_vid)`; post-floor owner
   `TARGET` / `ATTACK` fail closed; a third party may freshly `TARGET` the
   still-live mob after engagement release.
5. Non-floor proximity walk-away outside aggro radius remains silent (no invented
   `TARGET(0, 0)`).

## Focused coverage

- `TestGameRuntimeProximityAggroDelayedRetaliationReachesOwnerDeathFloorWithoutHitOrTarget`

```bash
go test ./internal/minimal -run 'TestGameRuntimeProximityAggroDelayedRetaliationReachesOwnerDeathFloorWithoutHitOrTarget$' -count=1
```

## What this is not yet

- broader corpse / revive menus
- skill / ranged / PvP runtime policy
- weighted/random loot tables
