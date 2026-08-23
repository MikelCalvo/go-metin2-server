# Invalid Empty Drop Table Fixture — 2026-08-23

## Objective

Close the remaining optional negative dry-run gap after the regen/orphan
checked-in reject fixtures: check in a deterministic fixture for a completely
empty `drop_tables` row (no EXP/gold/drop channels and no kill-quest credit) so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-empty-drop-table-bundle.json` authors one
   `drop_tables` row with only `ref`, plus one referencing
   `spawn_groups[].reward_drop_table_ref`.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / roadmap / prior plan docs name the fixture beside the existing
   regen and orphan-quest-gate negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsEmptyDropTableWithoutKillQuestCredit|CanonicalizeRejectsCheckedInEmptyDropTableExample|LocalContentBundleValidateEndpointRejectsEmptyDropTableExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
