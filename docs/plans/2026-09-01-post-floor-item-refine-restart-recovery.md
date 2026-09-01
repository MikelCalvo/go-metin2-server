# Post-floor non-refineable `REFINE` restart recovery — 2026-09-01

## Objective

Close the remaining item-refine recovery twin after packet `REFINE` already
owned quiet post-floor denial before template-authored reject feedback: prove
`/restart_here` and `/restart_town` restore a usable live owner so the same
non-refineable `REFINE` emits the ordinary self-only reject chat again without
mutating inventory.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while carrying one template-backed non-refineable item that authors
   `refine_reject_message`.
2. Later packet `REFINE` fails closed with:
   - no template-authored reject info-chat feedback
   - no inventory / quickslot / point / ground / persistence mutation
3. After `/restart_here` restores live HP, the same `REFINE` emits one
   self-only `CHAT_TYPE_INFO` using the authored `refine_reject_message` and
   leaves inventory unchanged beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `REFINE` likewise emits the ordinary reject chat beside recovered MaxHP
   and town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRejectFeedback`
   - `TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing accepted refine confirm / success gameplay in this slice
- widening into storage-packet (`SAFEBOX_CHECKIN` / checkout / mall) restart
  twins in this same commit
- changing already-owned live template-backed `REFINE` reject behavior or the
  already-owned refineable preview reopen twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemRefineFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
