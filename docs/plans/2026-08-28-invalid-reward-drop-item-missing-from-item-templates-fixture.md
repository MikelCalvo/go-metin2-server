# Invalid Reward Drop Item Missing From Item Templates Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the merchant-catalog
item-missing-from-templates fixture: check in a deterministic fixture for an
item-shaped spawn-group reward drop whose `reward_drop_vnums` entry is absent
from a present but incomplete `item_templates` collection, so operators do not
improvise that missing-template-ref reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-reward-drop-item-missing-from-item-templates-bundle.json`
   authors one `spawn_groups` row with non-empty `reward_drop_vnums` that
   references `27002`, plus a top-level `item_templates` collection that only
   covers `27001`, so the only fail-closed reason is a reward drop whose vnum
   is absent from the present templates collection.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects reward drops missing from bundled templates; this
   slice binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing reward-drop
   without item-templates / merchant incomplete-templates negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- combat-profile default death-reward incomplete-templates twin unless QA still
  improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsRewardDropMissingFromBundledItemTemplates|CanonicalizeRejectsCheckedInRewardDropItemMissingFromItemTemplatesExample|LocalContentBundleValidateEndpointRejectsRewardDropItemMissingFromItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example combat-profile default death-reward drop
   missing from a present but incomplete `item_templates` collection).~~ Done
   for combat-profile death-reward drops whose vnum is absent from a present
   templates collection:
   `docs/examples/bootstrap-invalid-combat-profile-death-reward-item-missing-from-item-templates-bundle.json`
   (`docs/plans/2026-08-28-invalid-combat-profile-death-reward-item-missing-from-item-templates-fixture.md`).
   ~~Also: combat-profile default death-reward drop that omits top-level
   `item_templates` entirely.~~ Done for portable
   `combat_profiles[].death_reward.drop_vnums` that omit bundled
   `item_templates`:
   `docs/examples/bootstrap-invalid-combat-profile-death-reward-without-item-templates-bundle.json`
   (`docs/plans/2026-08-28-invalid-combat-profile-death-reward-without-item-templates-fixture.md`).
