# Exchange-shell practice-mob floor post-`/restart_town` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after open bootstrap `EXCHANGE`
already emits self/peer `GC::EXCHANGE END` on the practice-mob `0`-HP floor and
the `/restart_here` twin already proves same-map recovery: prove that same
immediate / delayed floor clear also lets `/restart_town` then succeed at a
fresh destination-map `EXCHANGE START` against a living empire-town peer,
matching the already-owned merchant / MYSHOP / cube / safebox / refine town
twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `GC::EXCHANGE END` after `POINT_CHANGE(0)` → `DEAD` →
   `TARGET(0,0)`, and queue one `GC::EXCHANGE END` to the paired live source-map
   peer.
2. Later `EXCHANGE CANCEL` / display requests stay silent/no-frame;
   inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town position with recovered race create MaxHP, destination visibility
   includes the living town peer, and a fresh requester `EXCHANGE START` against
   that town peer succeeds with ordinary paired start frames instead of operating
   on a stale exchange pairing or busy reject.
4. Spec/QA name the same focused tests as the recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange`

## Explicit non-goals

- inventing a death-specific exchange packet family
- widening exchange mutation beyond the already-owned floor close
- reopening the already-owned `/restart_here` same-map twin

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenExchangeShellBeforeRestartTownExchange$' -count=1
gofmt -w internal/minimal/exchange_practice_mob_floor_restart_town_exchange_test.go
git diff --check
```
