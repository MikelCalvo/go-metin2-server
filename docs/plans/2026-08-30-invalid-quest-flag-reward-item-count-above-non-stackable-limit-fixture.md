# Invalid Quest-Flag Reward Item Count Above Non-Stackable Limit Fixture — 2026-08-30

## Objective

Close the next optional negative dry-run gap after the merchant-catalog
non-stackable over-count fixture: check in a deterministic fixture for a
`quest_flag` turn-in whose structured `reward_items` count exceeds `1` for a
non-stackable bundled template, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject quest-flag reward item counts above non-stackable
  `max_count = 1` via the shared count-fit helper used by merchant catalogs.
- Manual QA still invents throwaway JSON for the turn-in reward non-stackable
  count reject, which drifts from the owned authoring examples and from the
  stackable over-stack / missing-template / merchant non-stackable checked-in
  negatives.
- The owned PvE vertical grants non-stackable `Wooden Sword` (`11200`) through
  `QuestHunter` `reward_items`; a checked-in reject fixture keeps the
  non-stackable count-fit contract inspectable beside the stackable over-stack
  twin.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-flag-reward-item-count-above-non-stackable-limit-bundle.json`
   authors one `quest_flag` interaction definition whose `reward_items` table
   references `11200` with `count = 2`, plus a matching `item_templates` row
   with `stackable = false` and `max_count = 1`, so the only fail-closed reason
   is reward count above the non-stackable template limit.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects quest-flag reward counts that do not fit the
   template; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing quest-flag
   stackable over-stack / missing-template / merchant non-stackable negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- quest-flag consume non-stackable count twin unless QA still improvises that
  JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsQuestFlagRewardItemCountAboveNonStackableLimit|CanonicalizeRejectsCheckedInQuestFlagRewardItemCountAboveNonStackableLimitExample|LocalContentBundleValidateEndpointRejectsQuestFlagRewardItemCountAboveNonStackableLimitExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON. Done for quest-flag consume item count above non-stackable
   limit:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-count-above-non-stackable-limit-bundle.json`
   (`docs/plans/2026-08-30-invalid-quest-flag-consume-item-count-above-non-stackable-limit-fixture.md`).
