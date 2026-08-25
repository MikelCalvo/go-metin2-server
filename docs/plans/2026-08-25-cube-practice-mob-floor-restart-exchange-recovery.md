# Lab cube practice-mob floor post-`/restart_here` exchange recovery — 2026-08-25

## Objective

Close the remaining combat-lane proof gap after lab cube floor close already
emits `cube close`: prove that same immediate / delayed practice-mob `0`-HP
floor clear also lets `/restart_here` then succeed at `EXCHANGE START` against
a living peer, matching the already-owned safebox / refine recovery twins.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor still append
   one self-only `cube close` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`.
2. Later `/close_cube` stays silent; inventory/gold stay unchanged.
3. After `/restart_here` from that same floor, a fresh requester `EXCHANGE START`
   against a living visible peer succeeds with ordinary paired start frames
   instead of the open-cube busy info-chat reject.
4. Spec/QA name the same focused tests as the recovery twin:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenCube`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenCube`

## Explicit non-goals

- cube recipe make/add/list / `r_info` mutation
- inventing a binary cube packet header
- widening MYSHOP / safebox / refine floor proofs beyond their existing twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenCube' -count=1
gofmt -w internal/minimal/cube_practice_mob_floor_close_test.go
git diff --check
```
