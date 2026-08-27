# Invalid Quest-State Seed Alone Gate-Writer Fixture — 2026-08-26

## Objective

Close the remaining optional negative dry-run gap after the partial drop-table
kill-quest-credit fixture: check in a deterministic fixture for a gated service
definition that is backed only by portable `quest_state` seed rows (no
`quest_flag` interaction and no kill-quest credit writer), so operators do not
improvise that reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-state-seed-alone-gate-writer-bundle.json`
   authors one gated `talk` service on `quest:first_steps.met_guide = 1` plus one
   portable `quest_state` seed row for the same flag, with no `quest_flag`
   interaction and no kill-quest credit writer.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   orphan-gate / require-gate / partial-credit negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- treating portable `quest_state` seed rows as writers
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsQuestStateSeedAloneAsQuestGateWriter|CanonicalizeRejectsCheckedInQuestStateSeedAloneGateWriterExample|LocalContentBundleValidateEndpointRejectsQuestStateSeedAloneGateWriterExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for partial service quest gate (`quest_ref`
   without `quest_flag`):
   `docs/examples/bootstrap-invalid-partial-service-quest-gate-bundle.json`
   (`docs/plans/2026-08-27-invalid-partial-service-quest-gate-fixture.md`).
