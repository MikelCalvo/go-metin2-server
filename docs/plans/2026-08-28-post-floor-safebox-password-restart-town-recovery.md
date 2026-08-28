# Post-floor warehouse `/safebox_password` `/restart_town` recovery — 2026-08-28

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosed` already proved
`/restart_here` keeps a stale pre-death pending warehouse challenge fail-closed:
prove the same post-floor deny → recover path also works through `/restart_town`
into the owned empire town-return position, with a fresh destination-map warehouse
`INTERACT` required before open succeeds again.

## Contract frozen by this slice

1. After warehouse `INTERACT` arms a same-socket pending password challenge on the
   source map, immediate practice-mob retaliation to owner HP `0` clears that
   pending challenge without inventing a `CloseSafebox` companion (presentation
   was never open).
2. While still at the floor, later `/safebox_password <match>` fails closed with:
   - no `SAFEBOX_SIZE` / `SAFEBOX_SET` / `SAFEBOX_MONEY_CHANGE`
   - no same-socket safebox busy flag
   - no inventory / gold / persistence mutation
3. After `/restart_town` restores live HP at the empire town-return position
   (`map 21`, `52070`, `166600` for empire `2`), the same password still fails
   closed until a fresh destination-map warehouse `INTERACT` re-arms the
   challenge; then the matching password opens normally and town map/coords stay
   persisted.
4. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins for `/open_safebox`
- widening TMP4 CG `SAFEBOX_PASSWORD` request ownership (still deferred)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
