# Invalid Merchant Catalog Count Above Non-Stackable Limit Fixture — 2026-08-30

## Objective

Close the next optional negative dry-run gap after the stackable merchant-catalog
over-stack fixture: check in a deterministic fixture for a structured
`shop_preview` merchant catalog whose entry `count` exceeds `1` for a
non-stackable bundled template, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject merchant catalog counts above non-stackable
  `max_count = 1` via inline Go structs
  (`TestCanonicalizeRejectsMerchantCatalogMultipleNonStackableBundledItem`).
- Manual QA still invents throwaway JSON for the non-stackable merchant count
  reject, which drifts from the owned authoring examples and from the stackable
  over-stack / missing-template checked-in negatives.
- The owned PvE vertical sells a non-stackable `Wooden Sword` (`11200`); a
  checked-in reject fixture keeps the non-stackable count-fit contract
  inspectable beside the stackable over-stack twin.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-merchant-catalog-count-above-non-stackable-limit-bundle.json`
   authors one `shop_preview` interaction definition whose catalog references
   `11200` with `count = 2`, plus a matching `item_templates` row with
   `stackable = false` and `max_count = 1`, so the only fail-closed reason is
   catalog count above the non-stackable template limit.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects merchant catalog counts that do not fit the
   template; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing merchant
   stackable over-stack / missing-template negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- quest-flag reward / consume non-stackable count twins unless QA still
  improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsMerchantCatalogMultipleNonStackableBundledItem|CanonicalizeRejectsCheckedInMerchantCatalogCountAboveNonStackableLimitExample|LocalContentBundleValidateEndpointRejectsMerchantCatalogCountAboveNonStackableLimitExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example quest-flag reward / consume non-stackable
   count twins). Done for quest-flag reward item count above non-stackable
   limit:
   `docs/examples/bootstrap-invalid-quest-flag-reward-item-count-above-non-stackable-limit-bundle.json`
   (`docs/plans/2026-08-30-invalid-quest-flag-reward-item-count-above-non-stackable-limit-fixture.md`).
   Done for quest-flag consume item count above non-stackable
   limit:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-count-above-non-stackable-limit-bundle.json`
   (`docs/plans/2026-08-30-invalid-quest-flag-consume-item-count-above-non-stackable-limit-fixture.md`).
