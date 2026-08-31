# Invalid Open-Safebox Foreign Catalog Fixture — 2026-08-31

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox`
foreign-`title` and `open_cube` foreign-`size` / foreign-`title` fixtures: check
in a deterministic fixture for an `open_safebox` definition that illegally
authors a merchant `catalog`, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `catalog` on `open_safebox` via inline Go
  structs (`TestFileStoreRejectsInvalidOpenSafeboxDefinitions` /
  `"catalog not allowed"`).
- Manual QA still invents throwaway JSON when confirming that warehouse
  definitions cannot reuse merchant `catalog`, which drifts from the owned NPC
  service examples and from the other checked-in negatives.
- `catalog` is the highest-confusion remaining foreign field for warehouses
  after `title`: it is legal on `shop_preview` and illegal on `open_safebox`
  (symmetric to the landed warehouse foreign-`title` twin).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-foreign-catalog-bundle.json`
   authors one `open_safebox` interaction definition with optional informational
   `text` plus a single illegal merchant catalog entry, so the only fail-closed
   reason is the foreign catalog field (no bundled `item_templates` needed
   because store validation rejects catalog presence before template binding).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   foreign-title / open-cube foreign-size / open-cube foreign-title /
   unsupported-kind negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (warp coords, reward/consume
  gold/experience, mutating `quest_to`, oversize `size`, open_cube foreign
  catalog) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxForeignCatalogExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxForeignCatalogExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
