# PvE vertical authored merchant sell-back — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already packet-buys `npc:qa_merchant` catalog slot
`0` (`Small Red Potion` `27001` @ `50g`) after `QuestHunter` turn-in, and
manual QA already expects one whole-stack packet `SHOP SELL` from that same
open window, but the automated proof never sold the just-bought potion.

## Why now

- Packet `SHOP SELL` whole-stack success / gold credit frames are already owned.
- The bought potion template already authors `shop_sell_price = 2`.
- Opening and buying without sell-back hid a missing client-visible economy
  step on the same authoring-form import path.
- `QuestResetGuide` still needs to run while that merchant window is open.
- After this lane rebased onto `main`, the composed proof already packet-uses
  that potion later, so the unique sell-back must rebuy catalog slot `0` before
  reset instead of deleting the later `ITEM_USE`.

## Contract frozen by this slice

1. After the affordable catalog slot `0` buy, keep the QA merchant window open.
2. Packet `SHOP SELL` of carried slot `1` (the just-bought `27001` stack)
   returns self-only `ITEM_DEL` then `PLAYER_POINT_CHANGE(POINT_GOLD)` for
   authored sell credit `2`, with no extra `GC::SHOP OK`. Live and persisted
   gold become the post-buy total plus `2`; inventory keeps only the turn-in
   sword in slot `0`. Quest-state stays at the post-turn-in snapshot.
3. Packet `SHOP BUY` catalog slot `0` again on that same open window returns
   one self-only `ITEM_SET` into carried slot `1` with no extra `GC::SHOP OK`.
   Live and persisted gold debit by `50` from the post-sell total; inventory
   again holds the turn-in sword plus one `27001`. This restores the later
   last-stack `ITEM_USE` without hiding the unique sell-back.
4. `QuestResetGuide` still runs while that same window is open: first frame is
   self-only `GC::SHOP END`, then the authored `met_guide` clear. Gold and
   inventory stay at the post-rebuy snapshot.
5. A later packet `SHOP BUY` against the already-closed stale window still
   fails closed with no frames and no gold/inventory mutation.

## What this is not yet

- partial-stack `SHOP SELL2` in the same composed proof
- foreign-map warp coverage
- pack-member combat in the same proof

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$' -count=1
git diff --check
```
