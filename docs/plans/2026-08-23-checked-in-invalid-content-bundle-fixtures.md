# Checked-In Invalid Content-Bundle Fixtures — 2026-08-23

## Objective

Add deterministic negative authoring fixtures for the reject cases operators
still improvise during content-bundle QA:

1. invalid multi-count `regen_spawns` authoring (`count = 2` without `pack_spacing`,
   and later `count = 9`)
2. orphan kill-quest require gate without an in-bundle writer

This closes follow-up #1 from
[example bundles authored aggro/leash radii](2026-08-22-example-bundles-authored-aggro-leash-radii.md).

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject these shapes via inline Go structs.
- Manual QA still invents throwaway JSON for `/local/content-bundle/validate`
  dry-runs, which drifts from the owned authoring examples.
- Checked-in fixtures keep the reject contract inspectable beside the positive
  authoring examples without inventing pack runtime AI.

## Contract frozen by this slice

1. `docs/examples/bootstrap-invalid-regen-count-bundle.json` authors one
   `regen_spawns` row with `count = 2` and no `pack_spacing`.
2. `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json` authors one
   `regen_spawns` row with `count = 9` and `pack_spacing = 100`.
3. `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json`
   authors one `regen_spawns` row with `count = 1` and `pack_spacing = 100`
   (`docs/plans/2026-08-23-invalid-regen-one-count-pack-spacing-fixture.md`).
4. `docs/examples/bootstrap-invalid-orphan-quest-gate-bundle.json` mirrors the
   gated kill-quest-only drop-table authoring shape, but omits the
   `quest:first_steps.met_guide` `quest_flag` writer.
5. `docs/examples/bootstrap-invalid-empty-drop-table-bundle.json` authors one
   completely empty `drop_tables` row (only `ref`) plus a referencing
   `spawn_groups[].reward_drop_table_ref`
   (`docs/plans/2026-08-23-invalid-empty-drop-table-fixture.md`).
6. `Canonicalize(...)` returns `ErrInvalidBundle` for those fixtures.
7. Loopback `POST /local/content-bundle/validate` returns `400` for those.
8. Spec / QA docs name the fixtures as the preferred negative dry-runs.
9. Valid multi-count regen authoring is owned separately via
   [multi-count regen pack placement](2026-08-23-multi-count-regen-pack-placement-contract-freeze.md)
   and `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`.

## What this is not yet

- pack AI / synchronized respawn / assist linkage
- weighted/random loot
- branching quest scripts
- new NPC service kinds
- README churn beyond the owned QA/spec pointers

## TDD and validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInMultiCountRegenWithoutPackSpacingExample|CanonicalizeRejectsCheckedInOrphanQuestGateExample|LocalContentBundleValidateEndpointRejectsMultiCountRegenWithoutPackSpacingExample|LocalContentBundleValidateEndpointRejectsOverMaxRegenCountExample|LocalContentBundleValidateEndpointRejectsOrphanQuestGateExample)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Freeze multi-count regen pack placement in docs/spec before opening any RED
   that widens `regen_spawns.count`.~~ Done: see
   [multi-count regen pack placement contract freeze](2026-08-23-multi-count-regen-pack-placement-contract-freeze.md).
2. ~~After the freeze, open the first GREEN that accepts `count` in `2..8` with
   required `pack_spacing` and updates the invalid-count fixture/docs together.~~
   Done on `lane/content` with
   `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`.
3. ~~Optionally add another checked-in negative fixture only when a later reject
   case still forces QA to invent JSON.~~ Done for one-count + positive
   `pack_spacing`:
   `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json`
   (`docs/plans/2026-08-23-invalid-regen-one-count-pack-spacing-fixture.md`).
4. ~~Optionally add another checked-in negative fixture only when a later reject
   case still forces QA to invent JSON.~~ Done for completely empty
   `drop_tables` (no combat channels and no kill-quest credit):
   `docs/examples/bootstrap-invalid-empty-drop-table-bundle.json`
   (`docs/plans/2026-08-23-invalid-empty-drop-table-fixture.md`).
5. ~~Complete the incomplete automated twins for already-checked-in fixtures.~~
   Done: disk-backed canonicalize for over-max regen plus ops validate for the
   orphan service-gate fixture
   (`docs/plans/2026-08-24-checked-in-invalid-fixture-test-twins.md`).
