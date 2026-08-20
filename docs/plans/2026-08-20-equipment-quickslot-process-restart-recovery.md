# Equipment + Quickslot Process-Restart Recovery — 2026-08-20

## Objective

Extend Track E.4 crash/restart rematerialization coverage beyond gold / inventory /
quest flags to the other durable PvE character item-state already owned by the
bootstrap `accountstore` FileStore: equipment and quickslots.

After a live session equips an inventory item and binds a quickslot, a fresh
`gameRuntime` rebuilt from the same account / login-ticket FileStore paths must
rematerialize that committed equipment and quickslot state on EnterGame even when
the post-restart login ticket still carries the pre-mutation character snapshot.

## Contract frozen by this slice

1. Login continues to prefer the committed `accountstore` roster over the ticket
   character snapshot when both are present.
2. After process restart from the same FileStore paths:
   - rematerialized equipment matches the post-equip account snapshot
   - rematerialized quickslots match the post-bind account snapshot
   - rematerialized inventory / gold remain consistent with the same account
     snapshot (equipped item leaves inventory; gold unchanged)
   - a post-restart quickslot delete persists against the rematerialized state
3. The focused proof is
   `TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart`.
4. Pending bootstrap ground-item / ground-gold handles, position rematerialization,
   and point-state rematerialization remain out of scope.

## What this is not yet

- process-restart restoration of pending ground items / ground gold
- map/x/y or HP/EXP point rematerialization proofs
- SQL-backed repositories or import/backfill from quarantined exports
- deployment topology / artifact retention policy
- production-safe observability conventions
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add position (map/x/y) rematerialization across daemon restart next.
2. Add character point-state (HP/EXP from `0011`) rematerialization proofs.
3. Keep ground-item restart durability deferred until operators decide that
   quarantined `0010` exports should drive recovery.
4. ~~Add production-safe observability conventions before remote admin surfaces.~~
   Done: `internal/observability` + `docs/workflow/production-observability.md`.
