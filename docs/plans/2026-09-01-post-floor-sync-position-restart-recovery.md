# Post-floor `SYNC_POSITION` restart recovery — 2026-09-01

## Objective

Close the remaining relocation recovery twin after owner-side `SYNC_POSITION`
already owned quiet post-floor denial: prove `/restart_here` and `/restart_town`
restore a usable live owner so ordinary same-map synchronization succeeds again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Later owner-side `SYNC_POSITION` fails closed with:
   - no self `SYNC_POSITION_ACK`
   - no queued peer synchronization frame
   - no live-coordinate mutation
   - no transfer-trigger rebootstrap burst
3. After `/restart_here` restores live HP, the same owner-side same-map
   `SYNC_POSITION` emits ordinary self `SYNC_POSITION_ACK` beside recovered
   MaxHP (and may queue peer synchronization when the destination stays
   visible).
4. After `/restart_town` restores live HP at the owned empire town position, the
   same same-map `SYNC_POSITION` path likewise succeeds beside recovered MaxHP
   and town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSyncPositionFailsClosed`
   - `TestGameSessionFlowPostFloorSyncPositionFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific synchronization packet family
- widening into static-actor `INTERACT` restart twins in this same commit
- transfer-trigger destination SYNC success gameplay in this twin
- changing already-owned live SYNC_POSITION fanout / transfer-trigger behavior

## Runtime note

A focused probe already observed that `/restart_here` restores ordinary
same-map `SYNC_POSITION` success frames without further production changes.
This slice lands the GREEN proof twins under the names above; no production
runtime change was required.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSyncPositionFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_sync_position_restart_recovery_test.go
git diff --check
```
