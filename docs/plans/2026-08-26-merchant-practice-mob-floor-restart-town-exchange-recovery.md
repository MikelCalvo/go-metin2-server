# Merchant practice-mob floor post-`/restart_town` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after merchant floor `GC::SHOP END`
already lets `/restart_here` succeed at `EXCHANGE START`: prove that same
immediate / delayed practice-mob `0`-HP floor clear also lets `/restart_town`
succeed at destination-map `EXCHANGE START` against a living town peer,
matching the already-owned MYSHOP / cube / safebox / refine town recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `GC::SHOP END` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`.
2. Later `SHOP BUY` / `SHOP END` stay silent/no-frame; inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town-return position and a fresh requester `EXCHANGE START` against a
   living destination-map peer succeeds with ordinary paired start frames
   instead of the open-merchant busy info-chat reject.
4. Spec/QA name the same focused tests as the town recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMerchantBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMerchantBeforeRestartTownExchange`

## Explicit non-goals

- inventing a death-specific merchant packet family
- widening merchant buy/sell mutation beyond existing post-floor denials
- broader revive menus or corpse gameplay

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenMerchant(Window|BeforeRestartExchange|BeforeRestartTownExchange)$' -count=1
gofmt -w internal/minimal/merchant_practice_mob_floor_restart_town_exchange_test.go
git diff --check
```
