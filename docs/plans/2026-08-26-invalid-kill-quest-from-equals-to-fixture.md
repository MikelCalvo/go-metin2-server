# Invalid Kill-Quest From-Equals-To Fixture — 2026-08-26

## Objective

Close the remaining optional negative dry-run gap after the partial
kill-quest-credit fixture: check in a deterministic fixture for a spawn group
that authors complete kill-quest credit with `reward_quest_from ==
reward_quest_to`, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-kill-quest-from-equals-to-bundle.json`
   authors one `spawn_groups[]` row with complete kill-quest credit fields
   where `reward_quest_from = 1` and `reward_quest_to = 1` (no-op transition).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   kill-quest / orphan-gate / require-gate / partial-credit negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example partial drop-table kill-quest credit)

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsKillQuestCreditWithFromEqualsTo|CanonicalizeRejectsCheckedInKillQuestFromEqualsToExample|LocalContentBundleValidateEndpointRejectsKillQuestFromEqualsToExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for partial drop-table kill-quest credit:
   `docs/examples/bootstrap-invalid-partial-drop-table-kill-quest-credit-bundle.json`
   (`docs/plans/2026-08-26-invalid-partial-drop-table-kill-quest-credit-fixture.md`).
