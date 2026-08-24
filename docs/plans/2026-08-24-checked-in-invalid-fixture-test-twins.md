# Checked-In Invalid Fixture Test Twins — 2026-08-24

## Objective

Close the remaining incomplete automated bindings for already-checked-in
negative content-bundle fixtures so every preferred `/local/content-bundle/validate`
dry-run JSON is locked by both `Canonicalize(...)` and the loopback validate
endpoint.

## Gaps closed by this slice

1. `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json` already had:
   - inline `TestCanonicalizeRejectsOverMaxRegenSpawnCount`
   - ops `TestLocalContentBundleValidateEndpointRejectsOverMaxRegenCountExample`
   - but no disk-backed `TestCanonicalizeRejectsCheckedInOverMaxRegenCountExample`
2. `docs/examples/bootstrap-invalid-orphan-service-quest-gate-bundle.json` already had:
   - disk-backed `TestCanonicalizeRejectsCheckedInOrphanServiceQuestGateExample`
   - but no ops `TestLocalContentBundleValidateEndpointRejectsOrphanServiceQuestGateExample`

## Contract owned by this slice

1. `Canonicalize(...)` returns `ErrInvalidBundle` when reading the checked-in
   over-max regen fixture from disk.
2. Loopback `POST /local/content-bundle/validate` returns `400` for the checked-in
   orphan service-gate fixture.
3. No production code or fixture JSON changes; this only completes the twin
   coverage already promised by the earlier checked-in invalid-fixture plans.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- hermetic MemoryStore refactors in unrelated gameplay suites

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsOverMaxRegenSpawnCount|CanonicalizeRejectsCheckedInOverMaxRegenCountExample|LocalContentBundleValidateEndpointRejectsOverMaxRegenCountExample|CanonicalizeRejectsCheckedInOrphanServiceQuestGateExample|LocalContentBundleValidateEndpointRejectsOrphanServiceQuestGateExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Prefer hermetic MemoryStore conversion for remaining FileStore-backed content
   gameplay suites only when a later flake or CI cost forces it.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
