# Post-floor peer chat / whisper / info restart recovery — 2026-09-01

## Objective

Close the remaining communication recovery twin after owner-side peer-facing
`CHAT` / `WHISPER` and self-only `CHAT_TYPE_INFO` already owned quiet post-floor
denial: prove `/restart_here` and `/restart_town` restore a usable live owner so
those same communication surfaces succeed normally again.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Later owner-side `CHAT` (`TALKING` / `PARTY` / `GUILD` / `SHOUT`),
   `CHAT_TYPE_INFO`, and `WHISPER` fail closed with:
   - no self echo / info delivery
   - no queued peer chat / whisper delivery
   - no synthetic `WHISPER_TYPE_NOT_EXIST` fallback
3. After `/restart_here` restores live HP, the same owner-side talking chat
   emits ordinary self echo plus queued peer delivery, the same whisper queues
   one exact-name target delivery, and the same info chat emits one self-only
   delivery beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same talking / whisper / info path likewise succeeds against a living
   destination-map peer beside recovered MaxHP and town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorPeerFacingChatFailsClosed`
   - `TestGameSessionFlowPostFloorPeerFacingChatFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific chat packet family
- widening into recipient-skip / notice-skip restart twins in this same commit
- mute / block / party-membership / guild-roster policy
- changing already-owned live chat / whisper routing

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorPeerFacingChatFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_peer_chat_restart_recovery_test.go
git diff --check
```
