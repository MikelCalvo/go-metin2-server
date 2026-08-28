# Profile-Authored Homeward Delay (`homeward_delay_ms`) — 2026-08-28

## Objective

Land the Track A authored combat-profile seam after `return_delay_ms`: optional
`homeward_delay_ms` so registered practice-mob profiles can widen or narrow the
live homeward-step arming / re-arm delay without inventing a second scheduler or
homeward packets.

## Status

GREEN. Pure helpers, content-bundle / staticstore wiring, migration `0018`, and
live factory arming / re-arm now consume `EffectiveStaticActorSpawnHomewardDelay`.

## Contract

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
- SQL column via migration `0018_static_actor_combat_profile_homeward_delay`

## Validation

```bash
go test ./internal/worldruntime/ -run 'HomewardDelay|ReturnDelay' -count=1
go test ./internal/contentbundle/ -run 'HomewardDelay|ReturnDelay' -count=1
go test ./internal/minimal/ -run 'AuthoredHomewardDelay|AuthoredReturnDelay' -count=1
go test ./db/migrations/ -run 'TestBuiltInCatalogIsValid|TestCatalogSummaryUsesBuiltInCatalog|TestPlan' -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Explicit non-goals kept cancelled

- pack AI / pathfinding / target switching
- cross-map MOVE / `GC WARP`
- absolute chase / return / homeward due-at rematerialize across daemon restart
- chase-step or return-step delay authorship changes in this slice
