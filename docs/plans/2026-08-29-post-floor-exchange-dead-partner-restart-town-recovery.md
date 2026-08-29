# Post-floor living-peer EXCHANGE START against dead partner restart_town recovery — 2026-08-29

## Objective

Close the remaining town-return twin after
`TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosed` already
owned quiet fail-closed denial plus `/restart_here` recovery: prove the dead
owner's `/restart_town` restores a usable live partner on the owned empire town
map so a living destination-map peer can freshly `EXCHANGE START` against the
recovered owner with ordinary paired start frames and no inventory/gold
mutation.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob
   while a living same-map peer remains visible and a living destination-map
   empire-town peer is already present.
2. Later living same-map peer `EXCHANGE START` against that still-connected
   zero-HP owner fails closed with no requester/partner start frames and no
   pairing/display state.
3. After the dead owner's `/restart_town`, a living destination-map peer can
   freshly start against the recovered partner with ordinary paired
   `GC::EXCHANGE START` frames.
4. Inventory/gold stay unchanged across the deny + restart + fresh-start path for
   both owner and town peer; the owner persists race create MaxHP plus the owned
   empire town position.
5. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific exchange packet family
- changing busy-shell rejects for live owners
- reopening already-owned owner-side `EXCHANGE START` restart twins
- inventing revive menus or broader full action-lock policy

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorExchangeStartAgainstDeadPartnerFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check -- internal/minimal/player_death_busy_shell_open_guard_test.go \
  spec/protocol/player-death-bootstrap.md docs/qa/manual-client-checklist.md \
  docs/plans/2026-08-29-post-floor-exchange-dead-partner-restart-recovery.md \
  docs/plans/2026-08-29-post-floor-exchange-dead-partner-restart-town-recovery.md
```
