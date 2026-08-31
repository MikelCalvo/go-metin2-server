# Invalid Open-Cube Foreign Size Fixture — 2026-08-30

## Objective

Close the next optional negative dry-run gap after the owned `open_cube` NPC
service / route-summary / PvE fixture landings: check in a deterministic fixture
for an `open_cube` definition that illegally authors warehouse `size`, so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `size` on `open_cube` via inline Go structs
  (`TestFileStoreRejectsInvalidOpenCubeDefinitions` / `"size not allowed"`).
- Manual QA still invents throwaway JSON when confirming that craftsman
  definitions cannot reuse `open_safebox` page-count `size`, which drifts from
  the owned NPC service examples and from the other checked-in negatives.
- `size` is the highest-confusion foreign field because it is legal on
  `open_safebox` and illegal on `open_cube`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-cube-foreign-size-bundle.json`
   authors one `open_cube` interaction definition with optional informational
   `text` plus illegal `size = 1`, so the only fail-closed reason is the foreign
   warehouse page-count field.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   unsupported-kind / dangling-ref / gated-service negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- binary cube headers / OR-materials / craft dialog trees
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (`title`, catalog, warp coords,
  reward/consume gold/experience, mutating `quest_to`) unless QA still
  improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenCubeForeignSizeExample|LocalContentBundleValidateEndpointRejectsOpenCubeForeignSizeExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_safebox` foreign merchant `title`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-title-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-safebox-foreign-title-fixture.md`).
   Also done for `open_cube` foreign merchant `title`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-title-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-cube-foreign-title-fixture.md`).
   Also done for `open_safebox` foreign merchant `catalog`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-catalog-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-safebox-foreign-catalog-fixture.md`).
   Also done for `open_cube` foreign merchant `catalog`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-catalog-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-cube-foreign-catalog-fixture.md`).
