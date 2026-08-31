# Post-floor `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` restart recovery — 2026-08-31

## Objective

Close the remaining quickslot-edit recovery twins after packet
`QUICKSLOT_ADD` / `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` already owned quiet
post-floor denial and `QUICKSLOT_ADD` already owned `/restart_here` /
`/restart_town` recovery: prove the same restart paths restore a usable live
owner so occupied `QUICKSLOT_DEL` and occupied-to-occupied `QUICKSLOT_SWAP`
succeed normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying one template-backed item cell plus an occupied item quickslot
   and an occupied skill quickslot.
2. Later packet `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` fail closed with:
   - no `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` refresh frames
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP:
   - the same `QUICKSLOT_DEL` emits the ordinary self-only delete refresh and
     persists only the remaining skill binding
   - the same `QUICKSLOT_SWAP` emits the ordinary self-only swap refresh and
     persists the exchanged item/skill bindings
4. After `/restart_town` restores live HP at the owned empire town position, the
   same delete / swap likewise succeed and persist beside recovered MaxHP and
   the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorQuickslotDelFailsClosed`
   - `TestGameSessionFlowPostFloorQuickslotDelFailsClosedBeforeRestartTown`
   - `TestGameSessionFlowPostFloorQuickslotSwapFailsClosed`
   - `TestGameSessionFlowPostFloorQuickslotSwapFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific quickslot packet family
- widening into type-none `QUICKSLOT_ADD` clear restart twins in this same commit
- widening into partial `ITEM_DROP2`, merchant-sell, or give-success restart
  twins in this same commit
- changing already-owned live `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` behavior
- changing already-owned `QUICKSLOT_ADD` post-floor denial / recovery

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorQuickslot(Del|Swap)FailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
