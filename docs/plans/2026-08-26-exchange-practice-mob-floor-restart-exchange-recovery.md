# Exchange-shell practice-mob floor post-`/restart_here` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after open bootstrap `EXCHANGE`
already emits self/peer `GC::EXCHANGE END` on the practice-mob `0`-HP floor:
prove that same immediate / delayed floor clear also lets `/restart_here` then
succeed at a fresh `EXCHANGE START` against the same living peer, matching the
already-owned merchant / MYSHOP / cube / safebox / refine recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `GC::EXCHANGE END` after `POINT_CHANGE(0)` → `DEAD` →
   `TARGET(0,0)`, and queue one `GC::EXCHANGE END` to the paired live peer.
2. Later `EXCHANGE CANCEL` / display requests stay silent/no-frame;
   inventory/gold stay unchanged.
3. After `/restart_here` from that same floor, a fresh requester `EXCHANGE START`
   against the same living visible peer succeeds with ordinary paired start
   frames instead of operating on a stale exchange pairing or busy reject.
4. Spec/QA name the same focused tests as the recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenExchangeShellBeforeRestartExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenExchangeShellBeforeRestartExchange`

## Explicit non-goals

- inventing a death-specific exchange packet family
- widening exchange mutation beyond the already-owned floor close
- `/restart_town` destination-peer twin in this slice (follow the merchant /
  MYSHOP / cube / safebox / refine town twins later if still needed)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenExchangeShell(BeforeRestartExchange)?$' -count=1
gofmt -w internal/minimal/exchange_practice_mob_floor_restart_exchange_test.go
git diff --check
```
