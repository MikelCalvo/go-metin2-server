# Invalid Reward Drop Without Item Templates Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the duplicate
static-actor fixture: check in a deterministic fixture for an item-shaped
spawn-group reward drop that omits bundled `item_templates`, so operators do
not improvise that missing-template reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-reward-drop-without-item-templates-bundle.json`
   authors one `spawn_groups` row with non-empty `reward_drop_vnums` and no
   top-level `item_templates` collection, so the only fail-closed reason is
   missing item-template backing for item-shaped reward drops.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects reward drops without bundled templates; this
   slice binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing duplicate
   static-actor / unsupported interaction-kind negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- the related "wrong/partial template present" twin unless QA still improvises
  that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsRewardDropWithoutBundledItemTemplates|CanonicalizeRejectsCheckedInRewardDropWithoutItemTemplatesExample|LocalContentBundleValidateEndpointRejectsRewardDropWithoutItemTemplatesExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example reward drop missing from a present but
   incomplete `item_templates` collection, or merchant catalog without bundled
   templates).
