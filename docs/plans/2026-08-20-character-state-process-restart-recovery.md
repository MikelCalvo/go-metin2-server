# Character-State Process-Restart Recovery — 2026-08-20

## Objective

Prove Track E.4 crash/restart rematerialization for the durable PvE character
state already owned by bootstrap FileStores: gold, inventory, and quest flags.

After a successful `quest_flag` turn-in that grants gold + item and clears the
turn-in flag, a fresh `gameRuntime` rebuilt from the same account / quest /
interaction / item-template FileStore paths must rematerialize that committed
state on EnterGame even when the post-restart login ticket still carries the
pre-reward character snapshot.

## Contract frozen by this slice

1. Login continues to prefer the committed `accountstore` roster over the ticket
   character snapshot when both are present.
2. After process restart from the same FileStore paths:
   - rematerialized gold matches the post-turn-in account snapshot
   - rematerialized inventory matches the post-turn-in account snapshot
   - committed quest-state remains cleared for the consumed turn-in flag
   - a second interaction against the same quest_flag definition rejects with
     the ordinary requirements-not-met info chat and does not grant again
3. The focused proof is
   `TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart`.
4. Pending bootstrap ground-item / ground-gold handles remain out of scope.

## What this is not yet

- process-restart restoration of pending ground items / ground gold
- SQL-backed repositories or import/backfill from quarantined exports
- deployment topology / artifact retention policy
- remote admin auth
- changing backup/restore or crash-temp endpoint contracts

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide that
   quarantined `0010` exports should drive recovery.
2. Add production-safe observability conventions before remote admin surfaces.
3. Add deployment topology / artifact retention docs once production hosts are
   known.
