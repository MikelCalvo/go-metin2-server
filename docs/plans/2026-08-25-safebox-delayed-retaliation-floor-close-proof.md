# Safebox delayed retaliation floor CloseSafebox proof — 2026-08-25

## Objective

Close the combat-lane proof gap where open `/open_safebox` already emitted
self-only `CHAT_TYPE_COMMAND` `CloseSafebox` on the practice-mob death floor, but
only the immediate hit-piggyback path had focused coverage
(`TestGameSessionFlowPracticeMobDeathClearsOpenSafeboxBusyBeforeRestartExchange`).
MYSHOP / cube already owned both immediate and delayed twins.

## Contract frozen by this slice

1. Delayed server-origin retaliation that reaches owner HP `0` while
   `/open_safebox` is open queues:
   - `PLAYER_POINT_CHANGE(value=0)`
   - `DEAD(owner_vid)`
   - `TARGET(0, 0)`
   - self-only `CloseSafebox`
2. Later `/close_safebox` stays silent; inventory/gold stay unchanged.
3. Visible live peers still receive one queued `DEAD(owner_vid)`.
4. After `/restart_here`, a fresh `EXCHANGE START` against a living visible peer
   succeeds instead of returning the open-safebox busy reject.
5. Spec/QA name the delayed twin beside the existing immediate safebox floor
   proof:
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenSafebox`

## Explicit non-goals

- inventing a death-specific safebox packet family
- durable safebox cell/money mutation on the close companion

The refine-dialog delayed twin later landed separately as
`docs/plans/2026-08-25-refine-delayed-retaliation-floor-close-proof.md`
(`TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenRefine`).

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob(DeathClearsOpenSafeboxBusyBeforeRestartExchange|DelayedRetaliationFloorClosesOpenSafebox)$' -count=1
gofmt -w internal/minimal/player_death_busy_window_test.go
git diff --check
```
