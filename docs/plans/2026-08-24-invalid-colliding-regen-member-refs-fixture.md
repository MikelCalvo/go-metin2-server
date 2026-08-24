# Invalid Colliding Regen Member Refs Fixture — 2026-08-24

## Objective

Close the remaining multi-count regen reject dry-run gap: check in a
deterministic fixture where a multi-count `regen_spawns` expansion would
synthesize a member `ref` that already exists in authored `spawn_groups[]`, so
operators do not improvise that reject during `/local/content-bundle/validate`.

Inline `TestCanonicalizeRejectsCollidingMultiCountRegenMemberRefs` already owns
the rule; this slice binds the checked-in JSON and the loopback validate twin.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-colliding-regen-member-refs-bundle.json`
   authors one `spawn_groups[]` row at `practice.qa_colliding_regen_mob.m01`
   plus one `regen_spawns[]` row with `ref = practice.qa_colliding_regen_mob`,
   `count = 2`, and `pack_spacing = 100` (which would synthesize `.m01` / `.m02`).
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture.
3. Loopback `POST /local/content-bundle/validate` returns `400` for that fixture.
4. Spec / QA / roadmap / multi-count regen plans name the fixture beside the
   existing regen count / over-max / one-count-pack-spacing negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCollidingMultiCountRegenMemberRefs|CanonicalizeRejectsCheckedInCollidingRegenMemberRefsExample|LocalContentBundleValidateEndpointRejectsCollidingRegenMemberRefsExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
