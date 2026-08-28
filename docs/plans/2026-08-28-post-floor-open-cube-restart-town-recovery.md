# Post-floor lab `/open_cube` `/restart_town` recovery — 2026-08-28

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorOpenCubeFailsClosed` already proved the
busy-shell open denial while dead: prove `/restart_here` and `/restart_town`
both rematerialize a fresh lab `/open_cube` presentation after the
practice-mob retaliation HP floor.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor without an open cube
   presentation first.
2. Later `/open_cube` attempts fail closed with:
   - no `cube open` command chat
   - no peer-visible cube busy bit
   - no inventory / gold / persistence mutation
3. After `/restart_here` restores live HP in place, `/open_cube` emits
   ordinary `cube open 20022` and `/close_cube` clears the presentation with
   `cube close`.
4. After `/restart_town` restores live HP at the empire town-return position
   (`map 21`, `52070`, `166600` for empire `2`), the same lab open succeeds and
   town map/coords stay persisted.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorOpenCubeFailsClosed`
   - `TestGameSessionFlowPostFloorOpenCubeFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific cube packet family
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins for `/open_cube`
- widening MYSHOP / refine / exchange post-floor open recovery (next twins)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorOpenCubeFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
