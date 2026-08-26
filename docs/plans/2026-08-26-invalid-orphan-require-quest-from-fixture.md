# Invalid Orphan Require-Quest-From Fixture — 2026-08-26

## Objective

Close the remaining optional negative dry-run gap after the orphan-writer,
empty-drop-table, conflicting-kill-quest, and colliding-regen checked-in reject
fixtures: check in a deterministic fixture for a spawn group that authors full
kill-quest credit plus an orphan `require_quest_from` without both require
identities, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-orphan-require-quest-from-bundle.json`
   authors one `spawn_groups[]` row with complete kill-quest credit fields and
   `require_quest_from = 1`, but omits both `require_quest_ref` and
   `require_quest_flag`.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   kill-quest / orphan-gate negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- a second partial-require (`require_quest_ref` without `require_quest_flag`)
  fixture unless QA still invents that JSON after this slice

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsOrphanRequireQuestFromWithoutGate|CanonicalizeRejectsCheckedInOrphanRequireQuestFromExample|LocalContentBundleValidateEndpointRejectsOrphanRequireQuestFromExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example a partial require-gate twin).~~ Done for
   partial kill-quest require gate (`require_quest_ref` without
   `require_quest_flag`):
   `docs/examples/bootstrap-invalid-partial-require-quest-gate-bundle.json`
   (`docs/plans/2026-08-26-invalid-partial-require-quest-gate-fixture.md`).
