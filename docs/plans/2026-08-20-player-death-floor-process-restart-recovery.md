# Player Death-Floor Process-Restart Recovery — 2026-08-20

## Objective

Extend Track E.4 crash/restart rematerialization coverage to the retaliation-owned
player death floor already persisted by `player-death-bootstrap.md`: when practice-mob
retaliation drives selected-character bootstrap HP to `0`, a fresh `gameRuntime`
rebuilt from the same account / login-ticket / content FileStore paths must
rematerialize that dead snapshot on EnterGame even when the post-restart login
ticket still carries the pre-death live HP value.

Same-process reconnect and `/phase_select` re-entry already prove the floor
persists. This slice closes the remaining process-restart gap for the PvE vertical
(`mob kill -> death floor -> reconnect/restart`).

## Contract frozen by this slice

1. Login continues to prefer the committed `accountstore` roster over the ticket
   character snapshot when both are present.
2. Immediate practice-mob retaliation that reaches the bootstrap `0`-HP floor still
   persists selected-character `points[1]=0` with the owned death/clear frames.
3. After process restart from the same FileStore paths:
   - rematerialized bootstrap HP remains `0`
   - EnterGame rebuilds self `PLAYER_POINT_CHANGE` at floor `0` plus self
     `GC DEAD(owner_vid)` using the already-owned dead-owner bootstrap shape
     (non-player visibility skipped for the still-dead recipient)
   - connected-character / points operator snapshots report the rematerialized
     dead state (`dead: true`, `points[1]=0`)
   - a post-restart owner-side combat `TARGET` attempt against the rematerialized
     practice mob fails closed (no self target ack)
4. The focused proof is
   `TestGameRuntimePlayerDeathFloorRematerializesAcrossDaemonRestart`.
5. Pending bootstrap ground-item / ground-gold restart durability remains out of
   scope.

## What this is not yet

- process-restart restoration of pending ground items / ground gold
- broader corpse gameplay or revive menus beyond owned `/restart_here` /
  `/restart_town`
- SQL-backed repositories or import/backfill from quarantined exports
- automatic stale migration-lock removal
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'TestGameRuntimePlayerDeathFloorRematerializesAcrossDaemonRestart' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide that
   quarantined `0010` exports should drive recovery.
2. ~~Add hostname / build-identity fields to migration apply-lock artifacts.~~
   Done: see [migration apply lock host / build identity](2026-08-20-migration-apply-lock-host-build-identity.md).
3. Keep SQL import/backfill deferred until a driver-backed harness and backup
   policy exist.
