# Invalid Merchant Catalog Count Above Stack Limit Fixture — 2026-08-29

## Objective

Close the next optional negative dry-run gap after the quest-flag
`reward_items` over-stack fixture: check in a deterministic fixture for a
structured `shop_preview` merchant catalog whose entry `count` exceeds the
matching bundled template `max_count`, so operators do not improvise that
over-stack reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject merchant catalog counts above bundled template
  `max_count` via inline Go structs (`TestCanonicalizeRejectsMerchantCatalogCountAboveBundledStackLimit`).
- Manual QA still invents throwaway JSON for the merchant over-stack reject,
  which drifts from the owned authoring examples and from the quest-flag
  reward over-stack / missing-template checked-in negatives.
- The owned PvE vertical includes a structured merchant catalog; a checked-in
  reject fixture keeps the template count-fit contract inspectable beside the
  without-/incomplete-templates twins.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-merchant-catalog-count-above-stack-limit-bundle.json`
   authors one `shop_preview` interaction definition whose catalog references
   `27001` with `count = 11`, plus a matching `item_templates` row with
   `stackable = true` and `max_count = 10`, so the only fail-closed reason is
   catalog count above the bundled template stack limit.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects merchant catalog counts that do not fit the
   template; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing merchant
   missing-template / quest-flag over-stack negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- quest-flag consume / non-stackable count twins unless QA still improvises
  that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsMerchantCatalogCountAboveBundledStackLimit|CanonicalizeRejectsCheckedInMerchantCatalogCountAboveStackLimitExample|LocalContentBundleValidateEndpointRejectsMerchantCatalogCountAboveStackLimitExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example quest-flag consume / non-stackable count
   twins).~~ Done for quest-flag consume count above stack limit:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-count-above-stack-limit-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-consume-item-count-above-stack-limit-fixture.md`).
   Done for merchant catalog count above non-stackable limit:
   `docs/examples/bootstrap-invalid-merchant-catalog-count-above-non-stackable-limit-bundle.json`
   (`docs/plans/2026-08-30-invalid-merchant-catalog-count-above-non-stackable-limit-fixture.md`).
