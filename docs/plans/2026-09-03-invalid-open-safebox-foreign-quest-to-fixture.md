# Invalid Open-Safebox Foreign Quest-To Fixture — 2026-09-03

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox` /
`open_cube` foreign turn-in field fixtures: check in a deterministic fixture for
an `open_safebox` definition that illegally authors mutating `quest_to`, so
operators do not improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject mutating `quest_to` on gated non-`quest_flag` kinds
  via store `validOptionalServiceQuestGate` (`QuestTo` must remain `0` for
  `KindOpenSafebox`).
- Spec language already says foreign fields including mutating `quest_to` fail
  closed at store / content-bundle validation, but the checked-in dry-run list
  only covered oversize `size` plus foreign `title` / `catalog` / warp coords /
  reward/consume scalars and item tables — not the warehouse gated mutate case.
- Manual QA still invents throwaway JSON (or confuses the existing gated
  `talk` `quest_to` reject fixture with warehouse validation) when confirming
  warehouse definitions cannot author `quest_to`, which drifts from the owned
  NPC service examples and from the other checked-in negatives.
- Mutating `quest_to` is the highest-confusion remaining warehouse gate field:
  it is legal on `quest_flag` turn-ins and illegal on gated `open_safebox`
  (symmetric to the landed gated-service `talk` twin, but for the warehouse
  service kind).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-foreign-quest-to-bundle.json`
   authors one gated `open_safebox` interaction definition with optional
   informational `text`, complete `quest_ref` / `quest_flag` / `quest_from`,
   plus illegal `quest_to = 2`, and includes an in-bundle `quest_flag` writer
   for `quest:first_steps.met_guide` so the only fail-closed reason is the
   mutating warehouse `quest_to` rule.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   oversize-size / foreign-* / open-cube foreign-* / gated-service-quest-to
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (cube `quest_to`, scalar
  `reward_item_vnum` shorthand) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxForeignQuestToExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxForeignQuestToExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_cube` mutating `quest_to`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-quest-to-bundle.json`
   (`docs/plans/2026-09-03-invalid-open-cube-foreign-quest-to-fixture.md`).
   Highest-value remaining twin is scalar `reward_item_vnum` shorthand rejects
   on non-`quest_flag` kinds.
