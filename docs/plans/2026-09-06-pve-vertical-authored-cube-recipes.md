# PvE vertical authored cube recipes — 2026-09-06

## Objective

Close the remaining CubeMaster honesty gap: `open_cube` is already an authored
NPC service in the composed PvE / NPC-service fixtures, but craftable rows still
come from the process-local lab snapshot in `internal/cubestore`. This slice
makes cube recipes portable bundle content so one authoring-form import owns
`cube open` **and** the following `/cube r_info` / `r_list` / `m_info` rows.

## Why now

- Track D already places gated `CubeMaster` (`race_num = 20022`) in
  `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` and
  `docs/examples/bootstrap-npc-service-bundle.json`.
- Runtime cube execution (`r_list`, `m_info`, add/del, make) already reads
  `internal/cubestore` snapshots keyed by NPC vnum.
- Without bundle `cube_recipes`, importing those fixtures still silently leans
  on `cubestore.BootstrapSnapshot()` (`reward 27001 x1`, materials `27002 x2`,
  gold `100`, percent `100`). That is lab fallback, not authored content.
- The composed PvE gameplay proof currently stops at `cube open 20022` and
  never asks the imported craftsman for its recipe list.

## Contract owned by this slice

1. Content bundles may carry optional top-level `cube_recipes` as
   `[]cubestore.NPCRecipes` (`npc_vnum` + `recipes[]` with `reward`,
   `materials`, `gold`, `percent`).
2. `Canonicalize` / import / validate:
   - normalize through `cubestore.NormalizeSnapshot`
   - reject invalid snapshots (`cubestore.ValidSnapshot`)
   - reject `cube_recipes` whose `npc_vnum` is not the `race_num` of an
     authored `open_cube` static actor in the same bundle
   - reject recipe reward / material vnums missing from bundled
     `item_templates`
   - treat recipe vnums as item-template references so `27002` is not an
     unreferenced template when it only exists to back cube materials
3. Omitting `cube_recipes` stays legal (lab fallback remains for hermetic
   `/open_cube` tests). Import of a bundle without `cube_recipes` restores the
   lab snapshot; import of a bundle with `cube_recipes` full-replaces the live
   cube-recipe store before the craftsman is used.
4. Export includes live recipes whose `npc_vnum` matches an exported
   `open_cube` actor `race_num`.
5. The PvE authoring fixture, its canonical twin, and the NPC-service fixture
   author the current lab CubeMaster row plus `item_templates` `27002`
   (`Small Blue Potion`) so the composed QA loop is self-contained.
6. `TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` asks
   the unlocked craftsman for `/cube r_info` / `/cube r_info 0` and expects
   `cube r_list 20022 1 27001,1` and `cube m_info 0 1 27002,2/100`.

Checked-in negatives:

- `docs/examples/bootstrap-invalid-cube-recipes-without-item-templates-bundle.json`
- `docs/examples/bootstrap-invalid-cube-recipe-item-missing-from-item-templates-bundle.json`

## Explicit non-goals

- binary cube headers / OR-materials / complicated recipes
- FileStore `CubeRecipeStorePath` config knob (still MemoryStore + import)
- new ops summary endpoints for recipe rows
- changing `/cube make` execution semantics
- pack AI / synchronized respawn

## Validation

```bash
gofmt -w internal/cubestore/store.go internal/contentbundle/bundle.go \
  internal/contentbundle/bundle_test.go internal/minimal/factory.go \
  internal/minimal/pve_vertical_authoring_test.go \
  internal/minimal/npc_service_kill_quest_credit_authoring_test.go \
  internal/ops/contentbundle_test.go
go test ./internal/cubestore ./internal/contentbundle ./internal/ops ./internal/minimal \
  -run 'Test(CanonicalizeNormalizesCubeRecipesReferencedByOpenCubeActor|CanonicalizeRejectsCubeRecipesWithoutBundledItemTemplates|CanonicalizeRejectsCubeRecipeItemMissingFromBundledItemTemplates|CanonicalizeRejectsCheckedInCubeRecipesWithoutItemTemplatesExample|CanonicalizeRejectsCheckedInCubeRecipeItemMissingFromItemTemplatesExample|CanonicalJSONMatchesBootstrapNPCServiceExample|CanonicalJSONMatchesBootstrapPveVerticalCanonicalExample|CanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin|CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|LocalContentBundleValidateEndpointExpandsPveVerticalAuthoringExample|LocalContentBundleValidateEndpointRejectsCubeRecipesWithoutItemTemplatesExample|LocalContentBundleValidateEndpointRejectsCubeRecipeItemMissingFromItemTemplatesExample|GameRuntimeImportsNpcServiceKillQuestCreditExample|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' \
  -count=1
git diff --check
```
