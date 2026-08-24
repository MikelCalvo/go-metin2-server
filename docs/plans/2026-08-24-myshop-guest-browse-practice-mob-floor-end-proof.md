# MYSHOP guest browse practice-mob floor END proof — 2026-08-24

## Objective

Close the combat-lane proof gap where guest-browse `GC::SHOP END` was already
wired on the practice-mob death floor beside host empty-sign / merchant /
exchange / safebox teardown, but only explicit guest `SHOP END` and host
`/close_myshop` had focused coverage.

## Contract frozen by this slice

1. When a browsing guest's MYSHOP host reaches owner HP `0` through practice-mob
   retaliation, the guest receives one queued self-only `GC::SHOP END` beside the
   host empty-sign around-broadcast and clears browse association.
2. When a browsing guest themselves reach owner HP `0` through practice-mob
   retaliation, that guest appends one self-only `GC::SHOP END` after
   `PLAYER_POINT_CHANGE(value=0)` → `DEAD` → `TARGET(0, 0)` and clears browse
   without host empty-sign or inventory/gold mutation.
3. Later guest `SHOP END` / dead-guest `ON_CLICK` stay silent/no-frame.
4. Player-death / QA docs name the guest floor companion beside host empty-sign.

## Focused coverage

- `TestGameSessionFlowPracticeMobImmediateRetaliationFloorQueuesGuestBrowseShopEnd`
- `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesGuestBrowseOnDeadGuest`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobImmediateRetaliationFloor(QueuesGuestBrowseShopEnd|ClosesGuestBrowseOnDeadGuest)$' -count=1
```

## What this is not yet

- inventing a death-specific private-shop packet family
- guest sell-into-PC-shop or cube busy rejects
- weighted/random loot tables or broader corpse / revive menus
