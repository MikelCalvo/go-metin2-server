# Local Chat Peer Fanout

This document freezes the first minimal local chat behavior for players that are already mutually visible in the bootstrap runtime.

The goal of this slice is narrow:
- accept one minimal `CHAT` client packet in `GAME`
- keep the sender path deterministic by echoing one `GC_CHAT` delivery back to the sender
- queue the same `GC_CHAT` delivery to already-visible peers
- avoid broadening the slice into party chat, guild chat, whisper, shout, moderation, or command handling

## Covered packets

- `CHAT` client -> server (`0x0601`)
- `CHAT` server -> client (`0x0603`)

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are already visible to each other in `GAME`
2. player B sends `CHAT` with type `CHAT_TYPE_TALKING`
3. the server builds one deterministic delivery payload with:
   - `type = CHAT_TYPE_TALKING`
   - `vid = player B vid`
   - `empire = 0`
   - `message = "PlayerName : original message"`
4. player B receives that `GC_CHAT` delivery directly as the sender echo
5. player A receives the same `GC_CHAT` delivery through the queued server-frame path

## Current scope

This slice freezes:
- local talking chat only
- sender echo plus queued peer fanout
- reuse of the same `GC_CHAT` payload for sender and already-visible peers
- `Name : message` formatting in the payload text

It does not yet freeze:
- party chat
- guild chat
- whisper
- shout
- slash-command handling
- mute/block/spam systems
- range/sector filtering beyond the current shared-world visibility scope
