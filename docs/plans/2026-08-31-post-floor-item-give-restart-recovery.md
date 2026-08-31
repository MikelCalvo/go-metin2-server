# Post-floor `ITEM_GIVE` restart recovery — 2026-08-31

## Objective

Close the remaining item-give recovery twin after packet `ITEM_GIVE` already
owned quiet post-floor denial before anti-give feedback: prove `/restart_here`
and `/restart_town` restore a usable live owner so the same anti-give
`ITEM_GIVE` against a visible living peer emits the ordinary self-only reject
chat again without mutating inventory.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while carrying one template-backed `anti_give` stack and keeping a
   visible living peer in range.
2. Later packet `ITEM_GIVE` fails closed with:
   - no anti-give info-chat feedback
   - no peer delivery / inventory / quickslot / ground / persistence mutation
3. After `/restart_here` restores live HP, the same `ITEM_GIVE` against that
   same living peer emits one self-only `CHAT_TYPE_INFO` using the authored
   `give_reject_message` and leaves both inventories unchanged.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same `ITEM_GIVE` against a living destination-map peer likewise emits the
   ordinary anti-give reject chat beside recovered MaxHP and town-return
   coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeAntiGiveFeedback`
   - `TestGameSessionFlowPostFloorItemGiveFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing accepted peer-to-peer item transfer through `ITEM_GIVE`
- widening into `REFINE`, storage-packet, or sell restart twins in this same
  commit
- changing already-owned live anti-give `ITEM_GIVE` behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemGiveFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
