# Post-floor `ITEM_USE_TO_ITEM` restart recovery — 2026-08-31

## Objective

Close the remaining drag-to-item stack-merge recovery twins after packet
`ITEM_USE_TO_ITEM` already owned quiet post-floor denial and ordinary consumable
`ITEM_USE` already owned `/restart_here` / `/restart_town` recovery: prove the
same restart paths restore a usable live owner so both a full compatible stack
merge and a partial stack consolidation succeed normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying two compatible stackable potion stacks plus item/skill
   quickslots on the source/target cells.
2. Later packet `ITEM_USE_TO_ITEM` fails closed with:
   - no `ITEM_DEL` / `ITEM_UPDATE` / `QUICKSLOT_DEL` success burst
   - no inventory / quickslot / point / persistence mutation
3. After `/restart_here` restores live HP:
   - full compatible merge emits ordinary `ITEM_DEL` source + `ITEM_UPDATE` target +
     source item `QUICKSLOT_DEL`, persists the merged target stack, deletes only
     the source item quickslot, and leaves recovered MaxHP unchanged
   - partial consolidation emits ordinary source + target `ITEM_UPDATE`, preserves
     still-occupied source/target item and skill quickslots, and persists the
     merged counts beside recovered MaxHP
4. After `/restart_town` restores live HP at the owned empire town position, the
   same full and partial `ITEM_USE_TO_ITEM` paths likewise succeed and persist
   beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemUseToItemFailsClosed`
   - `TestGameSessionFlowPostFloorItemUseToItemFailsClosedBeforeRestartTown`
   - `TestGameSessionFlowPostFloorItemUseToItemPartialFailsClosed`
   - `TestGameSessionFlowPostFloorItemUseToItemPartialFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific use-to-item packet family
- widening into merchant buy/sell, safebox check-in/out/move success, or refine
  confirm restart twins in this same commit
- changing already-owned live `ITEM_USE_TO_ITEM` behavior
- changing already-owned ordinary consumable `ITEM_USE` post-floor recovery
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemUseToItem' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
