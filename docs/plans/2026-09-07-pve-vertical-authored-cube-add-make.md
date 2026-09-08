# PvE vertical authored cube add/make — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already INTERACTs gated `CubeMaster` and inspects
bootstrap `/cube r_info` / `/cube r_info 0` with empty inventory, and the
dedicated NPC-service proof already packet-buys catalog slot `2` then
`/cube add` / `/cube make`, but the composed PvE loop still stopped at
non-mutating inspect.

Keep the first CubeMaster open empty-inventory / low-gold. Insert material
buy + craft only after the authored sword `SHOP SELL`, when carried gold is
enough for `20g` materials plus `100g` craft cost and inventory is empty.

## Why now

- Dedicated NPC-service CubeMaster already owns packet-buy catalog slot `2`
  (`27002 x2` @ `20g`) then `/cube add 0 0` / `/cube make`
  (`docs/plans/2026-09-07-npc-service-authored-cube-material-buy.md`).
- Composed PvE already authors `cube_recipes` for NPC `20022` and template
  `27002`, but `npc:qa_merchant` still only sold `27001` / `11200`.
- After sword sell, gold is `237` and inventory is empty, so slot `0` can
  receive the stacked material cell the same way the dedicated proof does.
- Manual QA already expects one affordable merchant buy of cube materials
  before craft.

## Contract frozen by this slice

1. Import `docs/examples/bootstrap-pve-vertical-authoring-bundle.json`
   (canonical twin `docs/examples/bootstrap-pve-vertical-canonical-bundle.json`
   expands to the same catalog / template).
2. Template `27002` now also carries `shop_buy_price = 10`. `npc:qa_merchant`
   catalog slot `2` authors `27002 x2` @ `20g`.
3. First unlocked CubeMaster inspect still runs with empty inventory and
   gold `40` (post warehouse-save); `/cube r_info` / `/cube r_info 0` stay
   non-mutating.
4. After authored sword `SHOP SELL` (merchant window still open), packet
   `SHOP BUY` catalog slot `2` returns one self-only `ITEM_SET` of
   `27002 x2` into carried slot `0` (no extra `GC::SHOP OK`) and debits live
   and persisted gold by `20`.
5. `SHOP END` closes the window, then `CubeMaster` `INTERACT` emits authored
   info chat `The craftsman lights the forge.` plus `cube open 20022`.
6. `/cube add 0 0` against the bought stacked cell emits `cube info 100 0 0`
   with no gold/inventory mutation.
7. `/cube make` consumes that stacked cell and `100` gold, grants `27001 x1`,
   emits self-only `ITEM_DEL` / `ITEM_SET` / gold `PLAYER_POINT_CHANGE` /
   `cube success 27001 1` / follow-up `cube info 0 0 0`, and persists the
   same live snapshot. Quest flag stays `met_guide = 1`.

## What this is not yet

- `/cube make all`, injected-roll `1..99`, or authored `percent = 0` in the
  composed proof
- ~~FileStore `CubeRecipeStorePath` config knob~~ Later closed by
  [authored cube-recipe FileStore](2026-09-08-pve-vertical-authored-cube-recipe-filestore.md).
- merchant sell-back of leftover cube materials
- binary cube headers / OR-materials
- ~~using the cube-granted `27001` on the same composed proof~~ Later closed by
  authored cube-grant last-stack `ITEM_USE`; see
  [pve-vertical-authored-cube-grant-use](2026-09-07-pve-vertical-authored-cube-grant-use.md).
- refine / mall / party ownership

## Verification

```bash
gofmt -w \
  internal/minimal/pve_vertical_authoring_test.go \
  internal/contentbundle/bundle_test.go
go test ./internal/contentbundle ./internal/ops ./internal/minimal -count=1 \
  -run 'Test(CanonicalJSONMatchesBootstrapPveVerticalCanonicalExample|CanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin|CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|ExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles|LocalContentBundleValidateEndpointExpandsPveVerticalAuthoringExample|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$'
git diff --check
```
