# Post-floor gold `ITEM_DROP` restart recovery — 2026-08-31

## Objective

Close the remaining gold-drop recovery twin after packet `ITEM_DROP` /
`ITEM_DROP2` already owned quiet post-floor denial for both inventory items and
gold, and after whole-stack inventory `ITEM_DROP` already owned restart recovery:
prove `/restart_here` and `/restart_town` restore a usable live owner so the same
gold `ITEM_DROP` succeeds normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying enough gold for a non-zero drop.
2. Later gold `ITEM_DROP` (`Elk` / gold field non-zero) fails closed with:
   - no gold `PLAYER_POINT_CHANGE`
   - no `GROUND_ADD` / `OWNERSHIP` success burst
   - no ground-item registration
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same gold `ITEM_DROP` emits the
   ordinary gold point-change + ground-add (`vnum = 1`) + ownership burst, debits
   carried gold, and registers one owned gold marker at the recovery position.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same gold `ITEM_DROP` likewise succeeds and registers the gold marker at the
   town-return coordinates beside recovered MaxHP and the reduced gold total.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorGoldDropFailsClosed`
   - `TestGameSessionFlowPostFloorGoldDropFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific gold-drop packet family
- widening into partial `ITEM_DROP2` restart twins in this same commit (pickup
  restart recovery is owned separately in
  `2026-08-31-post-floor-item-pickup-restart-recovery.md`; quickslot `ADD`
  restart recovery is owned separately in
  `2026-08-31-post-floor-quickslot-add-restart-recovery.md`)
- changing already-owned live gold-drop behavior
- changing already-owned whole-stack inventory `ITEM_DROP` post-floor denial /
  recovery

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorGoldDropFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
