# Host MYSHOP practice-mob floor post-`/restart_town` exchange recovery — 2026-08-25

## Objective

Close the remaining combat-lane proof gap after host MYSHOP floor empty-sign
close already clears the private-shop busy flag for `/restart_here`: prove that
same immediate / delayed practice-mob `0`-HP floor clear also lets
`/restart_town` succeed at destination-map `EXCHANGE START` against a living
town peer.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one empty-sign `GC::SHOP_SIGN` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`,
   with the same empty-sign around-broadcast to currently visible source peers.
2. Later `/close_myshop` stays silent; inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town-return position and a fresh requester `EXCHANGE START` against a
   living destination-map peer succeeds with ordinary paired start frames
   instead of the open-private-shop busy info-chat reject.
4. Spec/QA name the same focused tests as the town recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMyShopBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShopBeforeRestartTownExchange`

## Explicit non-goals

- guest browse / buy / sell mutation beyond existing MYSHOP floor twins
- inventing a death-specific private-shop packet family
- widening cube / safebox / refine floor proofs to `/restart_town` in this slice

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenMyShopBeforeRestartTownExchange$' -count=1
gofmt -w internal/minimal/myshop_practice_mob_floor_close_test.go
git diff --check
```
