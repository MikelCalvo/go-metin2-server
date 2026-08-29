# Post-floor EXCHANGE START restart recovery — 2026-08-29

## Objective

Close the remaining busy-shell open twin after post-floor `EXCHANGE START`
already owns quiet fail-closed denial: prove `/restart_here` and
`/restart_town` restore a usable live owner so a fresh paired start against a
living visible peer succeeds with ordinary exchange start frames and no
inventory/gold mutation.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob.
2. Later requester `EXCHANGE START` against a living visible same-map peer fails
   closed with no self/peer start frames and no pairing/display state.
3. After `/restart_here`, the same living peer can be freshly started with
   ordinary paired `GC::EXCHANGE START` frames.
4. After `/restart_town`, a living destination-map empire-town peer can likewise
   be freshly started with ordinary paired start frames.
5. Inventory/gold stay unchanged across the deny + restart + fresh-start path.
6. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorExchangeStartFailsClosed`
   - `TestGameSessionFlowPostFloorExchangeStartFailsClosedBeforeRestartTown`
   - `TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosed`
     (living-peer-against-dead-owner deny + `/restart_here` recovery twin; see
     `2026-08-29-post-floor-exchange-dead-partner-restart-recovery.md`)

## Explicit non-goals

- inventing a death-specific exchange packet family
- changing busy-shell rejects for live owners
- reopening already-owned pre-open exchange-shell floor-close / restart twins
- inventing revive menus or broader full action-lock policy

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorExchangeStart' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check -- internal/minimal/player_death_busy_shell_open_guard_test.go \
  spec/protocol/player-death-bootstrap.md docs/qa/manual-client-checklist.md \
  docs/plans/2026-08-29-post-floor-exchange-start-restart-recovery.md
```
