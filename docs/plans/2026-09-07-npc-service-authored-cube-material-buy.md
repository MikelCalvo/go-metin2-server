# NPC-service authored cube-material merchant buy — 2026-09-07

## Objective

Close the remaining honesty gap on the dedicated NPC-service CubeMaster path:
`docs/examples/bootstrap-npc-service-bundle.json` already authors gated
`shop_preview` plus cube material template `27002`, and lab `/cube add` /
`/cube make` already consume carried materials, but the focused CubeMaster
proof still seeded two `27002` cells instead of buying them from the QA
merchant.

Keep this off
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`. That
composed loop still has empty inventory at first cube open, and `lane/items`
is already extending it.

## Why now

- Authored `CubeMaster` `INTERACT` + `/cube add` / `/cube make` are already
  owned on the NPC-service fixture.
- Packet `SHOP BUY` already grants catalog `count` into the first free
  carried slot.
- Cube matching already aggregates a single bound cell (`27002 x2`) as a
  complete material set.
- Manual QA already expects one affordable merchant buy before craft.

## Contract frozen by this slice

1. Import `docs/examples/bootstrap-npc-service-bundle.json`.
2. `npc:qa_merchant` catalog slot `2` authors `27002 x2` @ `20g`. Template
   `27002` now also carries `shop_buy_price = 10`.
3. `QuestGuide` `INTERACT` writes `quest:first_steps.met_guide = 1`.
4. `Merchant` `INTERACT` opens `GC::SHOP START` with slots `0` / `1` / `2`.
5. Packet `SHOP BUY` catalog slot `2` returns one self-only `ITEM_SET` of
   `27002 x2` into carried slot `0` (no extra `GC::SHOP OK`), debits live and
   persisted gold by `20`, then `SHOP END` closes the window.
6. `CubeMaster` `INTERACT` emits authored info chat
   `The craftsman lights the forge.` plus `cube open 20022`.
7. `/cube add 0 0` against the bought stacked cell emits `cube info 100 0 0`
   with no gold/inventory mutation.
8. `/cube make` consumes that stacked cell and `100` gold, grants `27001 x1`,
   emits self-only `ITEM_DEL` / `ITEM_SET` / gold `PLAYER_POINT_CHANGE` /
   `cube success 27001 1` / follow-up `cube info 0 0 0`, and persists the
   same live snapshot. Quest flag stays `met_guide = 1`.

The later cube-grant use follow-up
`docs/plans/2026-09-08-npc-service-authored-cube-grant-use.md` closes that
window and packet-uses the granted last stack.

## What this is not yet

- cube `add` / `make` inside the composed PvE vertical gameplay proof
- cube-grant last-stack `ITEM_USE` on this dedicated proof (now owned; see
  `docs/plans/2026-09-08-npc-service-authored-cube-grant-use.md`)
- `/cube make all`, injected-roll `1..99`, or authored `percent = 0`
- FileStore `CubeRecipeStorePath` config knob
- merchant sell-back of leftover cube materials
- binary cube headers / OR-materials

## Verification

```bash
gofmt -w \
  internal/minimal/npc_service_cube_make_authoring_test.go \
  internal/minimal/npc_service_kill_quest_credit_authoring_test.go \
  internal/contentbundle/bundle_test.go
go test ./internal/contentbundle ./internal/ops ./internal/minimal -count=1 \
  -run 'Test(CanonicalJSONMatchesBootstrapNPCServiceExample|ExampleBootstrapNPCServiceBundleStaysValid|ExampleBootstrapNPCServiceBundleCanonicalizes|ExampleBootstrapNPCServiceBundleCarriesMerchantItemTemplates|ExampleBootstrapNPCServiceBundleExportsAndQuarantinesStaticActorPvEMigrationShape|LocalContentBundleValidateEndpointAcceptsExampleBundle|GameRuntimeImportsNpcServiceExample|NpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists)$'
git diff --check
```
