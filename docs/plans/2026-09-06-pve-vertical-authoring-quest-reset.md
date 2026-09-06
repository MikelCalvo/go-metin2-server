# PvE vertical authored QuestResetGuide revoke — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already places `QuestResetGuide`
(`quest:first_steps_reset`) beside `QuestGuide` / `QuestHunter`, and manual QA
already expects clearing `met_guide` to revoke gated services, but the automated
proof never INTERACTed that reset actor.

## Why now

- `quest_flag` clear (`quest_from = 1`, `quest_to = 0`) is already owned.
- Gated `shop_preview` / `talk` / `info` / `warp` already fail closed on
  mismatch.
- INTERACT with a non-merchant actor already prepends `GC::SHOP END` when a
  merchant window is open.
- Seeding the reset through `POST /local/quest-state/transition` hid the
  authored NPC that QA actually uses.

## Contract frozen by this slice

1. Before `QuestGuide` unlock, INTERACT with `QuestResetGuide` returns
   `Quest requirements are not met.` and does not mutate quest-state.
2. After `QuestHunter` turn-in, reopen the QA merchant, then INTERACT
   `QuestResetGuide` while that window is still open:
   - first frame is self-only `GC::SHOP END`
   - second frame is the authored clear chat
     `Quest cleared: first_steps.met_guide = 0.`
   - persisted quest-state drops the selected character's `met_guide` row
   - gold / inventory stay at the post-turn-in snapshot
3. A later packet `SHOP BUY` against the already-closed stale window fails
   closed with no frames and no gold/inventory mutation.
4. Subsequent INTERACT with `Merchant`, `VillageGuide`, and `QuestResetGuide`
   returns `Quest requirements are not met.` until `QuestGuide` writes
   `met_guide = 1` again; after that re-unlock the merchant opens normally.

## What this is not yet

- foreign-map warp coverage in the same proof
- branching dialog trees or a second quest runtime
- proving the operator-only `ApplyQuestStateTransition` path again (already
  owned by `TestGameSessionFlowQuestGatedShopBuyClosesWhenRequirementClearedWhileOpen`)

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go internal/contentbundle/bundle_test.go
go test ./internal/minimal -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$' -count=1
go test ./internal/contentbundle -run 'TestCanonicalizePveVerticalAuthoringExampleExpandsQuestLoop$' -count=1
git diff --check
```
