# Lab cube close on practice-mob floor — 2026-08-25

## Objective

Prove the already-wired practice-mob retaliation `0`-HP floor closes an open
lab `/open_cube` presentation with self-only `CHAT_TYPE_COMMAND` `cube close`,
matching the MYSHOP / safebox floor-close combat-lane proofs.

## Contract frozen by this slice

1. Immediate hit-piggyback floor and delayed server-origin floor both append
   one self-only `cube close` after `POINT_CHANGE(0)` → `DEAD` → `TARGET(0,0)`.
2. Ordering stays merchant `SHOP END` → MYSHOP empty-sign → `cube close` →
   safebox `CloseSafebox` → exchange `END`.
3. Peer-visible cube busy bit clears; later `/close_cube` stays silent;
   inventory/gold stay unchanged.
4. Spec/QA name the combat-lane proof tests:
   - `TestGameSessionFlowPracticeMobImmediateRetaliationFloorClosesOpenCube`
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenCube`

## Explicit non-goals

- cube recipe make/add/list mutation
- exchange / MYSHOP / safebox / refine cube busy-window rejects (items lane)
- inventing a binary cube packet header

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob.*(Immediate|Delayed)RetaliationFloorClosesOpenCube' -count=1
gofmt -w internal/minimal/cube_practice_mob_floor_close_test.go
git diff --check
```
