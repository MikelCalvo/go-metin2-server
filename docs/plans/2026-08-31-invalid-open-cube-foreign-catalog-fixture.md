# Invalid Open-Cube Foreign Catalog Fixture — 2026-08-31

## Objective

Close the next optional negative dry-run gap after the owned `open_cube`
foreign-`size` / foreign-`title` and `open_safebox` foreign-`title` /
foreign-`catalog` fixtures: check in a deterministic fixture for an `open_cube`
definition that illegally authors a merchant `catalog`, so operators do not
improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `catalog` on `open_cube` via inline Go
  structs (`TestFileStoreRejectsInvalidOpenCubeDefinitions` /
  `"catalog not allowed"`).
- Manual QA still invents throwaway JSON when confirming that craftsman
  definitions cannot reuse merchant `catalog`, which drifts from the owned NPC
  service examples and from the other checked-in negatives.
- `catalog` is the highest-confusion remaining foreign field for craftsmen after
  `size` / `title`: it is legal on `shop_preview` and illegal on `open_cube`
  (symmetric to the landed warehouse foreign-`catalog` twin).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-cube-foreign-catalog-bundle.json`
   authors one `open_cube` interaction definition with optional informational
   `text` plus a single illegal merchant catalog entry, so the only fail-closed
   reason is the foreign catalog field (no bundled `item_templates` needed
   because store validation rejects catalog presence before template binding).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-cube
   foreign-size / foreign-title / open-safebox foreign-title / foreign-catalog /
   unsupported-kind negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- binary cube headers / OR-materials / craft dialog trees
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (warp coords, reward/consume
  gold/experience, mutating `quest_to`, oversize `size`) unless QA still
  improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenCubeForeignCatalogExample|LocalContentBundleValidateEndpointRejectsOpenCubeForeignCatalogExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_safebox` foreign warp coords:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-warp-coords-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-safebox-foreign-warp-coords-fixture.md`).
