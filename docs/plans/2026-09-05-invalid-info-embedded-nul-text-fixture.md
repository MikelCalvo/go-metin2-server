# Invalid Info Embedded-NUL Text Fixture — 2026-09-05

## Objective

Close the remaining optional negative dry-run gap after the owned merchant
scalar `reward_item_vnum` fixture: check in a deterministic fixture for an
`info` definition whose `text` contains an embedded JSON `\u0000` byte, so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Store / content-bundle validation already rejects embedded NUL bytes in
  owned client-visible text/title fields (`validDefinitionText` /
  `TestCanonicalizeRejectsEmbeddedNULInteractionDefinitionTextFields`).
- Spec language already names `info.text`, `talk.text`, optional `warp.text`,
  `shop_preview.title`, optional `open_safebox.text`, and optional
  `open_cube.text` as fail-closed NUL fields, but the checked-in dry-run list
  never bound the JSON `\u0000` authoring form operators actually POST.
- Manual QA still invents throwaway JSON for that reject
  (`docs/qa/manual-client-checklist.md` §4.5.10), which drifts from the other
  checked-in negatives.
- `info.text` is the highest-confusion remaining NUL case: it is the ordinary
  self-only chat payload, and a truncated `visible` / hidden suffix would
  otherwise reach compact previews and client chat.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-info-embedded-nul-text-bundle.json`
   authors one `info` interaction definition with required `text` containing
   JSON `\u0000` (`visible\u0000hidden`), so the only fail-closed reason is the
   embedded NUL byte.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   shop-preview foreign-reward-item-vnum / unsupported-kind / dangling-ref
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in NUL twins (`talk.text`, `warp.text`,
  `shop_preview.title`, `open_safebox.text`, `open_cube.text`) unless QA still
  improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInInfoEmbeddedNULTextExample|LocalContentBundleValidateEndpointRejectsInfoEmbeddedNULTextExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON. With the `info.text` JSON `\u0000` twin landed, Track D
   NUL-text dry-run coverage for the highest-confusion chat payload is
   otherwise closed unless a later reject still forces improvised JSON.
