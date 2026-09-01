# Post-floor `CHARACTER_POSITION` restart recovery — 2026-09-01

## Objective

Close the remaining combat-family stance recovery twin after owner-side client
`CHARACTER_POSITION` already owned quiet post-floor denial: prove
`/restart_here` and `/restart_town` restore a usable live owner so the same
stand/sit presentation succeeds normally again for self and currently visible
live peers.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Later owner-side `CHARACTER_POSITION` (accepted stand / ground-sit / chair
   normalized to ground-sit) fails closed with:
   - no self `GC CHARACTER_POSITION`
   - no queued peer stance frame
   - no selected-target, combat cadence, retaliation, point, inventory, or
     persistence side effect
3. After `/restart_here` restores live HP, the same owner-side ground-sit
   request emits one self `GC CHARACTER_POSITION(owner_vid, 4)` and queues the
   same presentation to the living visible peer beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same ground-sit path likewise succeeds against a living destination-map peer
   beside recovered MaxHP and town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorCharacterPositionFailsClosed`
   - `TestGameSessionFlowPostFloorCharacterPositionFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific stance packet family
- battle-mode / persistent stance / change-speed gameplay
- widening into MOVE / SYNC_POSITION / INTERACT restart twins in this same
  commit
- changing already-owned live stand/sit/chair presentation behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorCharacterPositionFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_character_position_restart_recovery_test.go
git diff --check
```
