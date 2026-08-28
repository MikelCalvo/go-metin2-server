# Profile-Authored Chase Delay (`chase_delay_ms`) — 2026-08-28

## Objective

Freeze the next Track A authored combat-profile seam after `aggro_radius` /
`leash_radius`: optional `chase_delay_ms` so registered practice-mob profiles can
widen or narrow the live chase arming / re-arm delay without inventing a second
scheduler or chase packets.

## Why freeze first

1. Live chase arming still hard-codes `bootstrapSpawnGroupChaseStepDelay = 5s`
   in `internal/minimal/factory.go`.
2. Spec already requires that delay to stay longer than the owned `1s` delayed
   retaliation beat so multi-beat hostility remains independently observable.
3. Profile-authored radii already round-trip through `combat_profiles`; chase
   delay is the smallest matching timing extension on that same portable surface.
4. Absolute chase / return / homeward due-at rematerialize across daemon restart
   stays cancelled as re-arm-from-now and must not be reopened by this seam.

## Contract frozen

See `spec/protocol/content-spawn-groups-bootstrap.md` § "First owned
profile-authored chase-delay seam" and the chase-executor pointer in
`spec/protocol/spawn-leash-bootstrap.md`.

Summary:

- JSON `chase_delay_ms` / Go `ChaseDelay` / snapshot `ChaseDelayMs`
- omit/zero → effective `5s`
- positive authored values must be `> 1000` ms and `<= 60000` ms
- `EffectiveStaticActorSpawnChaseDelay(profile)` (+ actor / defaults helpers)
- live arming + post-step re-arm consume the effective delay
- content-bundle / static-actor snapshot round-trip matches radii / respawn_delay

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up RED → GREEN

1. RED pure helpers + registration validation in `internal/worldruntime`.
2. RED live consumer: authored `chase_delay_ms = 2000` arms / re-arms at `2s`
   instead of hard-coded `5s` while still after the `1s` retaliation beat.
3. GREEN: defaults/snapshot/contentbundle/staticstore wiring + factory arming
   consumes `EffectiveStaticActorSpawnChaseDelayForActor`.
4. Keep pack AI / pathfinding / cross-map MOVE / absolute schedule rematerialize
   cancelled for Track A bootstrap.
