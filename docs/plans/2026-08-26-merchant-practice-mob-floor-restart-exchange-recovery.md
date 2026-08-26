# Merchant practice-mob floor post-`/restart_here` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after merchant floor `GC::SHOP END`
already clears the open shop-preview context: prove that same immediate /
delayed practice-mob `0`-HP floor clear also lets `/restart_here` then succeed
at `EXCHANGE START` against a living peer, matching the already-owned MYSHOP /
safebox / refine / cube recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `GC::SHOP END` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`.
2. Later `SHOP BUY` / `SHOP END` stay silent/no-frame; inventory/gold stay unchanged.
3. After `/restart_here` from that same floor, a fresh requester `EXCHANGE START`
   against a living visible peer succeeds with ordinary paired start frames
   instead of the open-merchant busy info-chat reject.
4. Spec/QA name the same focused tests as the recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenMerchantBeforeRestartExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenMerchantBeforeRestartExchange`

## Explicit non-goals

- inventing a death-specific merchant packet family
- widening merchant buy/sell mutation beyond existing post-floor denials
- `/restart_town` destination-peer twin in this slice (follow the MYSHOP/cube/safebox/refine town twins later if still needed)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenMerchant(Window|BeforeRestartExchange)$' -count=1
gofmt -w internal/minimal/merchant_practice_mob_floor_restart_exchange_test.go
git diff --check
```
