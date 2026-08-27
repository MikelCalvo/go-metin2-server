# Invalid Dangling Interaction Ref Fixture — 2026-08-27

## Objective

Close the remaining optional negative dry-run gap after the gated service
`quest_to` fixture: check in a deterministic fixture for a static actor that
authors owned interaction metadata (`talk` + canonical `interaction_ref`) while
the bundle's `interaction_definitions` only contain a different ref, so
operators do not improvise that dangling-ref reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-dangling-interaction-ref-bundle.json`
   authors one `talk` static actor on `npc:dangling_guard` plus an unrelated
   `info` definition `lore:alchemist`, so the only fail-closed reason is the
   dangling interaction reference.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects dangling refs; this slice binds the checked-in
   JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing gated
   service `quest_to` / orphan service / unsupported-kind reject notes.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example unsupported future interaction kinds)

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsDanglingInteractionReference|CanonicalizeRejectsCheckedInDanglingInteractionRefExample|LocalContentBundleValidateEndpointRejectsDanglingInteractionRefExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example unsupported future interaction kinds such as
   unfrozen `quest` actor metadata).
