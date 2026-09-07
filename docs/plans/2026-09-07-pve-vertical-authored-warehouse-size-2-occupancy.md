# PvE vertical authored warehouse size-2 occupancy — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already opens gated `Warehouse` `open_safebox`
(size `2`, capacity `size * 5 = 10` cells) and stores/retrieves turn-in
`11200` through `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT`, but that mutation
still occupies only cell `0`. Cell `5` is the first slot that default lab
`/open_safebox` size `1` cannot use, so authored size `2` stayed unproven as
live occupancy.

## Why now

- Whole-stack empty-destination `SAFEBOX_ITEM_MOVE` is already owned.
- Authored warehouse reopen after `QuestGuide` re-unlock already emits
  `SAFEBOX_SIZE` size `2`.
- Occupying cell `5` after check-in, then checking out from that cell, is
  the smallest client-visible proof that the composed loop uses the authored
  page count instead of lab size `1`.
- Out-of-range cell `10` (`>= size * 5`) stays fail-closed on that same
  size-`2` presentation.

## Contract frozen by this slice

1. After unequip `SAFEBOX_CHECKIN` of turn-in `11200` into safebox cell `0`,
   whole-stack `SAFEBOX_ITEM_MOVE` `0 -> 10` fails closed with no frames and
   leaves the durable sword in cell `0`.
2. Whole-stack empty-destination `SAFEBOX_ITEM_MOVE` `0 -> 5` emits
   `SAFEBOX_DEL` (cell `0`) + `SAFEBOX_SET` (cell `5`, `vnum=11200`) and
   persists that occupancy through the same-account safebox FileStore.
3. `SAFEBOX_CHECKOUT` from cell `5` onto carried slot `0` emits `SAFEBOX_DEL`
   + `ITEM_SET`, then authored `shop_sell_price = 100` sell stays unchanged.

## What this is not yet

- cube `add` / `make` / `make all` in the same composed proof
- refine / exchange / mall in the same composed proof
- occupying the rest of the size-`2` grid or claiming the whole storage
  system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
