# Party Chat Bootstrap

This document freezes the first minimal party-chat behavior for the bootstrap runtime.

The goal of this slice is narrow:
- reuse the existing `CHAT` / `GC_CHAT` packet family
- accept `CHAT_TYPE_PARTY` in `GAME`
- echo one deterministic `GC_CHAT` party delivery back to the sender
- queue the same `GC_CHAT` party delivery to the other connected bootstrap sessions
- avoid broadening the slice into real party invite/add/remove/link/update semantics yet

## Covered packets

- `CHAT` client -> server (`0x0601`) with `type = CHAT_TYPE_PARTY`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_PARTY`

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are connected in `GAME`
2. player B sends `CHAT` with `type = CHAT_TYPE_PARTY`
3. the server builds one deterministic delivery payload with:
   - `type = CHAT_TYPE_PARTY`
   - `vid = player B vid`
   - `empire = 0`
   - `message = "PlayerName : original message"`
4. player B receives that `GC_CHAT` delivery directly as the sender echo
5. player A receives the same `GC_CHAT` delivery through the queued server-frame path

## Bootstrap simplification

This slice intentionally treats all currently connected `GAME` sessions in the bootstrap runtime as one implicit temporary party.

That is a temporary bootstrap policy only.
It is not a claim that real party membership/invite/link semantics already exist.

## Current scope

This slice freezes:
- `CHAT_TYPE_PARTY` acceptance in `GAME`
- sender echo plus queued fanout to the other connected bootstrap sessions
- reuse of the same `GC_CHAT` payload shape already used for local chat
- `Name : message` formatting in the payload text
- runtime-owned target selection through `internal/worldruntime.Scopes.PartyTargets(...)`, while keeping the current bootstrap-global policy unchanged

It does not yet freeze:
- real party membership
- party invites
- party add/remove/link/unlink packets
- party parameter/state packets
- party skill behavior
- party UI/member synchronization
