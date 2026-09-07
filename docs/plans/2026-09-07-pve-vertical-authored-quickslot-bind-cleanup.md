# PvE vertical authored quickslot bind/cleanup — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already packet-uses last-stack `27001` and
packet-equips turn-in `11200`, and isolated last-stack use / equip already
delete matching item quickslots, but the playable loop never bound a
quickslot so those `QUICKSLOT_DEL` frames and persisted cleanup stayed
unproven on the authoring-form path.

## Why now

- Client `QUICKSLOT_ADD` plus last-stack `ITEM_USE` / carried-to-equipment
  `ITEM_MOVE` quickslot deletion are already owned.
- Authored `27001.use_effect` and `11200.equip_slot = weapon` are already
  imported by the PvE bundle.
- Binding after `QuestResetGuide` closes the merchant window keeps the
  existing buy/sell/rebuy gold math unchanged.
- An unrelated skill quickslot that shares the potion's byte payload (`1`)
  is the smallest proof that cleanup stays type-scoped.

## Contract frozen by this slice

1. After the stale post-reset `SHOP BUY` fail-closed, packet
   `QUICKSLOT_ADD` binds:
   - skill bar `1` → `{type=skill, pos=1}`
   - item bar `2` → carried potion cell `1`
   - item bar `3` → carried sword cell `0`
   and persists those three bindings.
2. Last-stack packet `ITEM_USE` of authored `27001` now emits
   `ITEM_USE` echo, `PLAYER_POINT_CHANGE` `+50`, `ITEM_DEL`,
   `QUICKSLOT_DEL(2)`, then info chat `consume:27001:+50`. Live and
   persisted quickslots keep the skill plus sword bindings.
3. Packet `ITEM_MOVE` of authored `11200` onto the empty weapon wear cell
   now emits `ITEM_DEL`, equipment `ITEM_SET`, self `CHARACTER_UPDATE`,
   then `QUICKSLOT_DEL(3)`. Live and persisted quickslots keep only the
   unrelated skill binding.

## What this is not yet

- warehouse size-2 occupancy / `SAFEBOX_ITEM_MOVE` in the same composed proof
- cube `add` / `make` / `make all` in the same composed proof
- exchange / refine / mall in the same composed proof
- claiming the whole quickslot system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
