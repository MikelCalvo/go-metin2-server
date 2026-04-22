# Server Notice Broadcast

This document freezes the first server-originated bootstrap path for `CHAT_TYPE_NOTICE`, including its initial local-only operator exposure.

The goal of this slice is narrow:
- keep client-originated `CHAT_TYPE_NOTICE` rejected in `GAME`
- let the runtime queue a system `GC_CHAT` notice to currently connected `GAME` sessions without a client request
- expose that path through a loopback-only `gamed` ops endpoint without opening a general remote admin surface

## Covered packet

- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_NOTICE`

## Current runtime behavior

1. one or more players are connected in `GAME`
2. an on-box operator issues `POST /local/notice` against the `gamed` ops server, or another local runtime caller triggers the same broadcast primitive directly
3. each connected session receives one queued `GC_CHAT` packet with:
   - `type = CHAT_TYPE_NOTICE`
   - `vid = 0`
   - `empire = 0`
   - `message = original notice text`
4. the payload is raw system text, not the actor-formatted `Name : message` shape
5. empty notice text is ignored and queues nothing

## Local-only endpoint contract

Path:
- `POST /local/notice`

Access policy:
- accepted only when the HTTP remote address is loopback
- intended for on-box operator use
- not registered on `authd`

Request body:
- raw text notice payload

Success response:
- plain text: `queued N\n`

Error behavior:
- non-loopback caller -> `403 Forbidden`
- empty body after trimming -> `400 Bad Request`
- wrong method -> `405 Method Not Allowed`

## Scope notes

The runtime still exposes a direct broadcast primitive, but the first operator-facing surface is now frozen too:
- `gamed` local-only ops endpoint

This is intentionally still conservative.
The project does not yet freeze broader operator/admin surfaces such as:
- GM chat commands
- remote admin HTTP APIs
- cron/event scheduling
- external admin authentication

## Current scope

This slice freezes:
- server-originated `CHAT_TYPE_NOTICE` fanout to connected `GAME` sessions
- the `gamed` loopback-only `POST /local/notice` trigger surface
- system-message payload shape with `vid = 0`
- raw notice text with no `Name : ` prefix
- client-originated `CHAT_TYPE_NOTICE` remaining rejected
- runtime-owned connected-target selection through `internal/worldruntime.Scopes.ConnectedTargets()`, while keeping the current bootstrap-global notice policy unchanged

It does not yet freeze:
- GM/operator notice tooling beyond the local-only ops endpoint
- remote admin HTTP/CLI notice triggers
- map/channel-scoped notice policies
- timed, scheduled, or event-driven notice generation
- localization or template systems
