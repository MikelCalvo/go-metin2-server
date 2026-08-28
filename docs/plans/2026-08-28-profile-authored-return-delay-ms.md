# Profile-Authored Return Delay (`return_delay_ms`) — 2026-08-28

## Objective

Freeze the next Track A authored combat-profile seam after `chase_delay_ms`:
optional `return_delay_ms` so registered practice-mob profiles can widen or
narrow the live return-step arming / re-arm delay without inventing a second
scheduler or return packets.

## Why freeze first

1. Live return-step arming still hard-codes `bootstrapSpawnGroupReturnStepDelay = 1s`
   in `internal/minimal/factory.go`.
2. Profile-authored chase delay already round-trips through `combat_profiles`;
   return delay is the smallest matching timing extension on that same portable
   surface for the return-step executor.
3. Homeward-step delay stays on its own bootstrap `1s` seam and must not be
   opened by this freeze.
4. Absolute chase / return / homeward due-at rematerialize across daemon restart
   stays cancelled as re-arm-from-now and must not be reopened by this seam.

## Contract frozen

See `spec/protocol/content-spawn-groups-bootstrap.md` § "First owned
profile-authored return-delay seam" and the return-executor pointer in
`spec/protocol/spawn-leash-bootstrap.md`.

Summary:

- JSON `return_delay_ms` / Go `ReturnDelay` / snapshot `ReturnDelayMs`
- omit/zero → effective `1s`
- positive authored values must be `>= 250` ms and `<= 60000` ms
- `EffectiveStaticActorSpawnReturnDelay(profile)` (+ actor / defaults helpers)
- live return arming + post-step re-arm consume the effective delay
- content-bundle / static-actor snapshot round-trip matches chase_delay / radii

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up RED → GREEN

1. RED pure helpers + registration validation in `internal/worldruntime`.
2. RED live consumer: authored `return_delay_ms = 2000` arms / re-arms at `2s`
   instead of hard-coded `1s`.
3. GREEN: defaults/snapshot/contentbundle/staticstore wiring + factory arming
   consumes `EffectiveStaticActorSpawnReturnDelay`.
4. Keep pack AI / pathfinding / cross-map MOVE / absolute schedule rematerialize
   / homeward-delay authorship cancelled for Track A bootstrap.
