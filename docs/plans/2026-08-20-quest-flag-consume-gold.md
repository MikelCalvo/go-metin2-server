# Quest-Flag Consume Gold — 2026-08-20

## Objective

Give successful `quest_flag` NPC turn-ins an optional authored `consume_gold` debit so the owned kill -> pickup -> turn-in loop can require a gold fee without inventing quest UI, a second economy runtime, or branching scripts.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional `consume_gold` (`uint64`, JSON `consume_gold`).
2. Only `kind = "quest_flag"` may author a non-zero `consume_gold`.
3. Non-zero values must fit the already-owned bootstrap gold `PLAYER_POINT_CHANGE` carrier (`<= 1<<31-1`), matching `reward_gold`.
4. Non-`quest_flag` interaction kinds must keep `consume_gold` absent/`0`.
5. Runtime debits gold **only** when the compare-and-set transition applies, and only after a fail-closed preflight proves the selected character's live gold can supply the authored amount.
6. On insufficient gold before CAS, do **not** mutate quest state / gold / inventory / experience; emit the existing self-only mismatch chat `Quest requirements are not met.` and apply the ordinary interaction cooldown.
7. On success with `consume_gold > 0`, after the acknowledgement chat and before any authored `reward_gold` / `reward_experience` frames, emit one self-only `PLAYER_POINT_CHANGE` gold debit frame (`Amount = -consume_gold`, `Value = gold after debit`).
8. Authored `reward_gold` still emits its ordinary positive gold frame after any consume-gold debit when present.
9. Consume-gold + reward-gold + reward-experience + consume-items + reward-items + quest mutation share one fail-closed transaction with the existing rollback posture.
10. Operator summaries / previews surface `consume_gold` on quest-flag trigger and route rows. Compact previews append `[consume_gold N]` after reward markers and before any `[consume_item ...]` markers. Character-scoped interaction visibility also dry-runs live gold sufficiency and previews the mismatch text when gold is missing.
11. QA fixtures make `quest:first_steps_kill_turnin` consume `25` gold beside the existing potion consume / reward path.

## What this is not yet

- experience spend gates
- distinct player-facing "not enough gold" text separate from the owned mismatch chat
- weighted / random fee tables
- quest UI / completion packets
- branching quest scripts

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'QuestFlag.*ConsumeGold|ConsumeGold' -count=1`
- `go test ./internal/player -run 'DeductLiveGold' -count=1`
- `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagConsumeGold|GameRuntimeInteractionVisibilityReturnsQuestFlagConsumeGold|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1`
- `go test ./internal/contentbundle -run 'NpcServiceBundle|PveVertical|QuestFlag.*ConsumeGold|KillTurnin' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional distinct insufficient-gold chat remains a later UX seam.
3. Required experience spend gates remain deferred.
