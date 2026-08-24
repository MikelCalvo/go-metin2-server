# Invalid Conflicting Kill-Quest Credit Fixture — 2026-08-24

## Objective

Close the remaining optional negative dry-run gap after the empty drop-table and
orphan-gate checked-in reject fixtures: check in a deterministic fixture for a
spawn group that already authors kill-quest credit and also expands a
`drop_tables` row that carries kill-quest credit, so operators do not improvise
that reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-conflicting-kill-quest-credit-bundle.json`
   authors one kill-quest-bearing `drop_tables` row plus one referencing
   `spawn_groups[]` row that already authors the same kill-quest credit fields
   and `reward_drop_table_ref`.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / roadmap / prior plan docs name the fixture beside the existing
   empty-drop-table and orphan-quest-gate negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsConflictingSpawnGroupDropTableKillQuestCredit|CanonicalizeRejectsCheckedInConflictingKillQuestCreditExample|LocalContentBundleValidateEndpointRejectsConflictingKillQuestCreditExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
