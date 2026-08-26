# Lab cube practice-mob floor post-`/restart_town` exchange recovery — 2026-08-26

## Objective

Close the remaining combat-lane proof gap after lab cube floor close already
lets `/restart_here` succeed at `EXCHANGE START`: prove that same immediate /
delayed practice-mob `0`-HP floor clear also lets `/restart_town` succeed at
destination-map `EXCHANGE START` against a living town peer, matching the
already-owned MYSHOP town recovery twin.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `cube close` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`.
2. Later `/close_cube` stays silent; inventory/gold stay unchanged.
3. After `/restart_town` from that same floor, the owner rebuilds at the owned
   empire town-return position and a fresh requester `EXCHANGE START` against a
   living destination-map peer succeeds with ordinary paired start frames
   instead of the open-cube busy info-chat reject.
4. Spec/QA name the same focused tests as the town recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenCubeBeforeRestartTownExchange`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenCubeBeforeRestartTownExchange`

## Explicit non-goals

- cube recipe make/add/list / `r_info` mutation
- inventing a binary cube packet header
- widening safebox / refine floor proofs to `/restart_town` in this slice

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenCube(BeforeRestartTownExchange)?$' -count=1
gofmt -w internal/minimal/cube_practice_mob_floor_close_test.go
git diff --check
```
