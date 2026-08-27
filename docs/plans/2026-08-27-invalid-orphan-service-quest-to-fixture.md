# Invalid Orphan Service Quest-To Fixture — 2026-08-27

## Objective

Close the remaining optional negative dry-run gap after the orphan service
`quest_from` fixture: check in a deterministic fixture for a non-mutating
service definition that authors an orphan `quest_to` without a service quest
gate (`quest_ref` + `quest_flag`), so operators do not improvise that reject
during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-orphan-service-quest-to-bundle.json`
   authors one `talk` service on `npc:orphan_quest_to_guide` with
   `quest_to = 1` but omits `quest_ref` / `quest_flag` (and does not author
   `quest_from`).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (store
   validation already rejects orphan `quest_to` on ungated services; this
   slice binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   orphan-service-`quest_from` / reverse-partial / partial-service-gate
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example gated service `quest_to` mutate)

## Validation

```bash
gofmt -w internal/contentbundle/quest_gate_writer_test.go internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/interactionstore/file_store_test.go
go test ./internal/interactionstore ./internal/contentbundle ./internal/ops -run 'Test(FileStoreRejectsInvalidQuestGatedServiceDefinitions|CanonicalizeRejectsOrphanServiceQuestTo|CanonicalizeRejectsCheckedInOrphanServiceQuestToExample|LocalContentBundleValidateEndpointRejectsOrphanServiceQuestToExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example gated service `quest_to` mutate).~~ Done for
   gated service `quest_to` mutate:
   `docs/examples/bootstrap-invalid-gated-service-quest-to-bundle.json`
   (`docs/plans/2026-08-27-invalid-gated-service-quest-to-fixture.md`).
