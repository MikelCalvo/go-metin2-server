# Profile-Authored Homeward Delay (`homeward_delay_ms`) — 2026-08-28

## Objective

Freeze the next Track A authored combat-profile seam after `return_delay_ms`:
optional `homeward_delay_ms` so registered practice-mob profiles can widen or
narrow the live homeward-step arming / re-arm delay without inventing a second
scheduler or homeward packets.

## Why freeze first

1. Live homeward-step arming still hard-codes `bootstrapSpawnGroupHomewardStepDelay = 1s`
   in `internal/minimal/factory.go`.
2. Profile-authored return delay already round-trips through `combat_profiles`;
   homeward delay is the smallest matching timing extension on that same portable
   surface for the homeward-step executor.
3. Chase-step and return-step delays stay on their already-owned seams and must
   not be reopened by this freeze.
4. Absolute chase / return / homeward due-at rematerialize across daemon restart
   stays cancelled as re-arm-from-now and must not be reopened by this seam.

## Contract frozen

See `spec/protocol/content-spawn-groups-bootstrap.md` § "First owned
profile-authored homeward-delay seam" and the homeward-executor pointer in
`spec/protocol/spawn-leash-bootstrap.md`.

Summary:

- JSON `homeward_delay_ms` / Go `HomewardDelay` / snapshot `HomewardDelayMs`
- omit/zero → effective `1s`
- positive authored values must be `>= 250` ms and `<= 60000` ms
- `EffectiveStaticActorSpawnHomewardDelay(profile)` (+ actor / defaults helpers)
- live homeward arming + post-step re-arm consume the effective delay
- content-bundle / static-actor snapshot round-trip matches return_delay / chase_delay / radii

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up RED → GREEN

1. RED pure helpers + registration validation in `internal/worldruntime`.
2. RED live consumer: authored `homeward_delay_ms = 2000` arms / re-arms at `2s`
   instead of hard-coded `1s`.
3. GREEN: defaults/snapshot/contentbundle/staticstore wiring + factory arming
   consumes `EffectiveStaticActorSpawnHomewardDelay`.
4. Keep pack AI / pathfinding / cross-map MOVE / absolute schedule rematerialize
   / chase-delay or return-delay authorship changes cancelled for Track A bootstrap.
