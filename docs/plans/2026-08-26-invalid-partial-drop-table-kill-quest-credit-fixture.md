# Invalid Partial Drop-Table Kill-Quest Credit Fixture — 2026-08-26

## Objective

Close the remaining optional negative dry-run gap after the spawn-group
from-equals-to fixture: check in a deterministic fixture for a `drop_tables`
row that authors incomplete kill-quest credit (missing `reward_quest_text`)
while a spawn group expands that table, so operators do not improvise that
reject during `/local/content-bundle/validate`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-partial-drop-table-kill-quest-credit-bundle.json`
   authors one `drop_tables[]` row with `reward_experience` plus incomplete
   kill-quest credit (`reward_quest_ref` / `reward_quest_flag` /
   `reward_quest_to` without `reward_quest_text`), referenced by one
   `spawn_groups[]` row via `reward_drop_table_ref`.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (already
   owned by inline unit coverage; this slice binds the checked-in JSON).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing
   kill-quest / orphan-gate / require-gate / partial-credit / from-equals-to
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON (for example quest_state seed alone as a gate writer)

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsPartialDropTableKillQuestCredit|CanonicalizeRejectsCheckedInPartialDropTableKillQuestCreditExample|LocalContentBundleValidateEndpointRejectsPartialDropTableKillQuestCreditExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example quest_state seed alone as a gate writer).
