# Invalid Duplicate Static Actor Fixture — 2026-08-28

## Objective

Close the remaining optional negative dry-run gap after the unsupported
interaction-kind fixture: check in a deterministic fixture for two portable
static-actor rows that collide after canonical trimming (whitespace around
`name` / `interaction_kind` / `interaction_ref`), so operators do not improvise
that duplicate-row reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-duplicate-static-actor-bundle.json`
   authors two `talk` static actors that share the same authoring key after
   trim, plus one matching `talk` definition so the only fail-closed reason is
   the duplicate portable static-actor row.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects duplicate authoring rows after trim; this slice
   binds the checked-in JSON plus an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing unsupported
   interaction-kind / dangling interaction-ref negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsDuplicateStaticActorAuthoringRows|CanonicalizeRejectsCheckedInDuplicateStaticActorExample|LocalContentBundleValidateEndpointRejectsDuplicateStaticActorExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for item-shaped reward drops that omit bundled
   `item_templates`:
   `docs/examples/bootstrap-invalid-reward-drop-without-item-templates-bundle.json`
   (`docs/plans/2026-08-28-invalid-reward-drop-without-item-templates-fixture.md`).
