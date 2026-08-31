# Post-floor `QUICKSLOT_ADD` restart recovery — 2026-08-31

## Objective

Close the remaining quickslot-edit recovery twin after packet
`QUICKSLOT_ADD` / `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` already owned quiet
post-floor denial: prove `/restart_here` and `/restart_town` restore a usable
live owner so the same `QUICKSLOT_ADD` succeeds normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying one template-backed item cell and an unrelated skill quickslot.
2. Later packet `QUICKSLOT_ADD` fails closed with:
   - no `QUICKSLOT_ADD` / `QUICKSLOT_DEL` refresh frames
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same `QUICKSLOT_ADD` emits the
   ordinary self-only add refresh and persists the new item binding beside the
   preserved skill quickslot.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `QUICKSLOT_ADD` likewise succeeds and persists the new binding beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorQuickslotAddFailsClosed`
   - `TestGameSessionFlowPostFloorQuickslotAddFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific quickslot packet family
- widening into `QUICKSLOT_DEL` / `QUICKSLOT_SWAP` restart twins in this same
  commit (denial remains covered by the existing practice-mob quickslot floor
  suite)
- widening into partial `ITEM_DROP2`, merchant-sell, or give restart twins in
  this same commit
- changing already-owned live `QUICKSLOT_ADD` behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorQuickslotAddFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
