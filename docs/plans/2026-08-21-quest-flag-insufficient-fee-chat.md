# Quest-Flag Insufficient-Fee Chat — 2026-08-21

## Objective

Give successful-path `quest_flag` turn-ins distinct self-only `CHAT_TYPE_INFO` text when an authored consume preflight fails for gold, experience, or carried materials, instead of collapsing those fee failures into the generic `Quest requirements are not met.` CAS mismatch message.

## Why now

- Reward-grant rejects already own distinct inventory-full / restricted chat.
- Consume fees (`consume_gold`, `consume_experience`, `consume_items`) still reuse the CAS mismatch string, so QA cannot tell a wrong quest flag from a missing fee.
- The failure branch already exists in `HandleInteraction` and loopback interaction-visibility previews; only the reason/message split is missing.

## Contract frozen by this slice

1. Keep the existing fail-closed mutation posture: quest-state, gold, experience, and inventory remain unchanged when a consume preflight fails.
2. Distinguish three player-actionable consume failures from `quest_current_value_mismatch`:
   - `quest_insufficient_gold`
   - `quest_insufficient_experience`
   - `quest_insufficient_materials`
3. Emit exactly one self-only `GC_CHAT` (`CHAT_TYPE_INFO`, `vid = 0`, `empire = 0`) and mark the ordinary interaction cooldown:
   - insufficient gold -> `You do not have enough gold.`
   - insufficient experience -> `You do not have enough experience.`
   - insufficient materials -> `You do not have the required items.`
4. True compare-and-set `current_value_mismatch` and non-mutating service quest gates continue to use `Quest requirements are not met.`
5. Loopback interaction-visibility previews for the selected character reuse the same distinct messages without mutation.
6. Spec/QA wording updates the previously generic insufficient-fee turn-in expectations to these chat strings.

## What this is not yet

- authored reject text fields on the interaction definition itself
- branching quest scripts / quest UI packets
- distinct chat for gold/experience overflow or account-save rollback paths
- new NPC service kinds

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagConsumeGoldRejectsInsufficientGoldWithoutMutation|GameSessionFlowStaticActorQuestFlagConsumeExperienceRejectsInsufficientExperienceWithoutMutation|GameSessionFlowStaticActorQuestFlagConsumeItemsRejectsInsufficientMaterialsWithoutMutation|GameRuntimeInteractionVisibilityReturnsQuestFlagInsufficientConsumeGoldPreview|GameRuntimeInteractionVisibilityReturnsQuestFlagInsufficientConsumeExperiencePreview)$' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional interaction-definition-authored reject text remains deferred unless the frozen strings prove insufficient for QA.
3. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
