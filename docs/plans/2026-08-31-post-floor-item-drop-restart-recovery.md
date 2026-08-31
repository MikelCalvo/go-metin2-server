# Post-floor `ITEM_DROP` restart recovery — 2026-08-31

## Objective

Close the remaining inventory-drop recovery twin after packet `ITEM_DROP` /
`ITEM_DROP2` already owned quiet post-floor denial: prove `/restart_here` and
`/restart_town` restore a usable live owner so the same whole-stack drop
succeeds normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying one whole stack.
2. Later packet `ITEM_DROP` fails closed with:
   - no `ITEM_DEL` / `GROUND_ADD` / `OWNERSHIP` success burst
   - no ground-item registration
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same `ITEM_DROP` emits the
   ordinary delete + ground-add + ownership burst, clears the carried slot, and
   registers one owned ground item at the recovery position.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `ITEM_DROP` likewise succeeds and registers the ground item at the
   town-return coordinates beside recovered MaxHP.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemDropFailsClosed`
   - `TestGameSessionFlowPostFloorItemDropFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific drop packet family
- widening into gold-drop or partial `ITEM_DROP2` restart twins in this same
  commit (pickup restart recovery is owned separately in
  `2026-08-31-post-floor-item-pickup-restart-recovery.md`; quickslot `ADD`
  restart recovery is owned separately in
  `2026-08-31-post-floor-quickslot-add-restart-recovery.md`)
- changing already-owned live `ITEM_DROP` / `ITEM_DROP2` behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemDropFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
