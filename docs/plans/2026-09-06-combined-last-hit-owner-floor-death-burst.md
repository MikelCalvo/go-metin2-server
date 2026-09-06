# Combined Last-Hit Owner-Floor Death Burst — 2026-09-06

## Objective

Close the remaining bootstrap DAMAGE_INFO / death-choreography gap where one
accepted practice-mob normal `ATTACK` could kill the dummy and would also drive
the engaged owner to the bootstrap `0`-HP floor, but the killing-hit path
dropped the owner-floor half.

## Why now

- Killing hits already emit dummy `DEAD(vid)` + selected-session `TARGET(0, 0)`
  + mob `DAMAGE_INFO`.
- Immediate owner-floor hits already emit `PLAYER_POINT_CHANGE(value=0)` +
  `DEAD(owner_vid)` + `TARGET(0, 0)` + owner `DAMAGE_INFO`, plus the mob
  companion when the dummy stays alive.
- A 1-HP dummy plus a 1-HP owner still took the killing-hit early return and
  never applied the immediate retaliation tick, so the owner stayed live while
  the dummy died.

## Contract owned by this slice

1. Dummy death still owns the first prefix:
   `DEAD(target_vid)` → `TARGET(0, 0)` → mob `DAMAGE_INFO`.
2. If that same accepted hit would also floor the owner through the ordinary
   immediate retaliation delta, the owner-floor suffix now follows in the same
   packet burst:
   `PLAYER_POINT_CHANGE(value=0)` → `DEAD(owner_vid)` → `TARGET(0, 0)` →
   owner `DAMAGE_INFO`.
3. Currently visible live peers receive dummy `DEAD` + dummy `DAMAGE_INFO`,
   then owner `DEAD` + owner `DAMAGE_INFO`.
4. A killing hit that leaves the owner above `0` still omits owner retaliation.
5. Delayed / proximity-armed floors stay unchanged: they never accepted an
   owner `ATTACK`, so they still omit a synthetic mob companion.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst$' -count=1
```

## What this is not yet

- delayed / proximity-armed floors inventing a synthetic mob hit-effect
- skill / ranged / PvP `DAMAGE_INFO` policy
- richer flag meanings (crit/miss/block)
- authored EXP / gold / drop rewards on that same combined last-hit (now owned
  by `2026-09-06-combined-last-hit-reward-then-floor.md`)
- same-socket `/restart_here` still-dead dummy catch-up after that combined
  last-hit (now owned by
  `2026-09-06-combined-last-hit-restart-here-still-dead-catch-up.md`)
- same-socket `/restart_here` kill-reward ground rematerialize plus pickup
  after that combined last-hit (now owned by
  `2026-09-06-combined-last-hit-restart-here-ground-catch-up.md`)
