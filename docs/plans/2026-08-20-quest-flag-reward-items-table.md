# Quest-Flag Multi-Entry Reward Items — 2026-08-20

## Objective

Extend successful `quest_flag` NPC turn-ins from a single scalar carried-item grant to a small deterministic `reward_items[]` table so the owned kill -> turn-in loop can deliver more than one client-visible inventory payoff without inventing quest UI, weighted loot, mail, or a second reward runtime.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional `reward_items` (`[]RewardItemEntry`, JSON `reward_items`).
2. Each entry is `{ "item_vnum": uint32, "count": uint16 }`.
3. Only `kind = "quest_flag"` may author reward items.
4. Scalar `reward_item_vnum` / `reward_item_count` remain a one-entry authoring shorthand:
   - when `reward_items` is empty and scalar vnum is non-zero, canonicalize expands them into a one-entry `reward_items` table and clears the scalars
   - authoring both a non-empty `reward_items` table and non-zero scalars together is invalid
5. Authored table rules after normalize:
   - `0..QuestFlagRewardItemsMax` entries (`8`)
   - each entry requires `item_vnum != 0` and `count` in `1..255`
   - duplicate vnums are allowed (each entry is an independent grant)
6. Content-bundle canonicalization / validation requires an in-bundle item template for every `reward_items[].item_vnum`, and each count must fit that template (`<= max_count`; non-stackable templates require `count == 1`).
7. Runtime grants every table entry **only** when the compare-and-set transition applies.
8. On success, after the self-only acknowledgement chat and any authored gold/experience point-change frames, the interacting player receives the ordinary carried-inventory SET/UPDATE frames for each granted placement in authored table order.
9. Placement still reuses merchant-buy carried rules (merge when allowed, otherwise first free slot) with fresh item instance ids.
10. Gold + experience + all reward items + quest mutation share one fail-closed transaction:
    - preflight must prove every entry is grantable (including sequential placement capacity) before applying the quest transition
    - if live grant or account save fails after the transition applies, roll back live gold/experience/inventory and reverse the quest transition; emit no frames
11. Operator summaries / previews surface the full `reward_items` table on quest-flag trigger and route rows. Compact previews append one `[reward_item <name|vnum> x<count>]` annotation per entry (after any `[reward_gold N]` / `[reward_experience N]` markers). Scalar summary fields remain populated from the first table entry for backward-compatible compact readback.
12. QA fixtures grant two carried items on `quest:first_steps_kill_turnin` (`QuestHunter`):
    - `Small Red Potion` (`27001` x1)
    - `Wooden Sword` (`11200` x1)
    beside the existing `reward_gold = 100` and `reward_experience = 50`.

## What this is not yet

- weighted / random turn-in loot
- multi-entry gold/experience tables
- ground-drop or mail delivery on turn-in
- required item consume / turn-in prerequisites beyond flag CAS
- quest UI / completion packets
- branching quest scripts
- SQL-backed interaction or quest repositories

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'QuestFlag.*RewardItems|RewardItems' -count=1`
- `go test ./internal/contentbundle -run 'QuestFlag.*RewardItems|KillTurnin|NpcServiceBundle|PveVertical' -count=1`
- `go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlagRewardItems|TestGameRuntimeInteractionVisibilityReturnsQuestFlagReward' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
3. Optional required-item consume gates on turn-in remain a later content seam.
