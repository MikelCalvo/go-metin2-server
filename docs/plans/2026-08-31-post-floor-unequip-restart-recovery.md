# Post-floor `/unequip_item` restart recovery — 2026-08-31

## Objective

Close the remaining equipment-mutation recovery twin after slash `/unequip_item`
already owned quiet post-floor denial: prove `/restart_here` and
`/restart_town` restore a usable live owner so the same slash unequip succeeds
normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while already wearing a template-backed weapon.
2. Later slash `/unequip_item` fails closed with:
   - no equipment-delete / inventory-set success burst
   - no inverse equip-effect `PLAYER_POINT_CHANGE`
   - no `CHARACTER_UPDATE` appearance refresh
   - no inventory / equipment / persistence mutation
3. After `/restart_here` restores live HP, the same `/unequip_item` emits the
   ordinary delete + set + inverse point-change + appearance burst and persists
   the carried weapon plus authored equip-effect removal.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `/unequip_item` likewise succeeds and persists the carried weapon beside
   the town-return position / recovered MaxHP minus the equip delta.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorUnequipItemFailsClosed`
   - `TestGameSessionFlowPostFloorUnequipItemFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific unequip packet family
- widening into drop / pickup / quickslot restart twins in this same commit
- changing already-owned live `/unequip_item` behavior
- changing already-owned `/equip_item` post-floor denial / recovery

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorUnequipItemFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
