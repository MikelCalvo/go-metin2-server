# PvE vertical authored warehouse password reopen — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already stores/retrieves turn-in `11200` after
`QuestGuide` re-unlock, but that mutation still opened through lab
`/open_safebox` (default size `1`) instead of the already-authored `Warehouse`
`open_safebox` actor (size `2`).

## Why now

- Warehouse `INTERACT` + `/safebox_password` and the 10-second same-socket
  reopen cooldown are already owned.
- The first composed warehouse close happens before cube + reconnect, so a
  same-session password attempt still hits the cooldown.
- Reconnect clears that same-socket cooldown; after `QuestGuide` re-unlock
  the authored warehouse can reopen without waiting or using the lab opener.
- Successful warehouse `INTERACT` while a merchant window is open prepends
  `GC::SHOP END` before `ShowMeSafeboxPassword`.

## Contract frozen by this slice

1. After the first unlocked warehouse close, same-session `Warehouse`
   `INTERACT` still emits authored info chat + `ShowMeSafeboxPassword`, but
   `/safebox_password 000000` returns `You cannot open the warehouse again so
   soon after closing it.` with no `SAFEBOX_SIZE`.
2. After reconnect and `QuestGuide` re-unlock reopen the QA merchant,
   warehouse `INTERACT` prepends `GC::SHOP END`, then `/safebox_password
   000000` emits authored `SAFEBOX_SIZE` size `2` plus `SAFEBOX_MONEY_CHANGE`.
3. Equipped `SAFEBOX_CHECKIN`, unequip, checkin, checkout, and authored
   `shop_sell_price = 100` sell stay on that authored-size-2 presentation
   instead of lab `/open_safebox` size `1`.

## What this is not yet

- cube mutation / refine in the same composed proof
- mall, password-change, or warehouse-money mutation on the PvE vertical
- claiming the whole storage system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
