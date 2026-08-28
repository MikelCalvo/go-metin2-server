# Post-floor lab `/open_safebox` `/restart_town` recovery — 2026-08-28

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorOpenSafeboxFailsClosed` already proved the
busy-shell open denial while dead: prove `/restart_here` and `/restart_town`
both rematerialize a fresh lab `/open_safebox` presentation after the
practice-mob retaliation HP floor.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor without an open safebox
   presentation first.
2. Later `/open_safebox` attempts fail closed with:
   - no `SAFEBOX_SIZE`
   - no `SAFEBOX_MONEY_CHANGE`
   - no same-socket safebox busy flag
   - no inventory / gold / persistence mutation
3. After `/restart_here` restores live HP in place, `/open_safebox` emits
   ordinary `SAFEBOX_SIZE` + `SAFEBOX_MONEY_CHANGE` and `/close_safebox` clears
   the presentation with `CloseSafebox`.
4. After `/restart_town` restores live HP at the empire town-return position
   (`map 21`, `52070`, `166600` for empire `2`), the same lab open succeeds and
   town map/coords stay persisted.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorOpenSafeboxFailsClosed`
   - `TestGameSessionFlowPostFloorOpenSafeboxFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins for `/open_safebox`
- widening warehouse password / money mutation recovery (already owned)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorOpenSafeboxFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
