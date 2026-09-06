# Combined Last-Hit Reward-Then-Floor — 2026-09-06

## Objective

Close the remaining bootstrap kill-reward gap on the combined last-hit: one
accepted practice-mob normal `ATTACK` can kill the dummy and floor the engaged
owner, but that same hit must still apply authored EXP / gold / drop rewards
before the owner-floor suffix.

## Why now

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst`
  already owns dummy death first, then the owner-floor suffix.
- Ordinary killing hits already emit EXP / gold / `ITEM_GROUND_ADD` /
  `ITEM_OWNERSHIP` after dummy `DEAD` / `TARGET(0, 0)` / mob `DAMAGE_INFO`.
- Ground-reward registration fails closed once the owner is at the bootstrap
  `0`-HP floor. If owner-floor ran first, the killer would lose the drop even
  though the kill was accepted.
- The current combined last-hit fixture is rewardless, so that ordering was
  unproven.

## Contract owned by this slice

1. Dummy death still owns the first prefix:
   `DEAD(target_vid)` → `TARGET(0, 0)` → mob `DAMAGE_INFO`.
2. Authored EXP / gold / drop reward frames stay next, while the killer is
   still live for scalar persist and ground-item registration.
3. The owner-floor suffix still follows in the same packet burst:
   `PLAYER_POINT_CHANGE(value=0)` → `DEAD(owner_vid)` → `TARGET(0, 0)` →
   owner `DAMAGE_INFO`.
4. Currently visible live peers receive dummy `DEAD` + dummy `DAMAGE_INFO`,
   then the ground-add / ownership pair, then owner `DEAD` + owner
   `DAMAGE_INFO`. They still do not receive the killer's self-only EXP/gold
   point-changes.
5. The accepted kill still persists scalar EXP/gold and owner HP `0`, and the
   drop remains a live ground item. Stale post-floor `ATTACK` still fails
   closed.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsRewardsBeforeOwnerFloor`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsRewardsBeforeOwnerFloor$' -count=1
```

## What this is not yet

- delayed / proximity-armed floors inventing kill rewards
- party share, random loot tables, or level-up choreography

Pickup after same-socket `/restart_here` rematerialize of that drop is now owned
by `2026-09-06-combined-last-hit-restart-here-ground-catch-up.md`.
