# Post-floor `SAFEBOX_CHECKOUT` restart recovery — 2026-09-01

## Objective

Close the remaining storage-packet recovery twin after packet
`SAFEBOX_CHECKOUT` already owned quiet post-floor denial and anti-safebox
`SAFEBOX_CHECKIN` restart recovery landed: prove `/restart_here` /
`/restart_town` restore a usable live owner that can freshly `/open_safebox`
and complete an ordinary remembered-cell `SAFEBOX_CHECKOUT`.

## Contract frozen by this slice

1. Seed one carried stack, open lab `/open_safebox`, accept `SAFEBOX_CHECKIN`
   into safebox cell `0`, then drive the owner to the retaliation HP floor so
   the floor edge closes the open presentation with `CloseSafebox`.
2. Later packet `SAFEBOX_CHECKOUT` fails closed with no safebox/inventory
   frames and no inventory / gold / persistence mutation while dead.
3. After `/restart_here` restores live HP, `/open_safebox` rematerializes the
   remembered cell and the same `SAFEBOX_CHECKOUT` emits ordinary
   `SAFEBOX_DEL` + inventory `ITEM_SET`, persisting the returned stack beside
   recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + checkout path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxCheckoutFailsClosed`
   - `TestGameSessionFlowPostFloorSafeboxCheckoutFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening into `SAFEBOX_ITEM_MOVE` / mall restart twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxCheckoutFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_safebox_checkout_restart_recovery_test.go
git diff --check
```
