# PvE vertical authored warehouse checkin — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already opens/closes the gated `Warehouse`
`open_safebox` actor, and packet `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` are
already owned, but the PvE proof never mutates warehouse contents. After
`QuestGuide` re-unlock the sword is worn, so a composed warehouse mutation
must fail closed until unequip.

## Why now

- Accepted checkin/checkout bursts and `anti_safebox` reject chat are already owned.
- Equipped items cannot be checked in (`SafeboxCheckinItem` rejects `item.Equipped`).
- Warehouse `INTERACT` + `/safebox_password` after `/close_safebox` hits the
  owned 10-second reopen cooldown on the same live session. This slice used
  lab `/open_safebox` to bypass that cooldown; the later reconnect + authored
  `Warehouse` password reopen replaces that lab opener (see
  [pve-vertical-authored-warehouse-password-reopen](2026-09-06-pve-vertical-authored-warehouse-password-reopen.md)).
- Successful checkin while a merchant window is open prepends `GC::SHOP END`.

## Contract frozen by this slice

1. After `QuestGuide` re-unlock reopens the QA merchant, this slice originally
   packet-closed that window and lab-opened `/open_safebox` (`SAFEBOX_SIZE` +
   `SAFEBOX_MONEY_CHANGE`; lab open does not prepend `SHOP END`). The current
   composed proof instead INTERACTs authored `Warehouse` while the merchant
   window is still open (`GC::SHOP END` then `ShowMeSafeboxPassword`) and
   opens with `/safebox_password 000000` (`SAFEBOX_SIZE` size `2`).
2. `SAFEBOX_CHECKIN` of carried slot `0` while `11200` is still worn fails
   closed with no frames and no inventory/equipment/gold mutation.
3. Packet unequip onto carried slot `0`, then `SAFEBOX_CHECKIN` into safebox
   cell `0` emits `ITEM_DEL` + `SAFEBOX_SET` (`vnum=11200`) and empties live
   inventory while gold stays at the post-buy snapshot.
4. `SAFEBOX_CHECKOUT` back onto carried slot `0` emits `SAFEBOX_DEL` + `ITEM_SET`,
   then `SHOP SELL` of that last stack still credits authored `shop_sell_price=100`.

## What this is not yet

- ~~warehouse `INTERACT` + `/safebox_password` reopen after the 10s cooldown in
  the same composed proof~~ Later closed by reconnect + authored `Warehouse`
  password reopen; see
  [pve-vertical-authored-warehouse-password-reopen](2026-09-06-pve-vertical-authored-warehouse-password-reopen.md).
- cube mutation / refine in the same composed proof
- claiming the whole storage system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go internal/player/runtime_test.go
go test ./internal/player ./internal/minimal -count=1 \
  -run 'TestRuntimeSafeboxCheckinItemRejectsEquippedCarriedSlotWithoutMutation|TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
