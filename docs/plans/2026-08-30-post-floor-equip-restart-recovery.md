# Post-floor `/equip_item` restart recovery — 2026-08-30

## Objective

Close the remaining equipment-mutation recovery twin after slash `/equip_item`
already owned quiet post-floor denial: prove `/restart_here` and
`/restart_town` restore a usable live owner so the same slash equip succeeds
normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob.
2. Later slash `/equip_item` fails closed with:
   - no item-delete / item-set success burst
   - no equip-effect `PLAYER_POINT_CHANGE`
   - no `CHARACTER_UPDATE` appearance refresh
   - no inventory / equipment / persistence mutation
3. After `/restart_here` restores live HP, the same `/equip_item` emits the
   ordinary delete + set + point-change + appearance burst and persists the
   equipped weapon plus authored equip-effect delta.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `/equip_item` likewise succeeds and persists the equipped weapon beside
   the town-return position / recovered MaxHP + equip delta.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorEquipItemFailsClosed`
   - `TestGameSessionFlowPostFloorEquipItemFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific equip packet family
- widening into unequip / drop / pickup / quickslot restart twins in this same
  commit
- changing already-owned live `/equip_item` behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorEquipItemFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
