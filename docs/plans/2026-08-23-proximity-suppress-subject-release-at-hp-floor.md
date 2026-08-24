# Proximity Suppress Subject Release At HP Floor — 2026-08-23

## Objective

Harden subject-side engagement release so proximity leave/re-enter suppress still
seeds for the releasing owner when that owner's shared-world snapshot is already
at the bootstrap `0`-HP floor.

`seedProximityAggroSuppressForInsideCandidatesLocked` intentionally skips floor
candidates (dead owners should not stay in the bystander suppress set forever).
Death-floor `/restart_here` recovery, however, restores live HP while the owner
may still stand inside aggro radius. Subject clear must therefore mark the
releasing subject explicitly before relying on bystander seed alone.

## Contract frozen by this slice

1. `ClearStaticActorCombatEngagementsBySubject` / `clearStaticActorCombatEngagementsBySubjectLocked`
   always marks the releasing subject for proximity suppress.
2. When the actor still resolves, bystander `seedProximity` still runs for other
   inside live candidates.
3. After the releasing owner's shared-world HP is restored while still inside the
   effective aggro radius, `AcquireProximitySpawnGroupAggro` must not instantly
   re-lock that owner.
4. Explicit leave + re-enter of the effective aggro radius clears suppress and
   allows fresh proximity acquisition.

## Focused coverage

- `TestSharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor`

```bash
go test ./internal/minimal -run 'TestSharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor$' -count=1
```

## What this is not yet

- remapping suppress across daemon restart
- inventing a second permanent suppress store keyed by name/VID
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- broader corpse / revive menus

## Follow-up

- ~~Remap proximity suppress across non-identical same-`spawn_group_ref` content-bundle replacement.~~ Done: see `docs/plans/2026-08-24-proximity-suppress-content-bundle-replacement-remap.md`.
- Keep daemon-restart suppress remapping deferred.
