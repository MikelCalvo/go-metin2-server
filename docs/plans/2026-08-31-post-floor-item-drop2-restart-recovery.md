# Post-floor partial `ITEM_DROP2` restart recovery — 2026-08-31

## Objective

Close the remaining partial-stack drop recovery twin after packet `ITEM_DROP` /
`ITEM_DROP2` already owned quiet post-floor denial and whole-stack /
gold `ITEM_DROP` already owned `/restart_here` / `/restart_town` recovery:
prove the same restart paths restore a usable live owner so a counted
`ITEM_DROP2` succeeds normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying one stackable potion stack plus item/skill quickslots on that
   source cell.
2. Later packet `ITEM_DROP2` fails closed with:
   - no `ITEM_UPDATE` / `GROUND_ADD` / `OWNERSHIP` success burst
   - no ground-item registration
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same counted `ITEM_DROP2` emits
   the ordinary update + ground-add + ownership burst, decrements the carried
   stack, preserves item/skill quickslots on the still-occupied cell, and
   registers one owned ground item for the dropped count at the recovery
   position.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same counted `ITEM_DROP2` likewise succeeds and registers the ground item at
   the town-return coordinates beside recovered MaxHP.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemDrop2FailsClosed`
   - `TestGameSessionFlowPostFloorItemDrop2FailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific drop packet family
- widening into merchant-sell or give-success restart twins in this same commit
- changing already-owned live `ITEM_DROP` / `ITEM_DROP2` behavior
- changing already-owned whole-stack / gold `ITEM_DROP` post-floor recovery

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemDrop2FailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
