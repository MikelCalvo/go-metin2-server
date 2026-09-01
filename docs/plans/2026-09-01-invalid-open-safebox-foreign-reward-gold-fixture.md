# Invalid Open-Safebox Foreign Reward Gold Fixture — 2026-09-01

## Objective

Close the next optional negative dry-run gap after the owned `open_safebox`
oversize-`size` and foreign-`title` / foreign-`catalog` / foreign-warp-coords
fixtures: check in a deterministic fixture for an `open_safebox` definition that
illegally authors turn-in `reward_gold`, so operators do not improvise that
reject during `/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject foreign `reward_gold` on `open_safebox` via inline Go
  structs (`TestFileStoreRejectsInvalidOpenSafeboxDefinitions` /
  `"reward gold not allowed"`).
- Spec language already says foreign reward/consume gold/experience fields fail
  closed at store / content-bundle validation, but the checked-in dry-run list
  only covered oversize `size` plus foreign `title` / `catalog` / warp coords —
  not the turn-in economy field case.
- Manual QA still invents throwaway JSON (or confuses `/local/interactions`
  non-`quest_flag` `reward_gold` rejects with authored warehouse validation)
  when confirming warehouse definitions cannot author turn-in `reward_gold`,
  which drifts from the owned NPC service examples and from the other
  checked-in negatives.
- Foreign `reward_gold` is the highest-confusion remaining warehouse field after
  oversize `size` / foreign placement fields: it is legal on `quest_flag`
  turn-ins and illegal on `open_safebox`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-foreign-reward-gold-bundle.json`
   authors one `open_safebox` interaction definition with optional informational
   `text` plus illegal `reward_gold = 10`, so the only fail-closed reason is the
   foreign turn-in gold field.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   oversize-size / foreign-title / foreign-catalog / foreign-warp-coords /
   open-cube foreign-* negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (reward/consume experience,
  consume gold, mutating `quest_to`) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxForeignRewardGoldExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxForeignRewardGoldExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_cube` foreign turn-in `reward_gold`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-reward-gold-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-cube-foreign-reward-gold-fixture.md`).
   Done for `open_cube` foreign turn-in `reward_experience`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-reward-experience-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-cube-foreign-reward-experience-fixture.md`).
   Done for `open_safebox` foreign turn-in `reward_experience`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-reward-experience-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-safebox-foreign-reward-experience-fixture.md`).
   Done for `open_safebox` foreign turn-in `consume_gold`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-consume-gold-bundle.json`
   (`docs/plans/2026-09-01-invalid-open-safebox-foreign-consume-gold-fixture.md`).
