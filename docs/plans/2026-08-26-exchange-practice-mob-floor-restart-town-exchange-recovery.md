# Exchange-shell practice-mob floor post-`/restart_town` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after open bootstrap `EXCHANGE`
already emits self/peer `GC::EXCHANGE END` on the practice-mob `0`-HP floor and
`/restart_here` already recovers a fresh same-map `EXCHANGE START`: prove that
same immediate / delayed floor clear also lets `/restart_town` succeed at
destination-map `EXCHANGE START` against a living town peer, matching the
already-owned merchant / MYSHOP / cube / safebox / refine town recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `GC::EXCHANGE END` after `POINT_CHANGE(0)` → `DEAD` →
   `TARGET(0,0)`, and queue one `GC::EXCHANGE END` to the paired live source-map
   peer.
2. Later `EXCHANGE CANCEL` / display requests stay silent/no-frame;
   inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town-return position and a fresh requester `EXCHANGE START` against a
   living destination-map peer succeeds with ordinary paired start frames
   instead of operating on a stale exchange pairing or busy reject.
4. Spec/QA name the same focused tests as the town recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange`

## Explicit non-goals

- inventing a death-specific exchange packet family
- widening exchange mutation beyond the already-owned floor close
- shop-bag consume / require-bag / OR-materials / binary cube headers

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenExchangeShell(BeforeRestartExchange|BeforeRestartTownExchange)$' -count=1
gofmt -w internal/minimal/exchange_practice_mob_floor_restart_town_exchange_test.go
git diff --check
```
