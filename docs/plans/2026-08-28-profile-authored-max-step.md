# Profile-Authored Spawn Max Step (`max_step`) — 2026-08-28

## Objective

Freeze the next Track A authored combat-profile seam after `homeward_delay_ms`:
optional `max_step` so registered practice-mob profiles can widen or narrow the
shared chase / return / homeward step cap without inventing pathfinding, a
second planner family, or per-executor authored step fields.

## Why freeze first

1. Live chase / return / homeward executors still hard-code
   `bootstrapSpawnGroup*MaxStep = 100` in `internal/minimal/factory.go`.
2. Profile-authored chase / return / homeward delays already round-trip through
   `combat_profiles`; step distance is the smallest matching geometry extension
   on that same portable surface.
3. One shared authored step cap keeps chase / return / homeward on the same
   deterministic step-math family already owned by the pure planners.
4. Absolute chase / return / homeward due-at rematerialize across daemon restart
   stays cancelled as re-arm-from-now and must not be reopened by this seam.
5. Cross-map MOVE / `GC WARP` / pack AI stay cancelled / out of scope.

## Contract frozen

See `spec/protocol/content-spawn-groups-bootstrap.md` § "First owned
profile-authored max-step seam" and the chase / return / homeward executor
pointers in `spec/protocol/spawn-leash-bootstrap.md`.

Summary:

- JSON `max_step` / Go `MaxStep` / snapshot `MaxStep`
- omit/zero → effective `100`
- positive authored values must be `>= 1` and `<= 1000`
- `EffectiveStaticActorSpawnMaxStep(profile)` (+ actor / defaults helpers)
- live chase / return / homeward planning + due-step execution consume the
  effective step for the actor's combat profile
- operator `POST .../return-step?max_step=` remains an explicit override when the
  query is present and valid; omitted query uses the effective profile step
- content-bundle / static-actor snapshot round-trip matches delay / radius seams

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up RED → GREEN

1. RED pure helpers + registration validation in `internal/worldruntime`.
2. RED live consumer: authored `max_step = 50` (or `200`) changes due chase /
   return / homeward planned step distance instead of hard-coding `100`.
3. GREEN: defaults/snapshot/contentbundle/staticstore wiring + factory planners
   consume `EffectiveStaticActorSpawnMaxStep`.
4. Keep pack AI / pathfinding / cross-map MOVE / absolute schedule rematerialize
   / delay-authorship changes cancelled for Track A bootstrap.
