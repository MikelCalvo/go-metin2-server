# Invalid Open-Cube Foreign Quest-To Fixture — 2026-09-03

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox`
foreign-`quest_to` fixture: check in a deterministic fixture for an `open_cube`
definition that illegally authors mutating `quest_to`, so operators do not
improvise that reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject mutating `quest_to` on gated non-`quest_flag` kinds
  via store `validOptionalServiceQuestGate` (`QuestTo` must remain `0` for
  `KindOpenCube`).
- Spec language already says foreign fields including mutating `quest_to` fail
  closed at store / content-bundle validation, but the checked-in dry-run list
  only covered foreign `size` / `title` / `catalog` / warp coords /
  reward/consume scalars and item tables plus the warehouse gated mutate case —
  not the craftsman gated mutate twin.
- Manual QA still invents throwaway JSON (or confuses the existing gated
  `talk` / warehouse `quest_to` reject fixtures with cube validation) when
  confirming craftsman definitions cannot author `quest_to`, which drifts from
  the owned NPC service examples and from the other checked-in negatives.
- Mutating `quest_to` is the highest-confusion remaining cube gate field: it is
  legal on `quest_flag` turn-ins and illegal on gated `open_cube` (symmetric to
  the landed warehouse twin).

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-cube-foreign-quest-to-bundle.json`
   authors one gated `open_cube` interaction definition with optional
   informational `text`, complete `quest_ref` / `quest_flag` / `quest_from`,
   plus illegal `quest_to = 2`, and includes an in-bundle `quest_flag` writer
   for `quest:first_steps.met_guide` so the only fail-closed reason is the
   mutating craftsman `quest_to` rule.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-cube
   foreign-* / open-safebox foreign-quest-to / gated-service-quest-to
   negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (`open_cube` scalar
  `reward_item_vnum` shorthand) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenCubeForeignQuestToExample|LocalContentBundleValidateEndpointRejectsOpenCubeForeignQuestToExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_safebox` scalar `reward_item_vnum` /
   `reward_item_count` shorthand:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-reward-item-vnum-bundle.json`
   (`docs/plans/2026-09-03-invalid-open-safebox-foreign-reward-item-vnum-fixture.md`).
   Highest-value remaining twin is the `open_cube` scalar `reward_item_vnum`
   shorthand reject.
