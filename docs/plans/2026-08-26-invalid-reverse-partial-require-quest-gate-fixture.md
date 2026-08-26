# Invalid Reverse Partial Require-Quest-Gate Fixture — 2026-08-26

## Objective

Close the remaining optional negative dry-run gap after the partial
`require_quest_ref`-without-flag fixture: check in a deterministic fixture for
a spawn group that authors complete kill-quest credit plus the reverse partial
require gate (`require_quest_flag` without `require_quest_ref`), so operators do
not improvise that reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-reverse-partial-require-quest-gate-bundle.json`
   authors one `spawn_groups[]` row with complete kill-quest credit fields and
   `require_quest_flag = "met_guide"`, but omits `require_quest_ref`
   (and does not author `require_quest_from`).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by the symmetric `validSpawnGroupKillQuestRequireGate` rule; this
   slice binds the checked-in JSON and adds the matching inline twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   kill-quest / orphan-gate / orphan-`require_quest_from` / partial-require-gate
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/kill_quest_credit_test.go internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsReversePartialSpawnGroupKillQuestRequireGate|CanonicalizeRejectsCheckedInReversePartialRequireQuestGateExample|LocalContentBundleValidateEndpointRejectsReversePartialRequireQuestGateExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for partial kill-quest credit (missing
   `reward_quest_text`):
   `docs/examples/bootstrap-invalid-partial-kill-quest-credit-bundle.json`
   (`docs/plans/2026-08-26-invalid-partial-kill-quest-credit-fixture.md`).
