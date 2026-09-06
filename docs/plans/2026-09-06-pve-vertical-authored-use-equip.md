# PvE vertical authored use/equip — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already grants `Wooden Sword` `11200` and packet-buys
`Small Red Potion` `27001`, but content-bundle import replaced the bootstrap
default templates with shop-only rows. That stripped `27001.use_effect` and
`11200.equip_slot`, so the playable PvE loop could not `/use_item` / `ITEM_USE`
the potion or `ITEM_MOVE` the sword onto the weapon wear cell.

## Why now

- Direct `ITEM_USE` and packet equip are already owned, template-backed paths.
- The built-in missing-file fallback already authors `consume:27001:+50` and
  `11200` `equip_slot = weapon`.
- Importing the PvE/NPC-service examples currently overwrote those fields and
  hid a missing client-visible use/equip step on the same authoring-form path.

## Contract frozen by this slice

1. Checked-in PvE/NPC-service example `item_templates` author:
   - `27001` `use_effect` `{point_type:1, point_index:1, point_delta:50, message:"consume:27001:+50"}`
   - `11200` `equip_slot = "weapon"`
   while keeping existing shop prices.
2. After `QuestResetGuide` closes the merchant window, packet `ITEM_USE` on the
   bought potion (carried slot `1`) emits the owned last-stack burst
   (`ITEM_USE` echo, `PLAYER_POINT_CHANGE` `+50`, `ITEM_DEL`, info chat
   `consume:27001:+50`) and persists the point/inventory change.
3. Packet `ITEM_MOVE` of the turn-in sword (carried slot `0`) onto the authored
   weapon wear cell emits `ITEM_DEL` / equipment `ITEM_SET` / self
   `CHARACTER_UPDATE` with weapon appearance part `11200`, and persists
   empty carried inventory plus `equip_slot = weapon`.
4. Fail-closed negatives stay owned: missing `use_effect` rejects `ITEM_USE`;
   mismatched authored `equip_slot` rejects weapon-cell `ITEM_MOVE`.

## What this is not yet

- merchant sell-back of the remaining/unequipped sword
- warehouse/safebox mutation in the same composed proof
- pack-member combat or foreign-map warp coverage
- claiming the whole item system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/contentbundle ./internal/minimal ./internal/ops ./internal/player -count=1 \
  -run 'TestCanonicalJSONMatchesBootstrapNPCServiceExample|TestCanonicalJSONMatchesBootstrapPveVerticalCanonicalExample|TestCanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin|TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn|TestPveVerticalTemplateBackedUseAndEquipFailClosedWithoutAuthoredMetadata|TestGameRuntimeImportsNpcServiceExample|TestLocalContentBundleValidateEndpointExpandsPveVerticalAuthoringExample'
git diff --check
```
