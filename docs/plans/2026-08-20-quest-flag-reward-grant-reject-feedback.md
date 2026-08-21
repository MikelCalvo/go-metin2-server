# Quest-Flag Reward Grant Reject Feedback — 2026-08-20

## Objective

Give player-facing self-only `CHAT_TYPE_INFO` feedback when an otherwise valid `quest_flag` turn-in fails closed because a `reward_items` grant cannot be placed or is restricted for the selected character, so kill -> turn-in QA is no longer silent on inventory-full / `anti_get` / `CanUseTemplate` rejects.

## Contract frozen by this slice

1. Keep the existing fail-closed mutation posture: quest-state, gold, experience, and inventory remain unchanged when reward grants cannot apply.
2. Distinguish two player-actionable reward-grant failures during the existing scratch preflight:
   - `quest_reward_inventory_full` when `ValidateCarriedItemGrant` returns `no_valid_placement`
   - `quest_reward_restricted` when `ValidateCarriedItemGrant` returns `invalid` for an owned restriction (`anti_get` or selected-character `CanUseTemplate` rejection)
3. Emit exactly one self-only `GC_CHAT` (`CHAT_TYPE_INFO`, `vid = 0`, `empire = 0`) and mark the ordinary interaction cooldown:
   - inventory full -> `You have too many items.`
   - restricted -> template-authored `buy_reject_message` when present, otherwise `You cannot receive this quest reward.`
4. Missing templates, store errors, and account-save / post-apply rollback failures remain silent fail-closed with no frames.
5. Loopback interaction-visibility previews for the selected character reuse the same messages without mutation when a dry-run CAS would apply but reward placement/restriction fails.
6. Spec/QA wording updates the previously silent restricted / inventory-full turn-in expectations to this chat-backed feedback.

## What this is not yet

- authored reject text fields on the interaction definition itself
- weighted/random turn-in loot
- branching quest scripts
- client quest UI / completion packets
- feedback for account-save rollback paths

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlagRewardItemRejects|TestGameSessionFlowStaticActorQuestFlagRewardItemsRejectsWhenSecondGrantWouldOverflow|GameRuntimeInteractionVisibility' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Keep interaction-definition-authored reject text deferred unless template `buy_reject_message` proves insufficient for QA.
3. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
4. ~~Distinct chat for reward gold/experience carrier overflow.~~ Done: see [quest-flag reward overflow chat](2026-08-21-quest-flag-reward-overflow-chat.md).
