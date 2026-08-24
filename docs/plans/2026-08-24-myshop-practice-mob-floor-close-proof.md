# MYSHOP Practice-Mob Floor Close Proof — 2026-08-24

## Objective

Close the combat-lane proof gap where host-only `MYSHOP` empty-sign close was
already wired on the practice-mob death floor beside merchant/exchange/safebox
teardown, but only lifecycle `/quit|/logout|/phase_select` and `/close_myshop`
had focused coverage.

## Contract frozen by this slice

1. Immediate and delayed practice-mob retaliation that reach owner HP `0` while
   an accepted host-only `MYSHOP` is open append one empty-sign `GC::SHOP_SIGN`
   after `PLAYER_POINT_CHANGE(value=0)` → `DEAD(owner_vid)` → `TARGET(0, 0)`.
2. Ordering stays merchant `SHOP END` before MYSHOP empty-sign before safebox
   `CloseSafebox` / exchange `END` when those shells close together.
3. Currently visible peers receive the same empty-sign around-broadcast already
   owned by the MYSHOP close companion.
4. Later `/close_myshop` stays silent; inventory/gold stay unchanged.
5. Player-death / combat / QA docs name the floor companion explicitly.

## Focused coverage

- `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMyShop`
- `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShop`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob(Immediate|Delayed)RetaliationFloorClosesOpenMyShop$' -count=1
```

## What this is not yet

- guest browse/buy or view-entry live-sign rematerialization
- inventing a death-specific private-shop packet family
- remapping proximity suppress across daemon restart
- weighted/random loot tables or broader corpse / revive menus
