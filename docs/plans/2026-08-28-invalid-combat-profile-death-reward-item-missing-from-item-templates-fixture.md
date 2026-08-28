# Invalid Combat-Profile Death-Reward Item Missing From Item Templates Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the spawn-group
reward-drop incomplete-templates fixture: check in a deterministic fixture for
a portable `combat_profiles[].death_reward.drop_vnums` entry whose vnum is
absent from a present but incomplete `item_templates` collection, so operators
do not improvise that profile-default missing-template-ref reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-combat-profile-death-reward-item-missing-from-item-templates-bundle.json`
   authors one portable `combat_profiles` row with non-empty
   `death_reward.drop_vnums` that references `27002`, plus a top-level
   `item_templates` collection that only covers `27001`, and one spawn group
   that references that profile without a direct reward descriptor, so the only
   fail-closed reason is a profile-default death-reward drop whose vnum is
   absent from the present templates collection.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects combat-profile reward drops missing from bundled
   templates; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing spawn-group
   reward-drop incomplete-templates / merchant incomplete-templates negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- combat-profile death-reward without-templates twin unless QA still improvises
  that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCombatProfileRewardDropMissingFromBundledItemTemplates|CanonicalizeRejectsCheckedInCombatProfileDeathRewardItemMissingFromItemTemplatesExample|LocalContentBundleValidateEndpointRejectsCombatProfileDeathRewardItemMissingFromItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example combat-profile default death-reward drop that
   omits top-level `item_templates` entirely).
