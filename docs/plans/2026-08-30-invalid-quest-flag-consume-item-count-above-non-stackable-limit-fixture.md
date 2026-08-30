# Invalid Quest-Flag Consume Item Count Above Non-Stackable Limit Fixture — 2026-08-30

## Objective

Close the remaining optional negative dry-run gap after the quest-flag reward
non-stackable over-count fixture: check in a deterministic fixture for a
`quest_flag` turn-in whose structured `consume_items` count exceeds `1` for a
non-stackable bundled template, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject quest-flag consume item counts above non-stackable
  `max_count = 1` via the shared count-fit helper used by reward items and
  merchant catalogs.
- Manual QA still invents throwaway JSON for the turn-in consume non-stackable
  count reject, which drifts from the owned authoring examples and from the
  stackable over-stack / missing-template / reward non-stackable checked-in
  negatives.
- The owned PvE vertical already proves non-stackable `Wooden Sword` (`11200`)
  through `QuestHunter` `reward_items`; a checked-in consume twin keeps the
  non-stackable count-fit contract inspectable for debit authoring too.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-flag-consume-item-count-above-non-stackable-limit-bundle.json`
   authors one `quest_flag` interaction definition whose `consume_items` table
   references `11200` with `count = 2`, plus a matching `item_templates` row
   with `stackable = false` and `max_count = 1`, so the only fail-closed reason
   is consume count above the non-stackable template limit.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects quest-flag consume counts that do not fit the
   template; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing quest-flag
   stackable over-stack / missing-template / reward non-stackable negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsQuestFlagConsumeItemCountAboveNonStackableLimit|CanonicalizeRejectsCheckedInQuestFlagConsumeItemCountAboveNonStackableLimitExample|LocalContentBundleValidateEndpointRejectsQuestFlagConsumeItemCountAboveNonStackableLimitExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_cube` foreign warehouse `size`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-size-bundle.json`
   (`docs/plans/2026-08-30-invalid-open-cube-foreign-size-fixture.md`).
