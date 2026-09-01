# Invalid Open-Cube Foreign Warp Coords Fixture — 2026-08-31

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox`
foreign-`warp-coords` and `open_cube` foreign-`size` / foreign-`title` /
foreign-`catalog` fixtures: check in a deterministic fixture for an `open_cube`
definition that illegally authors warp coordinates (`map_index` / `x` / `y`), so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign warp coords on `open_cube` via inline Go
  structs (`TestFileStoreRejectsInvalidOpenCubeDefinitions` /
  warp-coords case).
- Manual QA still invents throwaway JSON when confirming that craftsman
  definitions cannot reuse teleporter `map_index` / `x` / `y`, which drifts from
  the owned NPC service examples and from the other checked-in negatives.
- Warp coordinates are the highest-confusion remaining foreign field for
  craftsmen after `size` / `title` / `catalog`: they are legal on `warp` and
  illegal on `open_cube` (symmetric to the landed warehouse foreign-warp twin).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-cube-foreign-warp-coords-bundle.json`
   authors one `open_cube` interaction definition with optional informational
   `text` plus illegal teleporter `map_index` / `x` / `y`, so the only
   fail-closed reason is the foreign warp-coordinate fields.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-cube
   foreign-size / foreign-title / foreign-catalog / open-safebox foreign-title /
   foreign-catalog / foreign-warp-coords / unsupported-kind negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- binary cube headers / OR-materials / craft dialog trees
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (reward/consume gold/experience,
  mutating `quest_to`, oversize `size`) unless QA still improvises that JSON
  later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenCubeForeignWarpCoordsExample|LocalContentBundleValidateEndpointRejectsOpenCubeForeignWarpCoordsExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_safebox` oversize page-count `size`:
   `docs/examples/bootstrap-invalid-open-safebox-oversize-size-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-safebox-oversize-size-fixture.md`).
   Done for `open_safebox` foreign turn-in `reward_gold`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-reward-gold-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-safebox-foreign-reward-gold-fixture.md`).
   Done for `open_cube` foreign turn-in `reward_gold`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-reward-gold-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-cube-foreign-reward-gold-fixture.md`).
   Done for `open_cube` foreign turn-in `reward_experience`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-reward-experience-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-cube-foreign-reward-experience-fixture.md`).
