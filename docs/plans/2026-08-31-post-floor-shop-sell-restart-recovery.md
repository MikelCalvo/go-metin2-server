# Post-floor merchant `SHOP SELL` restart recovery — 2026-08-31

## Objective

Close the remaining merchant-sell recovery twin after packet `SHOP SELL` /
`SHOP SELL2` already owned quiet post-floor denial: prove `/restart_here` /
`/restart_town` restore a usable live owner that can freshly reopen a merchant
and complete an ordinary whole-stack `SHOP SELL`.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a merchant window is already open and the owner carries one
   sellable stack plus an item quickslot on that cell.
2. Later packet `SHOP SELL` fails closed with no frames and no inventory /
   gold / quickslot / persistence mutation.
3. After `/restart_here` restores live HP, wait past the owned static-actor
   interaction cooldown, then a fresh merchant `INTERACT` emits ordinary
   `GC::SHOP START`, and the same `SHOP SELL` emits `ITEM_DEL` + source item
   `QUICKSLOT_DEL` + gold `PLAYER_POINT_CHANGE` and persists empty inventory /
   credited gold / remaining skill quickslot beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + sell path against a destination-map merchant likewise
   succeeds and persists beside recovered MaxHP and the town-return
   coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorShopSellFailsClosed`
   - `TestGameSessionFlowPostFloorShopSellFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific merchant packet family
- widening into `SHOP BUY`, `SELL2`, safebox check-in success, or refine confirm
  twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorShopSellFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_shop_sell_restart_recovery_test.go
git diff --check
```
