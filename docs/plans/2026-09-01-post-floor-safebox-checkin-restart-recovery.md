# Post-floor anti-safebox `SAFEBOX_CHECKIN` restart recovery — 2026-09-01

## Objective

Close the remaining storage-packet recovery twin after
`TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeAntiSafeboxFeedback`
already proved quiet post-floor denial before anti-safebox feedback: prove
`/restart_here` and `/restart_town` restore a usable live owner so the same
`anti_safebox` `SAFEBOX_CHECKIN` emits the ordinary self-only reject chat again
without mutating inventory.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while carrying one template-backed `anti_safebox` item that authors
   `safebox_reject_message`.
2. Later packet `SAFEBOX_CHECKIN` fails closed with:
   - no template-authored `anti_safebox` info-chat feedback
   - no safebox/mall response frames
   - no inventory / quickslot / point / gold / ground / persistence mutation
3. After `/restart_here` restores live HP, the same `SAFEBOX_CHECKIN` emits one
   self-only `CHAT_TYPE_INFO` using the authored `safebox_reject_message` and
   leaves inventory unchanged beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `SAFEBOX_CHECKIN` likewise emits the ordinary reject chat beside
   recovered MaxHP and town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeAntiSafeboxFeedback`
   - `TestGameSessionFlowPostFloorSafeboxCheckinFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing accepted open-presentation check-in success gameplay in this slice
- widening into `SAFEBOX_CHECKOUT` / `SAFEBOX_ITEM_MOVE` / mall restart twins in
  this same commit
- changing already-owned live anti-safebox `SAFEBOX_CHECKIN` behavior or the
  already-owned `/open_safebox` reopen twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxCheckinFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
