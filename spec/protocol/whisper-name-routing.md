# Whisper Name Routing

This document freezes the first minimal whisper behavior for the bootstrap runtime.

The goal of this slice is narrow:
- accept one minimal `WHISPER` client packet in `GAME`
- route the whisper by exact target character name among currently connected bootstrap sessions
- deliver one `GC_WHISPER` packet only to the target on success
- return one `WHISPER_TYPE_NOT_EXIST` packet to the sender when the target is not connected
- avoid broadening the slice into block lists, cross-channel relay, empire filtering, GM/system whisper variants, or moderation

## Covered packets

- `WHISPER` client -> server (`0x0602`)
- `WHISPER` server -> client (`0x0604`)

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are connected in `GAME`
2. player B sends `WHISPER` targeting `PeerOne`
3. the server looks up the target by exact character name in the current shared-world runtime
4. on success, player A receives one `GC_WHISPER` packet with:
   - `type = WHISPER_TYPE_CHAT`
   - `from_name = player B name`
   - `message = original whisper text`
5. player B receives no direct echo on success
6. if the target name is not present, player B receives one `GC_WHISPER` packet with:
   - `type = WHISPER_TYPE_NOT_EXIST`
   - `from_name = requested target name`
   - empty message payload

## Current scope

This slice freezes:
- exact-name whisper routing among currently connected `GAME` sessions
- successful direct delivery only to the target
- `WHISPER_TYPE_NOT_EXIST` sender feedback for unknown targets
- no sender echo on successful whisper delivery

It does not yet freeze:
- target/sender blocked responses
- GM whisper type
- system/error whisper text payloads beyond not-found
- cross-channel relay
- offline messaging
- case-folding or locale-specific name matching
- slash-command behavior
