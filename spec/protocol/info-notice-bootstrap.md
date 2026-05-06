# Info and Notice Bootstrap

This document freezes the current bootstrap behavior for `CHAT_TYPE_INFO` and the client-originated side of `CHAT_TYPE_NOTICE`.

These chat types commonly behave as server-originated system messages in the compatibility target.
The current bootstrap runtime keeps `CHAT_TYPE_INFO` exposed through the existing `CHAT` request path for deterministic testing, while client-originated `CHAT_TYPE_NOTICE` remains rejected.
The later zero-HP owner rules in `player-death-bootstrap.md` now narrow that `INFO` acceptance further: once practice-mob retaliation has already driven the engaged owner's live bootstrap HP to `0`, client-originated `CHAT_TYPE_INFO` also fails closed there.
The actual server-originated notice path now lives in `server-notice-broadcast.md`.

## Covered packets

- `CHAT` client -> server (`0x0601`) with `type = CHAT_TYPE_INFO`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_INFO`
- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_NOTICE`

## Bootstrap behavior

### `CHAT_TYPE_INFO`

Current runtime behavior outside that later zero-HP death-floor carve-out:

1. player A is connected in `GAME`
2. player A sends `CHAT` with `type = CHAT_TYPE_INFO`
3. the server returns one deterministic `GC_CHAT` packet with:
   - `type = CHAT_TYPE_INFO`
   - `vid = 0`
   - `empire = 0`
   - `message = original message`
4. no peer fanout occurs

If the same selected owner session has already reached the practice-mob retaliation `0`-HP floor documented in `player-death-bootstrap.md`, that same client-originated `CHAT_TYPE_INFO` request now fails closed instead with no self `GC_CHAT` delivery.

This freezes `CHAT_TYPE_INFO` as a bootstrap system/self channel until a later zero-HP owner rule narrows it.

### `CHAT_TYPE_NOTICE`

Current runtime behavior:

1. player A sends `CHAT` with `type = CHAT_TYPE_NOTICE`
2. the bootstrap runtime rejects that client-originated request
3. no direct sender frame is returned
4. no queued peer fanout occurs

This freezes `CHAT_TYPE_NOTICE` as reserved for a future server-originated bootstrap notice path, not as a client-triggered broadcast channel.

## Scope notes

This slice intentionally keeps the payload as a raw system message with `vid = 0` instead of the actor-formatted `Name : message` shape used by talking/party/guild/shout.

That is deliberate.
It matches the current bootstrap goal of exercising system-message rendering paths separately from actor chat tails.

## Current scope

This slice freezes:
- `CHAT_TYPE_INFO` acceptance in `GAME` while the selected owner has not already been driven to the practice-mob retaliation `0`-HP floor documented later in `player-death-bootstrap.md`
- client-originated `CHAT_TYPE_NOTICE` rejection in `GAME`
- `vid = 0` for bootstrap `INFO` system-message delivery
- raw message passthrough with no `Name : ` prefix
- sender-only behavior for bootstrap `INFO`
- separation between client-originated `INFO` handling here and server-originated notice broadcasting documented separately

It does not yet freeze:
- real event-driven server info messages
- operator/GM notice tooling
- timed or scheduled notices
- localization/event pipelines
- any permission model around who may trigger a notice
