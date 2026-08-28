# Profile-Authored Return Delay (`return_delay_ms`) — 2026-08-28

## Objective

Land the Track A authored combat-profile seam after `chase_delay_ms`: optional
`return_delay_ms` so registered practice-mob profiles can widen or narrow the
live return-step arming / re-arm delay without inventing a second scheduler or
return packets.

## Status

GREEN. Pure helpers, content-bundle / staticstore wiring, migration `0017`, and
live factory arming / re-arm now consume `EffectiveStaticActorSpawnReturnDelay`.

## Contract

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
- SQL column via migration `0017_static_actor_combat_profile_return_delay`

## Validation

```bash
go test ./internal/worldruntime/ -run 'ReturnDelay|ChaseDelay' -count=1
go test ./internal/contentbundle/ -run 'ReturnDelay|ChaseDelay' -count=1
go test ./internal/minimal/ -run 'AuthoredReturnDelay|AuthoredChaseDelay' -count=1
go test ./db/migrations/ -run 'TestBuiltInCatalogIsValid|TestCatalogSummaryUsesBuiltInCatalog|TestPlan' -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Explicit non-goals kept cancelled

- pack AI / pathfinding / target switching
- cross-map MOVE / `GC WARP`
- absolute chase / return / homeward due-at rematerialize across daemon restart
- homeward-step delay authorship (stays on bootstrap `1s`)
