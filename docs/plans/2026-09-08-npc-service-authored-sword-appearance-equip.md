# NPC-service authored sword appearance/equip — 2026-09-08

## Objective

Close the remaining honesty gap on the dedicated NPC-service merchant path:
`docs/examples/bootstrap-npc-service-bundle.json` already authors gated
`shop_preview` catalog slot `1` as `Wooden Sword` `11200` with
`equip_slot = weapon`, and composed PvE already authors
`appearance_vnum = 11201` plus `equip_effect` `{point_type:1, point_index:1,
point_delta:10}` on that same vnum, but NPC-service import still strips those
fields. Buying that catalog row therefore cannot re-enter the ordinary
template-backed weapon equip burst.

Keep this off
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` and off
`TestNpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists`. Those proofs
already own composed PvE sword equip and NPC-service cube-grant use.

## Why now

- Packet `SHOP BUY` already grants catalog `count` into the first free
  carried slot.
- Packet `ITEM_MOVE` onto the authored `weapon` wear cell already emits
  `ITEM_DEL` / equipment `ITEM_SET` / template-backed `PLAYER_POINT_CHANGE` /
  `CHARACTER_UPDATE` when `equip_slot`, `appearance_vnum`, and `equip_effect`
  are present (`docs/plans/2026-09-06-pve-vertical-authored-use-equip.md`).
- NPC-service `27001.use_effect.special_effect_type` already caught up with
  PvE (`docs/plans/2026-09-08-npc-service-authored-cube-grant-use.md`); `11200`
  is the leftover template-parity hole named by that slice.
- Buying then equipping catalog slot `1` is the smallest client-visible proof
  that NPC-service sword output re-enters the ordinary template-backed equip
  path instead of remaining a shop-only leftover.

## Contract frozen by this slice

1. Checked-in NPC-service example `11200` authors `appearance_vnum = 11201`
   and `equip_effect` `{point_type:1, point_index:1, point_delta:10}` while
   keeping already-owned `equip_slot = weapon` and `shop_sell_price = 100`.
2. Import `docs/examples/bootstrap-npc-service-bundle.json`.
3. `QuestGuide` `INTERACT` writes `quest:first_steps.met_guide = 1`.
4. `Merchant` `INTERACT` opens `GC::SHOP START` with slots `0` / `1` / `2`.
5. Packet `SHOP BUY` catalog slot `1` (`11200 x1` @ `500g`) returns one
   self-only `ITEM_SET` of `11200 x1` into carried slot `0` (no extra
   `GC::SHOP OK`), debits live and persisted gold by `500`, then `SHOP END`
   closes the window.
6. Packet `ITEM_MOVE` of that bought sword onto the empty weapon wear cell
   emits `ITEM_DEL`, equipment `ITEM_SET` still carrying item `vnum = 11200`,
   one self-only HP `PLAYER_POINT_CHANGE` (`amount = +10`), then
   `CHARACTER_UPDATE` whose weapon `parts[1]` projects authored
   `appearance_vnum = 11201`. No `QUICKSLOT_DEL` because no item quickslot is
   bound to the bought cell.
7. Live and persisted inventory become empty, equipment holds `11200` in
   `weapon`, gold stays at the post-buy snapshot, and both live and persisted
   HP apply authored `point_delta = 10` onto the pre-equip values. Quest flag
   stays `met_guide = 1`.
8. Packet unequip onto empty carried slot `0` emits the inverse burst
   (`ITEM_DEL`, carried `ITEM_SET` `vnum = 11200`, `PLAYER_POINT_CHANGE`
   `amount = -10`, `CHARACTER_UPDATE` with cleared weapon part).

## What this is not yet

- leftover `27002` merchant sell-back
- sword sell-back / warehouse occupancy on this dedicated proof
- claiming the whole item system is now template-complete
- refine catalysts / mall / party ownership

## Verification

```bash
gofmt -w \
  internal/minimal/npc_service_sword_equip_authoring_test.go \
  internal/minimal/npc_service_kill_quest_credit_authoring_test.go \
  internal/contentbundle/bundle_test.go
go test ./internal/contentbundle ./internal/ops ./internal/minimal -count=1 \
  -run 'Test(CanonicalJSONMatchesBootstrapNPCServiceExample|ExampleBootstrapNPCServiceBundleStaysValid|ExampleBootstrapNPCServiceBundleCanonicalizes|ExampleBootstrapNPCServiceBundleCarriesMerchantItemTemplates|SummarizeReturnsOpenCubeRouteForCheckedInNPCServiceExample|LocalContentBundleValidateEndpointAcceptsExampleBundle|GameRuntimeImportsNpcServiceExample|NpcServiceBundleMerchantSwordBuyEquipsAppearanceAndEquipEffect)$'
git diff --check
```
