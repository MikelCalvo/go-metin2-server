# Invalid Open-Safebox Oversize Size Fixture — 2026-09-01

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox` /
`open_cube` foreign-field fixtures: check in a deterministic fixture for an
`open_safebox` definition whose authored page-count `size` is above the owned
bootstrap maximum (`OpenSafeboxSizeMax = 3`), so operators do not improvise that
reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject oversize `size` on `open_safebox` via inline Go
  structs (`TestFileStoreRejectsInvalidOpenSafeboxDefinitions` /
  `"size above max"`).
- Spec language already says invalid authored sizes fail closed at store /
  content-bundle validation, but the checked-in dry-run list only covered
  foreign `title` / `catalog` / warp coords — not the legal-field oversize case.
- Manual QA still invents throwaway JSON (or confuses slash `/open_safebox 4`
  with authored content validation) when confirming warehouse definitions cannot
  author `size = 4`, which drifts from the owned NPC service examples and from
  the other checked-in negatives.
- Oversize `size` is the highest-confusion remaining warehouse field after the
  foreign-field twins: `size` is legal in `0..3` (with `0` defaulting to `1`)
  and illegal at `4+`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-oversize-size-bundle.json`
   authors one `open_safebox` interaction definition with optional informational
   `text` plus illegal `size = 4`, so the only fail-closed reason is the
   oversize page-count field.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   foreign-title / foreign-catalog / foreign-warp-coords / open-cube foreign-*
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (reward/consume gold/experience,
  mutating `quest_to`) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxOversizeSizeExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxOversizeSizeExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
