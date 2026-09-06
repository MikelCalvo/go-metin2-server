# PvE vertical authored merchant buy — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already places a gated `Merchant` `shop_preview`
catalog (`27001` @ `50g`, `11200` @ `500g`), and manual QA already expects one
packet `SHOP BUY` from that open window, but the automated proof only opened
the shop and later used a silent stale buy after `QuestResetGuide`.

## Why now

- Packet `SHOP BUY` success / insufficient-gold frames are already owned.
- `QuestHunter` turn-in already leaves enough gold for catalog slot `0` and
  not enough for slot `1`.
- Opening the merchant without buying hid a missing client-visible economy
  step on the same authoring-form import path.

## Contract frozen by this slice

1. After `QuestHunter` turn-in, reopen the QA merchant.
2. Packet `SHOP BUY` catalog slot `1` (`Wooden Sword` `11200` @ `500g`)
   returns one self-only `GC::SHOP NOT_ENOUGH_MONEY`, leaves gold/inventory
   and quest-state unchanged, and keeps the merchant window open.
3. Packet `SHOP BUY` catalog slot `0` (`Small Red Potion` `27001` @ `50g`)
   returns one self-only `ITEM_SET` into the first free carried slot (slot `1`,
   because the turn-in sword already occupies slot `0`), with no extra
   `GC::SHOP OK`. Live and persisted gold debit by `50`; inventory keeps the
   sword and adds the potion. Quest-state stays at the post-turn-in snapshot.
4. `QuestResetGuide` still runs while that same window is open: first frame is
   self-only `GC::SHOP END`, then the authored `met_guide` clear. Gold and
   inventory stay at the post-buy snapshot.

## What this is not yet

- merchant sell-back in the same composed proof
- foreign-map warp coverage
- pack-member combat in the same proof

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$' -count=1
git diff --check
```
