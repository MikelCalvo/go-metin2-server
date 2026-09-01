# Post-floor accepted `SAFEBOX_CHECKIN` restart recovery — 2026-09-01

## Objective

Close the remaining accepted check-in recovery twin after anti-safebox
`SAFEBOX_CHECKIN` restart recovery and accepted checkout/item-move twins
already landed: prove `/restart_here` / `/restart_town` restore a usable live
owner that can freshly `/open_safebox` and complete an ordinary carried-stack
`SAFEBOX_CHECKIN`.

## Contract frozen by this slice

1. Seed one ordinary carried stack (not `anti_safebox`), drive the owner to the
   retaliation HP floor, then later packet `SAFEBOX_CHECKIN` fails closed with
   no safebox/inventory frames and no inventory / gold / persistence mutation
   while dead.
2. After `/restart_here` restores live HP, `/open_safebox` emits ordinary
   `SAFEBOX_SIZE` + money, and the same `SAFEBOX_CHECKIN` emits ordinary
   `ITEM_DEL` + `SAFEBOX_SET`, persisting empty carried inventory beside
   recovered MaxHP and durable safebox cell `0`.
3. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + check-in path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
4. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxCheckinSucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorSafeboxCheckinSucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening into mall / partial-split move restart twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxCheckinSucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_safebox_checkin_success_restart_recovery_test.go
git diff --check
```
