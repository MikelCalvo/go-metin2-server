# Invalid Merchant Catalog Item Missing From Item Templates Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the merchant-catalog
without item-templates fixture: check in a deterministic fixture for a
structured `shop_preview` merchant catalog whose catalog `item_vnum` is absent
from a present but incomplete `item_templates` collection, so operators do not
improvise that missing-template-ref reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-merchant-catalog-item-missing-from-item-templates-bundle.json`
   authors one `shop_preview` interaction definition with a non-empty
   structured `catalog` that references both `27001` and `11200`, plus a
   top-level `item_templates` collection that only covers `27001`, so the only
   fail-closed reason is a catalog entry whose `item_vnum` is absent from the
   present templates collection.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects merchant catalog refs missing from bundled
   templates; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing merchant
   catalog without item-templates / reward-drop without item-templates
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- the related reward-drop missing-from-present-templates twin unless QA still
  improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsMerchantCatalogRefMissingFromBundledItemTemplates|CanonicalizeRejectsCheckedInMerchantCatalogItemMissingFromItemTemplatesExample|LocalContentBundleValidateEndpointRejectsMerchantCatalogItemMissingFromItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example reward drop missing from a present but
   incomplete `item_templates` collection).
