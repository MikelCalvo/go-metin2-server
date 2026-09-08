# NPC-service authored cube-grant use — 2026-09-08

## Objective

Close the remaining honesty gap on the dedicated NPC-service CubeMaster path:
`TestNpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists` already
packet-buys catalog slot `2`, then `/cube add 0 0` / `/cube make` after
authored `INTERACT` and persists granted `27001 x1`, but that craft window
stays open and the granted potion is never consumed. NPC-service `27001`
also still omits `use_effect.special_effect_type` while the composed PvE
fixture already authors HP-up-red type `1`, so cube output on this fixture
cannot re-enter the ordinary template-backed last-stack burst.

Keep this off
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`. That
composed loop already owns cube-grant last-stack use.

## Why now

- Last-stack packet `ITEM_USE` of authored `27001` is already owned after
  merchant rebuy and after composed PvE cube-grant
  (`docs/plans/2026-09-06-pve-vertical-authored-use-equip.md`,
  `docs/plans/2026-09-07-pve-vertical-authored-cube-grant-use.md`).
- Cube make already grants that same `vnum` into carried slot `0` on the
  dedicated NPC-service proof.
- `/close_cube` is already owned; the dedicated craft window currently
  skips that close.
- Closing first, then using the granted last stack, is the smallest
  client-visible proof that NPC-service cube output re-enters the ordinary
  template-backed consume path, including authored `SPECIAL_EFFECT`,
  instead of remaining an unused inventory leftover.

## Contract frozen by this slice

1. Checked-in NPC-service example `27001.use_effect` authors
   `special_effect_type = 1` (HP-up-red) while keeping the already-owned
   `point_type=1`, `point_index=1`, `point_delta=50`, and
   `message=consume:27001:+50`.
2. After authored `/cube make` grants `27001 x1` into carried slot `0`,
   `/close_cube` emits one self-only `CHAT_TYPE_COMMAND` `cube close`.
3. Closed-window `/cube r_info` stays silent/no-frame and leaves gold,
   inventory, HP, and quest-state at the post-make snapshot.
4. Packet `ITEM_USE` of that cube-granted last stack emits the owned
   last-stack burst (`ITEM_USE` echo, `PLAYER_POINT_CHANGE` `+50`,
   `ITEM_DEL`, authored self-only HP-up-red `SPECIAL_EFFECT`, info chat
   `consume:27001:+50`) with no `QUICKSLOT_DEL` because no item quickslot
   is bound to the granted cell.
5. Live and persisted inventory become empty, gold stays at the post-make
   snapshot, and both live and persisted HP apply authored
   `point_delta = 50` onto the pre-use values. Quest flag stays
   `met_guide = 1`.

## What this is not yet

- NPC-service `11200` `appearance_vnum` / `equip_effect` parity with PvE
- leftover `27002` merchant sell-back
- `/cube make all`, injected-roll `1..99`, or authored `percent = 0`
- FileStore `CubeRecipeStorePath` config knob
- binary cube headers / OR-materials
- claiming the whole item system is now template-complete

## Verification

```bash
gofmt -w \
  internal/minimal/npc_service_cube_make_authoring_test.go \
  internal/minimal/npc_service_kill_quest_credit_authoring_test.go \
  internal/contentbundle/bundle_test.go
go test ./internal/contentbundle ./internal/ops ./internal/minimal -count=1 \
  -run 'Test(CanonicalJSONMatchesBootstrapNPCServiceExample|ExampleBootstrapNPCServiceBundleStaysValid|ExampleBootstrapNPCServiceBundleCanonicalizes|ExampleBootstrapNPCServiceBundleCarriesMerchantItemTemplates|SummarizeReturnsOpenCubeRouteForCheckedInNPCServiceExample|LocalContentBundleValidateEndpointAcceptsExampleBundle|GameRuntimeImportsNpcServiceExample|NpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists)$'
git diff --check
```
