# Quest-Flag Reward Item — 2026-08-20

## Objective

Give successful `quest_flag` NPC interactions an optional authored carried-inventory item grant so the owned kill -> turn-in loop can deliver a client-visible item payoff beside the already-owned `reward_gold` path, without inventing quest dialog trees, mail, or ground-drop rewards for turn-in.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional:
   - `reward_item_vnum` (`uint32`, JSON `reward_item_vnum`)
   - `reward_item_count` (`uint16`, JSON `reward_item_count`)
2. Only `kind = "quest_flag"` may author a non-zero reward item.
3. Authored pairing rules:
   - `reward_item_vnum == 0` requires `reward_item_count == 0`
   - `reward_item_vnum != 0` requires `reward_item_count` in `1..255`
4. Content-bundle canonicalization / validation additionally requires an in-bundle item template for every authored `reward_item_vnum`, and the count must fit that template (`count <= max_count`; non-stackable templates require `count == 1`).
5. Runtime grants the item **only** when the compare-and-set transition applies.
6. On success with `reward_item_vnum > 0`, after the self-only acknowledgement chat (and after any authored `reward_gold` point-change), the interacting player receives the ordinary carried-inventory item SET/UPDATE frames for the granted placement.
7. Item placement reuses the merchant-buy carried placement rules (merge into compatible stacks when allowed, otherwise first free carried slot) and allocates a fresh item instance id.
8. Gold + item + quest mutation share one fail-closed transaction:
   - validate gold overflow / item grantability before applying the quest transition
   - if live grant or account save fails after the transition applies, roll back live gold/inventory and reverse the quest transition; emit no frames
9. Inventory-full / missing template / AntiGet / unusable template fail closed with no frames (same posture as gold overflow).
10. Operator summaries / previews surface `reward_item_vnum` / `reward_item_count` on quest-flag trigger and route rows, and compact previews annotate successful text with `[reward_item <name|vnum> x<count>]` when present (after any `[reward_gold N]` annotation).
11. QA fixtures grant `reward_item_vnum = 27001`, `reward_item_count = 1` on `quest:first_steps_kill_turnin` (`QuestHunter`) beside the existing `reward_gold = 100`.

## What this is not yet

- quest EXP grants
- multi-item reward tables / weighted loot on turn-in
- ground-drop turn-in rewards
- quest UI / completion packets
- mail / storage delivery
- SQL-backed interaction or quest repositories

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'QuestFlag.*RewardItem|RewardItem' -count=1`
- `go test ./internal/player -run 'GrantCarriedItem|ValidateCarriedItemGrant' -count=1`
- `go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlagRewardItem' -count=1`
- `go test ./internal/contentbundle -run 'QuestFlag.*RewardItem|KillTurnin|NpcServiceBundle|PveVertical' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Multi-entry structured turn-in reward tables (`reward_items[]`) are now owned; keep weighted/random turn-in loot deferred.
3. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
