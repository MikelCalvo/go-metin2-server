# Kill-Reward Drop Process-Restart Rematerialize — 2026-08-22

## Objective

Close the combat-lane docs/test gap that still claimed kill-reward ownership /
despawn timers were deferred even though reward drops already register through
the same GroundItem FileStore path owned by player drops.

## Contract frozen by this slice

1. An accepted practice-mob killing hit that emits a fixed drop-vnum reward
   persists that pending ground handle through `GroundItemStorePath`.
2. A fresh `gameRuntime` rebuilt from the same FileStore rematerializes the
   exclusive kill-reward handle with absolute ownership/despawn timers.
3. After rejoin, exclusive ownership stays identity-keyed for the killer and
   blocks a peer mid-window; the killer may reclaim the rematerialized reward
   through ordinary `ITEM_PICKUP`.
4. Spec/QA stop saying restart-restored reward ownership/despawn state remains
   deferred; SQL import/backfill from quarantined `0010` exports stays out of
   scope.

## Focused coverage

- `TestGameRuntimePracticeMobKillRewardDropRematerializesAcrossDaemonRestart`

```bash
go test ./internal/minimal -run 'TestGameRuntimePracticeMobKillRewardDropRematerializesAcrossDaemonRestart$' -count=1
```

## What this is not yet

- weighted/random loot tables
- party-shaped owner-delivery notices
- SQL import/backfill from quarantined `0010` exports
- broader corpse / revive menus
