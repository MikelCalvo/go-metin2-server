# Invalid Open-Safebox Foreign Consume Experience Fixture — 2026-09-02

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox` /
`open_cube` foreign-`consume_gold` fixtures: check in a deterministic fixture
for an `open_safebox` definition that illegally authors turn-in
`consume_experience`, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `consume_experience` on non-`quest_flag`
  kinds via `TestFileStoreRejectsConsumeExperienceOnNonQuestFlagDefinitions`,
  and `validDefinition` requires `ConsumeExperience == 0` for
  `KindOpenSafebox`.
- Spec language already says foreign reward/consume gold/experience fields fail
  closed at store / content-bundle validation, but the checked-in dry-run list
  only covered oversize `size` plus foreign `title` / `catalog` / warp coords /
  `reward_gold` / `reward_experience` / `consume_gold` — not the warehouse
  turn-in consume-experience field case.
- Manual QA still invents throwaway JSON (or confuses `/local/interactions`
  non-`quest_flag` `consume_experience` rejects with authored warehouse
  validation) when confirming warehouse definitions cannot author turn-in
  `consume_experience`, which drifts from the owned NPC service examples and
  from the other checked-in negatives.
- Foreign `consume_experience` is the highest-confusion remaining warehouse
  scalar fee field after foreign reward gold/experience and consume gold: it is
  legal on `quest_flag` turn-ins and illegal on `open_safebox`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-foreign-consume-experience-bundle.json`
   authors one `open_safebox` interaction definition with optional informational
   `text` plus illegal `consume_experience = 10`, so the only fail-closed reason
   is the foreign turn-in consume-experience field.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   oversize-size / foreign-title / foreign-catalog / foreign-warp-coords /
   foreign-reward-gold / foreign-reward-experience / foreign-consume-gold /
   open-cube foreign-* negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (open_cube consume experience,
  reward items, consume items, mutating `quest_to`) unless QA still improvises
  that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxForeignConsumeExperienceExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxForeignConsumeExperienceExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON. Highest-value remaining twin is `open_cube` foreign
   turn-in `consume_experience`.
