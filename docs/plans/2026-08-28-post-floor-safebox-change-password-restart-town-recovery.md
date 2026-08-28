# Post-floor warehouse `/safebox_change_password` `/restart_town` recovery — 2026-08-28

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosed` already proved
`/restart_here` rematerializes durable warehouse password mutation: prove the
same post-floor deny → recover path also works through `/restart_town` into the
owned empire town-return position.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor without mutating durable warehouse
   password first.
2. Later `/safebox_change_password <old> <new>` attempts fail closed with:
   - no success/wrong-password info chat
   - no durable warehouse-password mutation
   - no inventory / gold / presentation / persistence mutation
3. After `/restart_town` restores live HP at the empire town-return position,
   `/safebox_change_password` succeeds normally, persists the new durable
   password, and keeps the town map/coords.
4. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening TMP4 CG `SAFEBOX_CHANGE_PASSWORD` request ownership (still deferred)
- destination-peer exchange recovery (already owned by open-safebox town twins)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
