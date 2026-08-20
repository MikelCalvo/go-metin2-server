# Quest-Flag Consume Items — 2026-08-20

## Objective

Give successful `quest_flag` NPC turn-ins an optional authored `consume_items[]` table so the owned kill -> pickup -> turn-in loop can require and remove carried quest materials without inventing quest UI, item-use scripting, or a second inventory mutation runtime.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional `consume_items` (`[]RewardItemEntry`, JSON `consume_items`).
2. Each entry is `{ "item_vnum": uint32, "count": uint16 }` — the same shape as `reward_items`.
3. Only `kind = "quest_flag"` may author consume items.
4. Authored table rules after normalize:
   - `0..QuestFlagConsumeItemsMax` entries (`8`)
   - each entry requires `item_vnum != 0` and `count` in `1..255`
   - duplicate vnums are allowed; runtime sums by vnum before consuming
5. No scalar shorthand for consume items in this slice.
6. Content-bundle canonicalization / validation requires an in-bundle item template for every `consume_items[].item_vnum`, and each count must fit that template (`<= max_count`; non-stackable templates require `count == 1`).
7. Runtime consumes every required count **only** when the compare-and-set transition applies, and only after a fail-closed preflight proves the selected character's carried inventory can supply every entry.
8. Consume planning mirrors refine material consumption:
   - ascending carried slot order
   - skip equipped / locked stacks
   - partial stacks become `ITEM_UPDATE`; emptied stacks become `ITEM_DEL`
   - full slot clears also delete matching item quickslots
9. On insufficient carried materials before CAS, do **not** mutate quest state / gold / inventory; emit the existing self-only mismatch chat `Quest requirements are not met.` and apply the ordinary interaction cooldown.
10. On success, after the acknowledgement chat and any authored gold/experience frames, emit consume DEL/UPDATE (+ quickslot DEL) frames, then the ordinary reward grant SET/UPDATE frames.
11. Gold + experience + consume + reward items + quest mutation share one fail-closed transaction with the existing rollback posture.
12. Operator summaries / previews surface `consume_items` on quest-flag trigger and route rows. Compact previews append one `[consume_item <name|vnum> x<count>]` annotation per authored entry after any reward markers. Character-scoped interaction visibility also dry-runs live inventory sufficiency and previews the mismatch text when materials are missing.
13. QA fixtures make `quest:first_steps_kill_turnin` consume the practice-mob drop (`Small Red Potion` `27001` x1), so the honest loop is kill -> pickup -> turn-in.

## What this is not yet

- equipment / safebox / exchange consumption
- anti-flag specific reject text for turn-in materials
- weighted / random consume tables
- quest UI / completion packets
- branching quest scripts

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'ConsumeCarriedItems' -count=1`
- `go test ./internal/interactionstore -run 'QuestFlag.*ConsumeItems|ConsumeItems' -count=1`
- `go test ./internal/contentbundle -run 'QuestFlag.*ConsumeItems|NpcServiceBundle|PveVertical' -count=1`
- `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagConsumeItems|GameRuntimeInteractionVisibilityReturnsQuestFlagReward|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional distinct player-facing text for missing materials remains a later UX seam.
3. Required gold / experience spend gates remain deferred.
