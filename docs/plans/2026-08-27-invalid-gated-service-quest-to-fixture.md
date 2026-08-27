# Invalid Gated Service Quest-To Fixture — 2026-08-27

## Objective

Close the remaining optional negative dry-run gap after the orphan service
`quest_to` fixture: check in a deterministic fixture for a non-mutating
service definition that authors a complete quest gate (`quest_ref` +
`quest_flag` + `quest_from`) plus a mutating `quest_to`, so operators do not
improvise that reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-gated-service-quest-to-bundle.json`
   authors one gated `talk` service on `npc:gated_quest_to_guide` with
   `quest_ref` / `quest_flag` / `quest_from = 1` / `quest_to = 2`, plus an
   in-bundle `quest_flag` writer for `quest:first_steps.met_guide` so the
   only fail-closed reason is the mutating gate (`quest_to` must remain `0`).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (store
   validation already rejects gated services with `quest_to != 0`; this slice
   binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   orphan-service-`quest_to` / orphan-service-`quest_from` / reverse-partial /
   partial-service-gate negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/quest_gate_writer_test.go internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/interactionstore ./internal/contentbundle ./internal/ops -run 'Test(FileStoreRejectsInvalidQuestGatedServiceDefinitions|CanonicalizeRejectsGatedServiceQuestTo|CanonicalizeRejectsCheckedInGatedServiceQuestToExample|LocalContentBundleValidateEndpointRejectsGatedServiceQuestToExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
