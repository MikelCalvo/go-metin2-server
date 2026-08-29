# Profile-Authored Reaction Delay (`reaction_delay_ms`) — 2026-08-29

## Objective

Land the Track A authored combat-profile seam after `max_step`:
optional `reaction_delay_ms` so registered practice-mob profiles can widen or
narrow the delayed server-origin retaliation arming delay without inventing a
second hostility scheduler, reaction packets, or absolute deadline rematerialize.

## Status

GREEN. Pure helpers, content-bundle / staticstore wiring, migration `0020`, and
live delayed server-origin retaliation arming / re-arm now consume
`EffectiveStaticActorSpawnReactionDelay`. Immediate hit-piggyback retaliation
stays unchanged.

## Contract

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
- SQL column via migration `0020_static_actor_combat_profile_reaction_delay`

## Validation

```bash
go test ./internal/worldruntime/ -run 'ReactionDelay|MaxStep' -count=1
go test ./internal/contentbundle/ -run 'ReactionDelay|MaxStep' -count=1
go test ./internal/minimal/ -run 'AuthoredReactionDelay' -count=1
go test ./db/migrations/ -run 'TestBuiltInCatalogIsValid|TestCatalogSummaryUsesBuiltInCatalog|TestPlan' -count=1
go test ./internal/staticstore/ -run 'ImportSchema|ReactionDelay|MaxStep' -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Explicit non-goals kept cancelled

- pack AI / pathfinding / target switching
- cross-map MOVE / `GC WARP`
- absolute chase / return / homeward / reaction due-at rematerialize across daemon restart
- inventing reaction packets, a second scheduler/goroutine, or stacked delayed beats
- chase / return / homeward / max_step authorship changes in this slice
- changing immediate hit-piggyback retaliation semantics
