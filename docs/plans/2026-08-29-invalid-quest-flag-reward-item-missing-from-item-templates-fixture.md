# Invalid Quest-Flag Reward Item Missing From Item Templates Fixture — 2026-08-29

## Objective

Close the remaining optional negative dry-run gap after the merchant-catalog /
reward-drop / combat-profile incomplete-templates fixtures: check in a
deterministic fixture for a `quest_flag` turn-in whose structured
`reward_items` entry is absent from a present but incomplete `item_templates`
collection, so operators do not improvise that missing-template-ref reject
during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject quest-flag reward items missing from bundled
  templates via inline Go structs.
- Manual QA still invents throwaway JSON for the turn-in incomplete-templates
  reject, which drifts from the owned authoring examples and from the
  merchant/reward-drop checked-in negatives.
- The owned PvE vertical closes kill-quest credit through `QuestHunter`
  `reward_items`; a checked-in reject fixture keeps that turn-in contract
  inspectable beside the positive NPC service bundle.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-flag-reward-item-missing-from-item-templates-bundle.json`
   authors one `quest_flag` interaction definition whose `reward_items` table
   references both `27001` and `11200`, plus a top-level `item_templates`
   collection that only covers `27001`, so the only fail-closed reason is a
   turn-in reward whose `item_vnum` is absent from the present templates
   collection.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects quest-flag reward items missing from bundled
   templates; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing merchant /
   reward-drop / combat-profile incomplete-templates negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- quest-flag `consume_items` incomplete-templates twin unless QA still
  improvises that JSON later
- quest-flag reward without top-level `item_templates` twin unless QA still
  improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsQuestFlagRewardItemsMissingFromBundledItemTemplates|CanonicalizeRejectsCheckedInQuestFlagRewardItemMissingFromItemTemplatesExample|LocalContentBundleValidateEndpointRejectsQuestFlagRewardItemMissingFromItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example `quest_flag` `consume_items` whose vnum is
   absent from a present but incomplete `item_templates` collection).~~ Done for
   structured quest-flag consume items missing from present templates:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-missing-from-item-templates-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-consume-item-missing-from-item-templates-fixture.md`).
