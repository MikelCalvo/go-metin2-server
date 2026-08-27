# Invalid Unsupported Interaction Kind Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the dangling
interaction-ref fixture: check in a deterministic fixture for a static actor
that authors unfrozen `quest` interaction metadata (plus an unrelated owned
`info` definition that happens to share the same `ref`), so operators do not
improvise that unsupported-kind reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-unsupported-interaction-kind-bundle.json`
   authors one static actor on `interaction_kind = "quest"` /
   `interaction_ref = "quest:first_steps"` plus an `info` definition with the
   same ref, so the only fail-closed reason is the unsupported actor kind (a
   same-ref owned definition must not make unfrozen quest/dialog metadata
   importable).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects unsupported static-actor interaction kinds; this
   slice binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing dangling
   interaction-ref / gated service `quest_to` negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds or a real `quest` / dialog interaction family
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example duplicate authored static-actor rows)

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsStaticActorUnsupportedInteractionKinds|CanonicalizeRejectsCheckedInUnsupportedInteractionKindExample|LocalContentBundleValidateEndpointRejectsUnsupportedInteractionKindExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example duplicate authored static-actor rows after
   canonical trimming).
