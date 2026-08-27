# Post-floor warehouse `/safebox_password` open fail-closed — 2026-08-27

## Objective

Close the remaining warehouse busy-shell **open** twin after lab `/open_safebox`
already owns post-floor denial: prove a pending `ShowMeSafeboxPassword` challenge
cannot open `SAFEBOX_SIZE` / money / busy while the selected owner is still at
the practice-mob retaliation `0`-HP floor, and that the same floor transition
clears that pending challenge so `/restart_here` recovery cannot reopen the
warehouse from a stale pre-death password attempt.

## Contract frozen by this slice

1. After warehouse `INTERACT` arms a same-socket pending password challenge,
   immediate practice-mob retaliation to owner HP `0` clears that pending
   challenge without inventing a `CloseSafebox` companion (presentation was
   never open).
2. While still at the floor, later `/safebox_password <match>` fails closed with:
   - no `SAFEBOX_SIZE` / `SAFEBOX_SET` / `SAFEBOX_MONEY_CHANGE`
   - no same-socket safebox busy flag
   - no inventory / gold / persistence mutation
3. After `/restart_here` restores live HP, the same password still fails closed
   until a fresh warehouse `INTERACT` re-arms the challenge; then the matching
   password opens normally.
4. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxPasswordOpenFailsClosed`

## Explicit non-goals

- inventing a death-specific safebox packet family
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins for `/open_safebox`
- widening `/safebox_change_password` or money save/withdraw beyond existing
  floor gates

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(OpenSafebox|SafeboxPasswordOpen|OpenCube|MyShopOpen|RefinePreviewOpen|ExchangeStart)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
