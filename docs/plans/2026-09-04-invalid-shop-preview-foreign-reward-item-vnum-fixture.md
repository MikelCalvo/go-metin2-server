# Invalid Shop-Preview Foreign Reward-Item-Vnum Fixture — 2026-09-04

## Objective

Close the next optional negative dry-run gap after the owned warehouse/craftsman
scalar `reward_item_vnum` fixtures: check in a deterministic fixture for a
`shop_preview` definition that illegally authors scalar `reward_item_vnum` /
`reward_item_count` shorthand beside an otherwise valid merchant catalog, so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit / store validation already rejects reward items on non-`quest_flag` kinds:
  `ValidDefinition` normalizes scalar `reward_item_vnum` / `reward_item_count`
  into `reward_items` first, then `KindShopPreview` requires
  `!hasRewardItems(definition)`.
- Spec language already says foreign reward/consume fields fail closed at store /
  content-bundle validation for owned NPC service kinds, but the checked-in
  dry-run list only covered warehouse / craftsman scalar shorthand — not the
  merchant catalog family authors commonly copy turn-in fields onto.
- Manual QA still invents throwaway JSON (or confuses the landed
  `merchant-catalog-*` / warehouse-craftsman `foreign-reward-item-vnum`
  fixtures with merchant scalar shorthand) when confirming `shop_preview`
  definitions cannot author `reward_item_vnum`, which drifts from the owned NPC
  service examples and from the other checked-in negatives.
- Scalar `reward_item_vnum` is the highest-confusion remaining merchant turn-in
  field after warehouse / craftsman twins landed: it is legal on `quest_flag`
  turn-ins and illegal on gated/ungated `shop_preview`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-shop-preview-foreign-reward-item-vnum-bundle.json`
   authors one `shop_preview` interaction definition with required `title` +
   valid one-entry `catalog` + matching `item_templates`, plus illegal
   `reward_item_vnum = 27001` / `reward_item_count = 1`, so the only fail-closed
   reason is the foreign scalar reward-item shorthand.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   / open-cube foreign-reward-item-vnum / merchant-catalog / gated-service-quest-to
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (`shop_preview` table
  `reward_items`, `warp` scalar shorthand twins) unless QA still improvises that
  JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInShopPreviewForeignRewardItemVnumExample|LocalContentBundleValidateEndpointRejectsShopPreviewForeignRewardItemVnumExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON. With warehouse + craftsman + merchant scalar
   `reward_item_vnum` shorthand twins landed, Track D foreign-field dry-run
   coverage for the highest-confusion turn-in shorthand is otherwise closed
   unless a new reject still forces improvised JSON.
