# Quest-Flag Reward Gold/Experience Overflow Chat — 2026-08-21

## Objective

Give successful-path `quest_flag` turn-ins distinct self-only `CHAT_TYPE_INFO` text when an authored `reward_gold` or `reward_experience` grant would overflow the bootstrap `PLAYER_POINT_CHANGE` carrier (`<= 1<<31-1`), instead of disappearing silently after consume-fee and reward-item preflights already own player-facing chat.

## Why now

- Insufficient consume fees and reward-item inventory-full / restricted grants already return deterministic self-only info chat.
- Gold / experience overflow still returns `Accepted=false` with no frames, so QA cannot tell a near-cap wallet/XP character from a store or template failure.
- The overflow branch already exists as a fail-closed preflight in `HandleInteraction`; only the reason/message split and loopback preview dry-run are missing.

## Contract frozen by this slice

1. Keep the existing fail-closed mutation posture: quest-state, gold, experience, and inventory remain unchanged when a reward gold/experience overflow preflight fails.
2. Distinguish two player-actionable scalar-reward overflows from silent store/template failures:
   - `quest_reward_gold_overflow`
   - `quest_reward_experience_overflow`
3. Emit exactly one self-only `GC_CHAT` (`CHAT_TYPE_INFO`, `vid = 0`, `empire = 0`) and mark the ordinary interaction cooldown:
   - gold overflow -> `You cannot carry any more gold.`
   - experience overflow -> `You cannot gain any more experience.`
4. Missing templates, account-save / post-apply rollback failures, and other non-overflow store errors remain silent fail-closed with no frames.
5. Loopback interaction-visibility previews for the selected character reuse the same distinct messages without mutation when a dry-run CAS would apply and consume fees/materials would succeed, but the authored scalar reward would overflow.
6. Spec/QA wording updates the previously silent gold/experience overflow turn-in expectations to these chat strings.

## What this is not yet

- authored reject text fields on the interaction definition itself
- branching quest scripts / quest UI packets
- distinct chat for account-save rollback paths
- new NPC service kinds

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagRewardGoldRejectsOverflowWithoutMutation|GameSessionFlowStaticActorQuestFlagRewardExperienceRejectsOverflowWithoutMutation|GameRuntimeInteractionVisibilityReturnsQuestFlagRewardGoldOverflowPreview|GameRuntimeInteractionVisibilityReturnsQuestFlagRewardExperienceOverflowPreview)$' -count=1`
- adjacent green keepers:
  - `go test ./internal/minimal -run 'Test(GameSessionFlowStaticActorQuestFlagConsumeGoldRejectsInsufficientGoldWithoutMutation|GameSessionFlowStaticActorQuestFlagConsumeExperienceRejectsInsufficientExperienceWithoutMutation|GameSessionFlowStaticActorQuestFlagRewardItemsRejectsWhenSecondGrantWouldOverflowWithoutMutation|GameRuntimeInteractionVisibilityReturnsQuestFlagRewardInventoryFullPreviewWithoutMutatingQuestState)$' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional interaction-definition-authored reject text remains deferred unless the frozen strings prove insufficient for QA.
3. Keep distinct chat for account-save / post-apply rollback paths deferred.
4. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
