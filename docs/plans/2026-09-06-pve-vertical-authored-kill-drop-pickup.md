# PvE vertical authored kill-drop pickup — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already drops `Small Red Potion` (`27001`) on the
gated kill, and the quest-state spec already says the player picks that drop
up before `QuestHunter` turn-in, but the automated proof seeded the potion in
account inventory.

## Why now

- `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` / `ITEM_PICKUP` are already owned.
- `QuestHunter` already consumes authored `consume_items` (`27001` x1).
- Manual QA already expects pickup of the practice-mob drop after kill credit.
- Seeding the turn-in item hid a missing client-visible step in the composed
  PvE loop.

## Contract frozen by this slice

1. The composed proof no longer seeds carried inventory.
2. The pre-guide kill still emits the authored `GROUND_ADD` + `OWNERSHIP` pair
   for `27001` (no quest chat). Owner reconnect already deletes that still-owned
   handle, so the later credited kill can reuse the same reward VID.
3. After `QuestGuide` unlock, the credited killing hit emits the same drop pair
   plus kill-quest chat. The proof then sends `ITEM_PICKUP` and requires
   `GROUND_DEL` + `ITEM_SET` + `ITEM_GET` for `27001` in carried slot `0`.
4. Live and persisted inventory must hold that picked-up potion before
   `QuestHunter` turn-in. Turn-in still consumes it and grants `Wooden Sword`
   (`11200`).

## What this is not yet

- a new interaction kind
- loot-table / random-drop redesign
- `QuestResetGuide` / foreign-map warp coverage in the same proof

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$' -count=1
git diff --check
```
