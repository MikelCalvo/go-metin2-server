# Invalid Quest-Flag Reward Item Without Item Templates Fixture — 2026-08-29

## Objective

Close the remaining optional negative dry-run gap after the quest-flag
`reward_items` / `consume_items` incomplete-templates fixtures: check in a
deterministic fixture for a `quest_flag` turn-in whose structured
`reward_items` entry omits the top-level `item_templates` collection entirely,
so operators do not improvise that missing-template-backing reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject quest-flag reward items when the referenced vnum is
  absent from bundled templates via inline Go structs (including the empty
  templates collection case through the same lookup).
- Manual QA still invents throwaway JSON for the turn-in reward without-
  templates reject, which drifts from the owned authoring examples and from the
  merchant / reward-drop / combat-profile without-templates checked-in
  negatives.
- The owned PvE vertical closes kill-quest credit through `QuestHunter`
  `reward_items`; a checked-in reject fixture keeps that turn-in grant contract
  inspectable beside the incomplete present-templates twin.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-flag-reward-item-without-item-templates-bundle.json`
   authors one `quest_flag` interaction definition whose `reward_items` table
   references `27001`, with no top-level `item_templates` collection, so the
   only fail-closed reason is missing item-template backing for the turn-in
   reward.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects quest-flag reward items missing from bundled
   templates; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin and an inline without-templates unit twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing quest-flag
   incomplete-templates / merchant / reward-drop / combat-profile
   without-templates negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- quest-flag `consume_items` without top-level `item_templates` twin unless QA
  still improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsQuestFlagRewardItemsWithoutBundledItemTemplates|CanonicalizeRejectsCheckedInQuestFlagRewardItemWithoutItemTemplatesExample|LocalContentBundleValidateEndpointRejectsQuestFlagRewardItemWithoutItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example `quest_flag` `consume_items` that omit the
   top-level `item_templates` collection entirely).~~ Done for structured
   quest-flag consume items that omit bundled `item_templates`:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-without-item-templates-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-consume-item-without-item-templates-fixture.md`).
   ~~Also: `quest_flag` `reward_items` whose count exceeds the bundled template
   `max_count`.~~ Done for structured quest-flag reward items above stack limit:
   `docs/examples/bootstrap-invalid-quest-flag-reward-item-count-above-stack-limit-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-reward-item-count-above-stack-limit-fixture.md`).
