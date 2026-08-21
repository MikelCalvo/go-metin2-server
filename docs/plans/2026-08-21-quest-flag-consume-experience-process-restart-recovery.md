# Quest-Flag Consume/Reward Experience Process-Restart Recovery — 2026-08-21

## Objective

Prove Track E.4 crash/restart rematerialization for the durable PvE experience
delta owned by a successful `quest_flag` turn-in that both debits
`consume_experience` and grants `reward_experience`.

After a successful turn-in, a fresh `gameRuntime` rebuilt from the same account /
quest / interaction / item-template FileStore paths must rematerialize the net
experience point on EnterGame even when the post-restart login ticket still
carries the pre-turn-in experience snapshot. The cleared quest flag must stay
idempotent so a second interaction does not debit or grant again.

## Contract frozen by this slice

1. Login continues to prefer the committed `accountstore` roster over the ticket
   character snapshot when both are present.
2. After process restart from the same FileStore paths:
   - rematerialized experience matches the post-turn-in account snapshot
     (`seed - consume_experience + reward_experience`)
   - rematerialized gold / inventory match the post-turn-in account snapshot
   - committed quest-state remains cleared for the consumed turn-in flag
   - a second interaction against the same quest_flag definition rejects with
     the ordinary requirements-not-met info chat and does not mutate experience,
     gold, or inventory again
3. The focused proof is:
   - `TestGameRuntimeQuestFlagConsumeExperienceStateRematerializesAcrossDaemonRestart`
4. Pending bootstrap ground-item / ground-gold handles remain out of scope.
5. Distinct insufficient-experience chat, branching quest scripts, and SQL
   import/backfill remain out of scope.

## What this is not yet

- process-restart restoration of pending ground items / ground gold
- SQL-backed repositories or import/backfill from quarantined exports
- automatic stale-lock removal or DB-engine advisory locks
- remote admin auth
- changing backup/restore or crash-temp endpoint contracts
- distinct player-facing "not enough experience" text

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'TestGameRuntimeQuestFlagConsumeExperienceStateRematerializesAcrossDaemonRestart' -count=1`
- adjacent green keepers:
  - `go test ./internal/minimal -run 'TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart|TestGameRuntimePositionAndPointsRematerializeAcrossDaemonRestart|TestGameSessionFlowStaticActorQuestFlagConsumeExperience' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide that
   quarantined `0010` exports should drive recovery.
2. Keep SQL import/backfill deferred until a driver-backed harness and backup
   policy exist.
3. Optional distinct insufficient-experience chat remains a later UX seam.
4. Keep automatic stale-lock removal deferred until a deployment-specific
   recovery policy exists.
