# Post-floor static-actor `INTERACT` restart recovery — 2026-09-01

## Objective

Close the remaining authored-interaction recovery twin after owner-side
static-actor `INTERACT` already owned quiet post-floor denial: prove
`/restart_here` and `/restart_town` restore a usable live owner so ordinary
talk `INTERACT` succeeds again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Later owner-side static-actor `INTERACT` against a visible talk NPC fails
   closed with:
   - no self chat/info delivery
   - no merchant / warp / transfer companion
   - no live-coordinate mutation
3. After `/restart_here` restores live HP, the same owner-side talk `INTERACT`
   (past the owned static-actor cooldown when needed) emits ordinary self-only
   talk chat delivery beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, a
   destination-map talk `INTERACT` likewise succeeds beside recovered MaxHP and
   town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorInteractFailsClosed`
   - `TestGameSessionFlowPostFloorInteractFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific interaction packet family
- widening into warp-transfer success gameplay in this twin
- changing already-owned live talk / merchant / warp interaction behavior
- revive policy or a broader general post-death action-lock contract

## Runtime note

Existing denial coverage already proves quiet post-floor `INTERACT` failure for
immediate and delayed retaliation floors. A focused probe is expected to show
`/restart_here` / `/restart_town` restore ordinary talk success without further
production changes; this slice lands the GREEN proof twins under the names
above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorInteractFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_interact_restart_recovery_test.go
git diff --check
```
