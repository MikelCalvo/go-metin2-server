# Post-floor ITEM_MOVE restart recovery — 2026-08-30

## Objective

Close the remaining inventory-mutation recovery twin after packet/slash
`ITEM_MOVE` already owned quiet post-floor denial: prove `/restart_here` and
`/restart_town` restore a usable live owner so the same carried move succeeds
normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob.
2. Later packet `ITEM_MOVE` and slash `/inventory_move` fail closed with:
   - no item-delete / item-set success burst
   - no quickslot refresh
   - no inventory / gold / persistence mutation
3. After `/restart_here` restores live HP, the same packet `ITEM_MOVE` emits the
   ordinary item-delete + item-set success burst and persists the destination slot.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same packet `ITEM_MOVE` likewise succeeds and persists the destination slot
   beside the town-return position / recovered MaxHP.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemMoveFailsClosed`
   - `TestGameSessionFlowPostFloorItemMoveFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific inventory-move packet family
- widening into equip/unequip, drop, pickup, or quickslot restart twins in this
  same commit
- changing already-owned live `ITEM_MOVE` merge/swap/equip behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemMoveFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
