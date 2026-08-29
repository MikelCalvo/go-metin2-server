# Profile-Authored Reaction Delay (`reaction_delay_ms`) — 2026-08-29

## Objective

Freeze the next Track A authored combat-profile seam after `max_step`:
optional `reaction_delay_ms` so registered practice-mob profiles can widen or
narrow the delayed server-origin retaliation arming delay without inventing a
second hostility scheduler, reaction packets, or absolute deadline rematerialize.

## Why freeze first

1. Live delayed server-origin retaliation still hard-codes
   `bootstrapPracticeMobServerOriginRetaliationDelay = 1s` in
   `internal/minimal/factory.go`.
2. Profile-authored chase / return / homeward delays and `max_step` already
   round-trip through `combat_profiles`; reaction timing is the smallest matching
   hostility-cadence extension on that same portable surface.
3. The delayed beat is already server-owned (hit-triggered and proximity-armed);
   authorship only widens/narrows the arming delay for that existing one-pending
   beat policy.
4. Absolute chase / return / homeward / reaction due-at rematerialize across
   daemon restart stays cancelled as re-arm-from-now and must not be reopened.
5. Cross-map MOVE / `GC WARP` / pack AI stay cancelled / out of scope.

## Contract frozen

See `spec/protocol/content-spawn-groups-bootstrap.md` § "First owned
profile-authored reaction-delay seam".

Summary:

- JSON `reaction_delay_ms` / Go `ReactionDelay` / snapshot `ReactionDelayMs`
- omit/zero → effective `1s`
- positive authored values must be `>= 250` ms and `<= 60000` ms
- `EffectiveStaticActorSpawnReactionDelay(profile)` (+ actor / defaults helpers)
- live delayed server-origin retaliation arming (accepted-hit and proximity-armed
  paths) consume the effective delay; re-arm of the next beat after a non-floor
  delayed tick also consumes it
- content-bundle / static-actor snapshot round-trip matches delay / radius /
  max_step seams
- live arming stays on bootstrap `1s` until GREEN

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up RED → GREEN

1. RED pure helpers + registration validation in `internal/worldruntime`.
2. RED live consumer: authored `reaction_delay_ms = 2000` arms the delayed
   server-origin beat at 2s instead of hard-coding bootstrap 1s.
3. GREEN: defaults/snapshot/contentbundle/staticstore wiring + factory consumers
   consume `EffectiveStaticActorSpawnReactionDelay`.
4. Keep pack AI / pathfinding / cross-map MOVE / absolute schedule rematerialize /
   chase/return/homeward/max_step authorship changes cancelled for Track A
   bootstrap.
