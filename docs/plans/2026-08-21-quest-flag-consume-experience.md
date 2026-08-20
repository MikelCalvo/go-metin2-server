# Quest-Flag Consume Experience — 2026-08-21

## Objective

Give successful `quest_flag` NPC turn-ins an optional authored `consume_experience` debit so the owned kill -> pickup -> turn-in loop can require an experience fee without inventing quest UI, a second progression runtime, or branching scripts.

## Contract frozen by this slice

1. `interactionstore.Definition` gains optional `consume_experience` (`uint64`, JSON `consume_experience`).
2. Only `kind = "quest_flag"` may author a non-zero `consume_experience`.
3. Non-zero values must fit the already-owned bootstrap experience `PLAYER_POINT_CHANGE` carrier (`<= 1<<31-1`), matching `reward_experience` / `consume_gold`.
4. Non-`quest_flag` interaction kinds must keep `consume_experience` absent/`0`.
5. Runtime debits experience **only** when the compare-and-set transition applies, and only after a fail-closed preflight proves the selected character's live experience point can supply the authored amount.
6. On insufficient experience before CAS, do **not** mutate quest state / gold / inventory / experience; emit the existing self-only mismatch chat `Quest requirements are not met.` and apply the ordinary interaction cooldown.
7. On success with `consume_experience > 0`, after the acknowledgement chat and any authored `consume_gold` debit, and before any authored `reward_gold` / `reward_experience` frames, emit one self-only `PLAYER_POINT_CHANGE` experience debit frame (`Amount = -consume_experience`, `Value = experience after debit`).
8. Authored `reward_experience` still emits its ordinary positive experience frame after any consume-experience debit when present.
9. Consume-experience + consume-gold + reward-gold + reward-experience + consume-items + reward-items + quest mutation share one fail-closed transaction with the existing rollback posture.
10. Operator summaries / previews surface `consume_experience` on quest-flag trigger and route rows. Compact previews append `[consume_experience N]` after `[consume_gold N]` (when present) and before any `[consume_item ...]` markers. Character-scoped interaction visibility also dry-runs live experience sufficiency and previews the mismatch text when experience is missing.
11. QA fixtures make `quest:first_steps_kill_turnin` consume `10` experience beside the existing gold / potion consume / reward path.

## What this is not yet

- distinct player-facing "not enough experience" text separate from the owned mismatch chat
- weighted / random fee tables
- quest UI / completion packets
- branching quest scripts

## TDD and validation

Focused coverage:

- `go test ./internal/interactionstore -run 'QuestFlag.*ConsumeExperience|ConsumeExperience' -count=1`
- `go test ./internal/player -run 'DeductLiveExperience' -count=1`
- `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagConsumeExperience|GameRuntimeInteractionVisibilityReturnsQuestFlagConsumeExperience|GameRuntimeInteractionVisibilityReturnsQuestFlagInsufficientConsumeExperience|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1`
- `go test ./internal/contentbundle -run 'NpcServiceBundle|PveVertical|QuestFlag.*ConsumeExperience|KillTurnin' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional distinct insufficient-experience chat remains a later UX seam.
3. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
