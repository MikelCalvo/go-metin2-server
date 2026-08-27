# Invalid Partial Service-Quest-Gate Fixture — 2026-08-27

## Objective

Close the remaining optional negative dry-run gap after the quest-state
seed-alone gate-writer fixture: check in a deterministic fixture for a
non-mutating service definition that authors a partial quest gate
(`quest_ref` without `quest_flag`), so operators do not improvise that
reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-partial-service-quest-gate-bundle.json`
   authors one `talk` service on `npc:gated_guide` with
   `quest_ref = "quest:first_steps"` but omits `quest_flag` (and does not
   author `quest_from` / `quest_to`).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (store
   validation already rejects partial service gates; this slice binds the
   checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   orphan-service-gate / seed-alone / kill-quest require-gate negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example reverse partial service gate `quest_flag` without
  `quest_ref`, or orphan `quest_from` on an ungated service)

## Validation

```bash
gofmt -w internal/contentbundle/quest_gate_writer_test.go internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsPartialServiceQuestGate|CanonicalizeRejectsCheckedInPartialServiceQuestGateExample|LocalContentBundleValidateEndpointRejectsPartialServiceQuestGateExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example reverse partial service gate or orphan
   service `quest_from`).~~ Done for reverse partial service quest gate
   (`quest_flag` without `quest_ref`):
   `docs/examples/bootstrap-invalid-reverse-partial-service-quest-gate-bundle.json`
   (`docs/plans/2026-08-27-invalid-reverse-partial-service-quest-gate-fixture.md`).
