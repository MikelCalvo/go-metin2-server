# Quest-Flag Reward Item Restriction Guards — 2026-08-20

## Objective

Prove the already-owned `quest_flag` carried-item grant path fails closed before any quest transition / gold / inventory mutation when a reward template is marked `anti_get` or is rejected by selected-character `CanUseTemplate` guards (job/sex/empire/`min_level`), so kill -> turn-in cannot silently hand restricted loot into inventory.

## Contract frozen by this slice

1. `player.ValidateCarriedItemGrant` / `GrantCarriedItem` reject `anti_get` and `!CanUseTemplate(...)` as `CarriedItemGrantFailureInvalid` with no live inventory/gold mutation.
2. `quest_flag` turn-in preflight uses that same grant validation before applying the compare-and-set quest transition.
3. Session/runtime proof: restricted reward templates emit no chat / point-change / item frames, leave quest-state, gold, experience, inventory, and persisted account snapshots unchanged.
4. Spec/QA wording names `anti_get` and selected-character restrictions explicitly beside the already-documented missing-template / inventory-full / account-save fail-closed posture.

## What this is not yet

- authored reject-chat feedback for restricted turn-in grants
- weighted/random turn-in loot
- branching quest scripts
- accepted safebox/storage mutations

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'ValidateCarriedItemGrantRejectsAntiGet|GrantCarriedItem' -count=1`
- `go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlagRewardItemRejects' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange `START` rejects deferred until those presentation seams exist.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
3. Player-facing restricted turn-in grant reject chat is now owned (`buy_reject_message` / `You cannot receive this quest reward.`); keep interaction-definition-authored reject text deferred.
