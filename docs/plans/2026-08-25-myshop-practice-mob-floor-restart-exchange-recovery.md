# Host MYSHOP practice-mob floor post-`/restart_here` exchange recovery — 2026-08-25

## Objective

Close the remaining combat-lane proof gap after host MYSHOP floor empty-sign
close already clears the private-shop busy flag: prove that same immediate /
delayed practice-mob `0`-HP floor clear also lets `/restart_here` then succeed
at `EXCHANGE START` against a living peer, matching the already-owned safebox /
refine / lab cube recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one empty-sign `GC::SHOP_SIGN` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`,
   with the same empty-sign around-broadcast to currently visible peers.
2. Later `/close_myshop` stays silent; inventory/gold stay unchanged.
3. After `/restart_here` from that same floor, a fresh requester `EXCHANGE START`
   against a living visible peer succeeds with ordinary paired start frames
   instead of the open-private-shop busy info-chat reject.
4. Spec/QA name the same focused tests as the recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMyShop`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMyShop`

## Explicit non-goals

- guest browse / buy / sell mutation beyond existing MYSHOP floor twins
- inventing a death-specific private-shop packet family
- widening merchant / safebox / refine / cube floor proofs beyond their existing twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenMyShop$' -count=1
gofmt -w internal/minimal/myshop_practice_mob_floor_close_test.go
git diff --check
```
