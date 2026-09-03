# Invalid Open-Safebox Foreign Reward-Item-Vnum Fixture — 2026-09-03

## Objective

Close the next optional negative dry-run gap after the owned warehouse/cube
foreign-`quest_to` fixtures: check in a deterministic fixture for an
`open_safebox` definition that illegally authors scalar `reward_item_vnum` /
`reward_item_count` shorthand, so operators do not improvise that reject during
`/local/content-bundle/validate`.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit / store validation already rejects reward items on non-`quest_flag` kinds:
  `ValidDefinition` normalizes scalar `reward_item_vnum` / `reward_item_count`
  into `reward_items` first, then `KindOpenSafebox` requires
  `!hasRewardItems(definition)`.
- Spec language already says foreign reward/consume fields fail closed at store /
  content-bundle validation, but the checked-in dry-run list only covered the
  table form (`reward_items`) for warehouse definitions — not the scalar
  shorthand authors commonly copy from `quest_flag` turn-ins.
- Manual QA still invents throwaway JSON (or confuses the landed
  `foreign-reward-items` table fixture with scalar shorthand authoring) when
  confirming warehouse definitions cannot author `reward_item_vnum`, which drifts
  from the owned NPC service examples and from the other checked-in negatives.
- Scalar `reward_item_vnum` is the highest-confusion remaining warehouse turn-in
  field after the table / gold / experience / quest_to twins: it is legal on
  `quest_flag` turn-ins and illegal on gated/ungated `open_safebox`.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-open-safebox-foreign-reward-item-vnum-bundle.json`
   authors one `open_safebox` interaction definition with optional informational
   `text` plus illegal `reward_item_vnum = 27001` / `reward_item_count = 1`, so
   the only fail-closed reason is the foreign scalar reward-item shorthand.
2. `Canonicalize(...)` returns `ErrInvalidBundle` for that fixture (bundle
   validation already rejects invalid interaction definitions through
   `interactionstore.ValidDefinition`; this slice binds the checked-in JSON plus
   an explicit content-bundle reject twin).
3. Loopback `POST /local/content-bundle/validate` returns `400` for that
   fixture.
4. Spec / QA / prior plan docs name the fixture beside the existing open-safebox
   foreign-* / open-cube foreign-* / gated-service-quest-to negatives.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / store reject rule
- further checked-in foreign-field negatives (open_cube scalar
  `reward_item_vnum` twin) unless QA still improvises that JSON later

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInOpenSafeboxForeignRewardItemVnumExample|LocalContentBundleValidateEndpointRejectsOpenSafeboxForeignRewardItemVnumExample)$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Keep branching quest scripts deferred.
3. ~~Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.~~ Done for `open_cube` scalar `reward_item_vnum` /
   `reward_item_count` shorthand:
   `docs/examples/bootstrap-invalid-open-cube-foreign-reward-item-vnum-bundle.json`
   (`docs/plans/2026-09-03-invalid-open-cube-foreign-reward-item-vnum-fixture.md`).
   With warehouse + craftsman scalar shorthand twins landed, Track D
   foreign-field dry-run coverage for owned NPC service kinds is otherwise
   closed unless a new reject still forces improvised JSON.
