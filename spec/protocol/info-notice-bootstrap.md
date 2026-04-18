# Info and Notice Bootstrap

This document freezes the first minimal bootstrap behavior for `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE`.

These chat types commonly behave as server-originated system messages in the compatibility target.
The current bootstrap runtime intentionally exposes them through the existing `CHAT` request path so the project can freeze deterministic wire behavior without adding separate event, GM, or operator-trigger systems yet.

## Covered packets

- `CHAT` client -> server (`0x0601`) with `type = CHAT_TYPE_INFO`
- `CHAT` client -> server (`0x0601`) with `type = CHAT_TYPE_NOTICE`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_INFO`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_NOTICE`

## Bootstrap behavior

### `CHAT_TYPE_INFO`

Current runtime behavior:

1. player A is connected in `GAME`
2. player A sends `CHAT` with `type = CHAT_TYPE_INFO`
3. the server returns one deterministic `GC_CHAT` packet with:
   - `type = CHAT_TYPE_INFO`
   - `vid = 0`
   - `empire = 0`
   - `message = original message`
4. no peer fanout occurs

This freezes `CHAT_TYPE_INFO` as a bootstrap system/self channel.

### `CHAT_TYPE_NOTICE`

Current runtime behavior:

1. player A and player B are connected in `GAME`
2. player A sends `CHAT` with `type = CHAT_TYPE_NOTICE`
3. the server builds one deterministic `GC_CHAT` packet with:
   - `type = CHAT_TYPE_NOTICE`
   - `vid = 0`
   - `empire = 0`
   - `message = original message`
4. player A receives that packet directly
5. player B receives the same packet through the queued server-frame path

This freezes `CHAT_TYPE_NOTICE` as a bootstrap system/broadcast channel.

## Scope notes

This slice intentionally keeps the payload as a raw system message with `vid = 0` instead of the actor-formatted `Name : message` shape used by talking/party/guild/shout.

That is deliberate.
It matches the current bootstrap goal of exercising system-message rendering paths separately from actor chat tails.

## Current scope

This slice freezes:
- `CHAT_TYPE_INFO` acceptance in `GAME`
- `CHAT_TYPE_NOTICE` acceptance in `GAME`
- `vid = 0` for both bootstrap system-message deliveries
- raw message passthrough with no `Name : ` prefix
- sender-only behavior for bootstrap `INFO`
- sender + queued peer fanout for bootstrap `NOTICE`

It does not yet freeze:
- real event-driven server info messages
- GM/operator notice tooling
- timed or scheduled notices
- localization/event pipelines
- any permission model around who may trigger a bootstrap notice request
