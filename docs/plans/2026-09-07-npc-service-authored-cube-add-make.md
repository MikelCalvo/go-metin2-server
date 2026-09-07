# NPC-service authored CubeMaster add/make — 2026-09-07

## Objective

Close the remaining honesty gap on the dedicated NPC-service CubeMaster path:
`docs/examples/bootstrap-npc-service-bundle.json` already authors gated
`open_cube` plus portable `cube_recipes`, and lab `/cube add` / `/cube make`
already mutate inventory/gold, but no focused proof opened that craftsman
through `INTERACT` and then bound/consumed the authored materials.

Keep this off
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`. That
composed loop still has empty inventory at first cube open, and `lane/items`
is already extending it for warehouse money.

## Why now

- Authored `CubeMaster` `INTERACT` + `/cube r_info` / `m_info` are already owned.
- Bundle `cube_recipes` already author `27002 x2` → `27001 x1` / gold `100` /
  `percent = 100` for NPC `20022`.
- Lab `/open_cube` add/make proofs do not import the NPC-service fixture or
  walk the quest-gated craftsman.
- Manual QA already expects `/cube add` then `/cube make` once materials are
  in inventory.

## Contract frozen by this slice

1. Import `docs/examples/bootstrap-npc-service-bundle.json`.
2. `QuestGuide` `INTERACT` writes `quest:first_steps.met_guide = 1`.
3. `CubeMaster` `INTERACT` emits authored info chat
   `The craftsman lights the forge.` plus `cube open 20022`.
4. `/cube add 0 5` then `/cube add 1 6` against two seeded `27002` cells emit
   `cube info 0 0 0` then `cube info 100 0 0` with no gold/inventory mutation.
5. `/cube make` consumes both materials and `100` gold, grants `27001 x1`,
   emits self-only `ITEM_DEL` / `ITEM_SET` / gold `PLAYER_POINT_CHANGE` /
   `cube success 27001 1` / follow-up `cube info 0 0 0`, and persists the
   same live snapshot. Quest flag stays `met_guide = 1`.

The later merchant-buy follow-up
`docs/plans/2026-09-07-npc-service-authored-cube-material-buy.md` replaces
those seeded cells with packet `SHOP BUY` catalog slot `2`.

## What this is not yet

- cube `add` / `make` inside the composed PvE vertical gameplay proof
- `/cube make all`, injected-roll `1..99`, or authored `percent = 0`
- FileStore `CubeRecipeStorePath` config knob
- merchant catalog cube-material buy as a prerequisite (now owned on the
  dedicated NPC-service proof; see
  `docs/plans/2026-09-07-npc-service-authored-cube-material-buy.md`)
- binary cube headers / OR-materials

## Verification

```bash
gofmt -w internal/minimal/npc_service_cube_make_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestNpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists$'
git diff --check
```
