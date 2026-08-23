# Invalid One-Count Regen Pack Spacing Fixture — 2026-08-23

## Objective

Close the remaining optional negative dry-run gap after multi-count regen pack
placement: check in a deterministic fixture for `count = 1` with positive
`pack_spacing` so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json`
   authors one `regen_spawns` row with `count = 1` and `pack_spacing = 100`.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / roadmap / prior plan docs name the fixture beside the existing
   multi-count-without-spacing and over-max-count negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- new NPC service kinds
- weighted/random loot or branching quest scripts
- changing the already-owned canonicalize reject rule

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsOneCountRegenSpawnWithPackSpacing|CanonicalizeRejectsCheckedInOneCountRegenWithPackSpacingExample|LocalContentBundleValidateEndpointRejectsOneCountRegenWithPackSpacingExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
