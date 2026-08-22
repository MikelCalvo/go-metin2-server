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

- remapping suppress across content-bundle replacement or daemon restart
- inventing a second permanent suppress store keyed by name/VID
- cross-map return MOVE / warp packet choreography
- broader corpse / revive menus
