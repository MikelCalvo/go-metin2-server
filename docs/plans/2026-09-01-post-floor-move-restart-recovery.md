# Post-floor `MOVE` restart recovery — 2026-09-01

## Objective

Close the remaining relocation recovery twin after owner-side `MOVE` already
owned quiet post-floor denial: prove `/restart_here` and `/restart_town`
restore a usable live owner so ordinary same-map movement succeeds again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Later owner-side `MOVE` fails closed with:
   - no self `MOVE_ACK`
   - no queued peer relocation frame
   - no live-coordinate mutation
   - no transfer-trigger rebootstrap burst
3. After `/restart_here` restores live HP, the same owner-side same-map `MOVE`
   emits ordinary self `MOVE_ACK` beside recovered MaxHP (and may queue peer
   relocation when the destination stays visible).
4. After `/restart_town` restores live HP at the owned empire town position, the
   same same-map `MOVE` path likewise succeeds beside recovered MaxHP and
   town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorMoveFailsClosed`
   - `TestGameSessionFlowPostFloorMoveFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific movement packet family
- widening into `SYNC_POSITION` / `INTERACT` restart twins in this same commit
- transfer-trigger destination MOVE success gameplay in this twin
- changing already-owned live MOVE fanout / transfer-trigger behavior

## Runtime note

A focused probe already observed that `/restart_here` restores ordinary
same-map `MOVE` success frames without further production changes. This slice
lands the GREEN proof twins under the names above; no production runtime change
was required.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorMoveFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_move_restart_recovery_test.go
git diff --check
```
