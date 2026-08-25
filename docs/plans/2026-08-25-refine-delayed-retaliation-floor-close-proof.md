# Refine delayed retaliation floor busy-clear proof — 2026-08-25

## Objective

Close the combat-lane proof gap where an open refine-dialog preview already
cleared silently on the practice-mob death floor, but only the immediate
hit-piggyback path had focused coverage
(`TestGameSessionFlowPracticeMobDeathClearsOpenRefineBusyBeforeRestartExchange`).
Safebox / MYSHOP / cube already owned both immediate and delayed twins.

## Contract frozen by this slice

1. Delayed server-origin retaliation that reaches owner HP `0` while a refine
   preview is open queues only:
   - `PLAYER_POINT_CHANGE(value=0)`
   - `DEAD(owner_vid)`
   - `TARGET(0, 0)`
   with no synthetic refine cancel/result frame.
2. Later `REFINE type = 255` cancel stays silent; inventory/gold stay unchanged.
3. Visible live peers still receive one queued `DEAD(owner_vid)`.
4. After `/restart_here`, a fresh `EXCHANGE START` against a living visible peer
   succeeds instead of returning the open-refine busy reject.
5. Spec/QA name the delayed twin beside the existing immediate refine floor
   proof:
   - `TestGameSessionFlowPracticeMobDelayedRetaliationFloorClosesOpenRefine`

## Explicit non-goals

- inventing a death-specific refine packet family
- inventory / gold / material mutation on the silent busy clear
- broadening post-floor refine packet denial beyond the already-owned fail-closed path

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMob(DeathClearsOpenRefineBusyBeforeRestartExchange|DelayedRetaliationFloorClosesOpenRefine)$' -count=1
gofmt -w internal/minimal/player_death_busy_window_test.go
git diff --check
```
