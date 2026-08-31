# Post-floor ordinary consumable `ITEM_USE` restart recovery — 2026-08-31

## Objective

Close the remaining ordinary consumable-use recovery twin after packet
`ITEM_USE` / slash `/use_item` already owned quiet post-floor denial: prove
`/restart_here` and `/restart_town` restore a usable live owner so the same
`ITEM_USE` succeeds normally again.

Shop/silk bag USE already owned dedicated restart twins; this slice covers the
template-backed point-changing consumable path used by the PvE loop.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while carrying one template-backed stackable consumable with authored
   `use_effect` (negative HP delta avoids post-restart MaxHP overflow clamping).
2. Later packet `ITEM_USE` and slash `/use_item` fail closed with:
   - no use echo / `PLAYER_POINT_CHANGE` / `ITEM_UPDATE` / info-chat success burst
   - no inventory / point / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same `ITEM_USE` emits the ordinary
   use-echo + point-change + stack refresh + authored info-chat burst and
   persists the stack decrement beside the recovered MaxHP minus the authored
   delta.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `ITEM_USE` likewise succeeds and persists the stack decrement beside
   recovered MaxHP minus the authored delta and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemUseFailsClosed`
   - `TestGameSessionFlowPostFloorItemUseFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific item-use packet family
- widening into `ITEM_USE_TO_ITEM`, merchant buy/sell, `ITEM_GIVE`, safebox
  check-in/out/move, refine confirm, gold-pickup, or `QUICKSLOT_DEL` /
  `QUICKSLOT_SWAP` restart twins in this same commit
- changing already-owned live consumable-use behavior
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemUseFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
