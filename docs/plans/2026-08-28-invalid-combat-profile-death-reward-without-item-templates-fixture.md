# Invalid Combat-Profile Death-Reward Without Item Templates Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the spawn-group /
merchant without-templates fixtures: check in a deterministic fixture for a
portable `combat_profiles[].death_reward.drop_vnums` entry that omits bundled
`item_templates`, so operators do not improvise that profile-default
missing-template reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-combat-profile-death-reward-without-item-templates-bundle.json`
   authors one portable `combat_profiles` row with non-empty
   `death_reward.drop_vnums` and no top-level `item_templates` collection, plus
   one spawn group that references that profile without a direct reward
   descriptor, so the only fail-closed reason is missing item-template backing
   for a profile-default death-reward drop.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects combat-profile reward drops without bundled
   templates; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing spawn-group
   reward-drop without-templates / merchant without-templates negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- combat-profile death-reward incomplete-templates twin unless QA still
  improvises that JSON later (combat lane may land that separately)
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCombatProfileRewardDropWithoutBundledItemTemplates|CanonicalizeRejectsCheckedInCombatProfileDeathRewardWithoutItemTemplatesExample|LocalContentBundleValidateEndpointRejectsCombatProfileDeathRewardWithoutItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example combat-profile default death-reward drop
   whose vnum is absent from a present but incomplete `item_templates`
   collection).~~ Done for combat-profile death-reward drops whose vnum is
   absent from a present templates collection:
   `docs/examples/bootstrap-invalid-combat-profile-death-reward-item-missing-from-item-templates-bundle.json`
   (`docs/plans/2026-08-28-invalid-combat-profile-death-reward-item-missing-from-item-templates-fixture.md`).
   ~~Also: `quest_flag` turn-in `reward_items` whose vnum is absent from a
   present but incomplete `item_templates` collection.~~ Done for structured
   quest-flag reward items missing from present templates:
   `docs/examples/bootstrap-invalid-quest-flag-reward-item-missing-from-item-templates-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-reward-item-missing-from-item-templates-fixture.md`).
   ~~Also: `quest_flag` turn-in `consume_items` whose vnum is absent from a
   present but incomplete `item_templates` collection.~~ Done for structured
   quest-flag consume items missing from present templates:
   `docs/examples/bootstrap-invalid-quest-flag-consume-item-missing-from-item-templates-bundle.json`
   (`docs/plans/2026-08-29-invalid-quest-flag-consume-item-missing-from-item-templates-fixture.md`).
