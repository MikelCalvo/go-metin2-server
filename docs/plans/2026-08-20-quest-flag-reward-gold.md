# Quest-Flag Reward Gold — 2026-08-20

## Objective

Give successful `quest_flag` NPC interactions an optional authored gold grant so the owned kill -> turn-in loop can deliver a client-visible economy payoff without inventing quest item grants, dialog trees, or a second reward runtime.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional `reward_gold` (`uint64`, JSON `reward_gold`).
2. Only `kind = "quest_flag"` may author a non-zero `reward_gold`.
3. Non-zero values must fit the already-owned bootstrap gold `POINT_CHANGE` carrier (`<= 1<<31-1`).
4. Runtime grants gold **only** when the compare-and-set transition applies.
5. On success with `reward_gold > 0`, the interacting player receives:
   - the existing self-only `CHAT_TYPE_INFO` acknowledgement (`definition.text`)
   - then one self-only `PLAYER_POINT_CHANGE` gold frame for the granted amount
6. Gold is persisted into the selected-character account snapshot with the same live/persisted refresh pattern already used by practice-mob death rewards.
7. Fail-closed before mutation when the gold grant cannot be applied (overflow / unavailable selected character): no quest transition, no gold, no frames.
8. If the quest transition applies and live gold mutates but account save fails, roll back live gold and reverse the quest transition; emit no frames.
9. Operator summaries / previews surface `reward_gold` on quest-flag trigger and route rows, and compact previews annotate successful text with `[reward_gold N]` when present.
10. QA fixtures grant `reward_gold = 100` on `quest:first_steps_kill_turnin` (`QuestHunter`).

## What this is not yet

- quest item grants / inventory mutation on turn-in
- EXP grants on `quest_flag`
- reward grants on kill-quest credit itself beyond the already-owned combat death descriptor
- quest UI / completion packets
- SQL-backed interaction or quest repositories

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'QuestFlag.*RewardGold|RewardGold' -count=1`
- `go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlagRewardGold' -count=1`
- `go test ./internal/contentbundle -run 'QuestFlag.*RewardGold|KillTurnin|NpcServiceBundle|PveVertical' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optional `reward_item_vnum` / carried inventory grant on the same successful `quest_flag` path.
2. Optional `reward_experience` beside gold.
3. Keep weighted/random loot and branching quest scripts deferred.
