# Profile-Authored Spawn Max Step (`max_step`) — 2026-08-28

## Objective

Land the Track A authored combat-profile seam after `homeward_delay_ms`:
optional `max_step` so registered practice-mob profiles can widen or narrow the
shared chase / return / homeward step cap without inventing pathfinding, a
second planner family, or per-executor authored step fields.

## Status

GREEN. Pure helpers, content-bundle / staticstore wiring, migration `0019`, live
factory planners / due-step execution / pending inspection, and operator
`return-step` omit→effective now consume `EffectiveStaticActorSpawnMaxStep`.

## Contract

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
- SQL column via migration `0019_static_actor_combat_profile_max_step`

## Validation

```bash
go test ./internal/worldruntime/ -run 'MaxStep|HomewardDelay' -count=1
go test ./internal/contentbundle/ -run 'MaxStep|HomewardDelay' -count=1
go test ./internal/minimal/ -run 'AuthoredMaxStep|AuthoredHomewardDelay' -count=1
go test ./db/migrations/ -run 'TestBuiltInCatalogIsValid|TestCatalogSummaryUsesBuiltInCatalog|TestPlan' -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Explicit non-goals kept cancelled

- pack AI / pathfinding / target switching
- cross-map MOVE / `GC WARP`
- absolute chase / return / homeward due-at rematerialize across daemon restart
- inventing per-executor authored step fields
- chase / return / homeward delay authorship changes in this slice
