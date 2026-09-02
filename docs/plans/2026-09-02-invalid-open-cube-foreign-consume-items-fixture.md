# Invalid Open-Cube Foreign Consume Items Fixture — 2026-09-02

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox`
foreign-`consume_items` fixture: check in a deterministic fixture for an
`open_cube` definition that illegally authors turn-in `consume_items`, so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `consume_items` on non-`quest_flag` kinds via
  store `validDefinition` (`!hasConsumeItems(definition)` required for
  `KindOpenCube`).
- Spec language already says foreign reward/consume fields fail closed at store
  / content-bundle validation, but the checked-in dry-run list only covered
  foreign `size` / `title` / `catalog` / warp coords / `reward_gold` /
  `reward_experience` / `consume_gold` / `consume_experience` / `reward_items`
  for craftsman definitions — not the turn-in consume-items table case.
- Manual QA still invents throwaway JSON (or confuses `/local/interactions`
  non-`quest_flag` `consume_items` rejects with authored craftsman validation)
  when confirming cube definitions cannot author turn-in `consume_items`, which
  drifts from the owned NPC service examples and from the other checked-in
  negatives.
- Foreign `consume_items` is the highest-confusion remaining craftsman turn-in
  table field after foreign `reward_items`: it is legal on `quest_flag`
  turn-ins and illegal on `open_cube` (symmetric to the landed warehouse
  foreign-consume-items twin).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-cube-foreign-consume-items-bundle.json`
   authors one `open_cube` interaction definition with optional informational
   `text` plus illegal `consume_items = [{item_vnum: 27001, count: 1}]`, so the
   only fail-closed reason is the foreign turn-in consume-items table.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-cube
   foreign-size / foreign-title / foreign-catalog / foreign-warp-coords /
   foreign-reward-gold / foreign-reward-experience / foreign-consume-gold /
   foreign-consume-experience / foreign-reward-items / open-safebox
   foreign-consume-items negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- binary cube headers / OR-materials / craft dialog trees
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (scalar reward_item_vnum shorthand,
  mutating `quest_to`) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenCubeForeignConsumeItemsExample|LocalContentBundleValidateEndpointRejectsOpenCubeForeignConsumeItemsExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_safebox` mutating `quest_to`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-quest-to-bundle.json`
   (`docs/plans/2026-09-03-invalid-open-safebox-foreign-quest-to-fixture.md`).
   Highest-value remaining twins are mutating `quest_to` on `open_cube`
   definitions, or scalar `reward_item_vnum` shorthand rejects on
   non-`quest_flag` kinds.
