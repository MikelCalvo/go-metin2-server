# Safebox practice-mob floor post-`/restart_town` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after safebox floor close already
lets `/restart_here` succeed at `EXCHANGE START`: prove that same immediate /
delayed practice-mob `0`-HP floor clear also lets `/restart_town` succeed at
destination-map `EXCHANGE START` against a living town peer, matching the
already-owned MYSHOP / cube town recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `CloseSafebox` command chat after `POINT_CHANGE(0)` → `DEAD` →
   `TARGET(0,0)`.
2. Later `/close_safebox` stays silent; inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town-return position and a fresh requester `EXCHANGE START` against a
   living destination-map peer succeeds with ordinary paired start frames
   instead of the open-safebox busy info-chat reject.
4. Spec/QA name the same focused tests as the town recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenSafeboxBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenSafeboxBeforeRestartTownExchange`

## Explicit non-goals

- safebox password / mall / money mutation beyond existing presentation
- inventing a death-specific safebox packet family
- widening refine floor proofs to `/restart_town` in this slice

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenSafebox(BeforeRestartTownExchange)?$|TestGameSessionFlowPracticeMobDeathClearsOpenSafeboxBusyBeforeRestartExchange' -count=1
gofmt -w internal/minimal/safebox_practice_mob_floor_restart_town_exchange_test.go
git diff --check
```
