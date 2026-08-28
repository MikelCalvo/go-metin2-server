# Profile-Authored Chase Delay (`chase_delay_ms`) — 2026-08-28

## Objective

Land the Track A authored combat-profile seam after `aggro_radius` /
`leash_radius`: optional `chase_delay_ms` so registered practice-mob profiles can
widen or narrow the live chase arming / re-arm delay without inventing a second
scheduler or chase packets.

## Status

GREEN. Pure helpers, content-bundle / staticstore wiring, migration `0016`, and
live factory arming / re-arm now consume `EffectiveStaticActorSpawnChaseDelay`.

## Contract

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
- SQL column via migration `0016_static_actor_combat_profile_chase_delay`

## Validation

```bash
go test ./internal/worldruntime/ -run 'ChaseDelay|AggroRadius|LeashRadius' -count=1
go test ./internal/contentbundle/ -run 'ChaseDelay|AggroRadius|LeashRadius' -count=1
go test ./internal/minimal/ -run 'AuthoredChaseDelay|ProximityArmedSpawnGroupChase|HitArmedSpawnGroupChase' -count=1
go test ./db/migrations/ -run 'TestBuiltInCatalogIsValid|TestCatalogSummaryUsesBuiltInCatalog|TestPlan' -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Explicit non-goals kept cancelled

- pack AI / pathfinding / target switching
- cross-map MOVE / `GC WARP`
- absolute chase / return / homeward due-at rematerialize across daemon restart
