# Invalid Quest-Flag Consume Item Count Above Stack Limit Fixture — 2026-08-29

## Objective

Close the next optional negative dry-run gap after the merchant-catalog and
quest-flag `reward_items` over-stack fixtures: check in a deterministic fixture
for a `quest_flag` turn-in whose structured `consume_items` count exceeds the
matching bundled template `max_count`, so operators do not improvise that
over-stack reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit / canonicalize validation already rejects quest-flag consume item counts
  above bundled template `max_count` via the shared count-fit helper used by
  `reward_items`.
- Manual QA still invents throwaway JSON for the turn-in consume over-stack
  reject, which drifts from the owned authoring examples and from the
  missing-template / reward over-stack checked-in negatives.
- The owned PvE vertical closes kill-quest credit through `QuestHunter`
  `consume_items`; a checked-in reject fixture keeps the template count-fit
  contract inspectable beside the without-/incomplete-templates twins.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-quest-flag-consume-item-count-above-stack-limit-bundle.json`
   authors one `quest_flag` interaction definition whose `consume_items` table
   references `27001` with `count = 11`, plus a matching `item_templates` row
   with `stackable = true` and `max_count = 10`, so the only fail-closed reason
   is consume count above the bundled template stack limit.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects quest-flag consume counts that do not fit the
   template; this slice binds the checked-in JSON plus an explicit
   content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing quest-flag
   missing-template / reward over-stack / merchant over-stack negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize reject rule
- non-stackable count twins unless QA still improvises that JSON later
- further checked-in negatives unless a later reject case still forces QA to
  invent JSON

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInQuestFlagConsumeItemCountAboveStackLimitExample|LocalContentBundleValidateEndpointRejectsQuestFlagConsumeItemCountAboveStackLimitExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON (for example non-stackable count twins).
