# Guild Chat Bootstrap

This document freezes the first minimal guild-chat behavior for the bootstrap runtime.

The goal of this slice is narrow:
- reuse the existing `CHAT` / `GC_CHAT` packet family
- accept `CHAT_TYPE_GUILD` in `GAME`
- echo one deterministic `GC_CHAT` guild delivery back to the sender
- queue the same `GC_CHAT` guild delivery to the other connected bootstrap sessions with the same non-zero `GuildID`
- avoid broadening the slice into real guild create/join/leave/member/rank/state semantics yet

## Covered packets

- `CHAT` client -> server (`0x0601`) with `type = CHAT_TYPE_GUILD`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_GUILD`

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are connected in `GAME` and both characters have the same non-zero `GuildID`
2. player B sends `CHAT` with `type = CHAT_TYPE_GUILD`
3. the server builds one deterministic delivery payload with:
   - `type = CHAT_TYPE_GUILD`
   - `vid = player B vid`
   - `empire = 0`
   - `message = "PlayerName : original message"`
4. player B receives that `GC_CHAT` delivery directly as the sender echo
5. player A receives the same `GC_CHAT` delivery through the queued server-frame path
6. peers in other guilds do not receive that delivery

## Bootstrap simplification

This slice now uses the currently available persisted `GuildID` as its first real guild scope boundary.

That is still a bootstrap policy only.
It is not yet a claim that real guild create/join/leave/member/state semantics already exist.

## Current scope

This slice freezes:
- `CHAT_TYPE_GUILD` acceptance in `GAME`
- sender echo plus queued fanout to the other connected bootstrap sessions with the same non-zero `GuildID`
- no queued fanout to peers in other guilds
- reuse of the same `GC_CHAT` payload shape already used for local chat
- `Name : message` formatting in the payload text

It does not yet freeze:
- real guild membership
- guild create/join/leave flows
- guild rank/grade/state packets
- guild war or territory behavior
- guild notice/member synchronization
- guild storage or economy behavior
