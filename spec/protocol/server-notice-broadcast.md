# Server Notice Broadcast

This document freezes the first server-originated bootstrap path for `CHAT_TYPE_NOTICE`.

The goal of this slice is narrow:
- keep client-originated `CHAT_TYPE_NOTICE` rejected in `GAME`
- let the runtime queue a system `GC_CHAT` notice to currently connected `GAME` sessions without a client request
- avoid coupling that notice path to a final operator surface too early

## Covered packet

- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_NOTICE`

## Current runtime behavior

1. one or more players are connected in `GAME`
2. the bootstrap runtime triggers a server-originated notice broadcast programmatically
3. each connected session receives one queued `GC_CHAT` packet with:
   - `type = CHAT_TYPE_NOTICE`
   - `vid = 0`
   - `empire = 0`
   - `message = original notice text`
4. the payload is raw system text, not the actor-formatted `Name : message` shape
5. empty notice text is ignored and queues nothing

## Scope notes

This slice intentionally freezes only the runtime broadcast contract.
It does not yet freeze a final operator surface such as:
- GM commands
- HTTP endpoints
- cron/event scheduling
- external admin authentication

In other words, the runtime now has a real server-originated `NOTICE` path, but the bootstrap project still has not chosen how operators will trigger it in production.

## Current scope

This slice freezes:
- programmatic server-originated `CHAT_TYPE_NOTICE` fanout to connected `GAME` sessions
- system-message payload shape with `vid = 0`
- raw notice text with no `Name : ` prefix
- client-originated `CHAT_TYPE_NOTICE` remaining rejected

It does not yet freeze:
- operator/GM notice tooling
- HTTP/CLI notice triggers
- map/channel-scoped notice policies
- timed, scheduled, or event-driven notice generation
- localization or template systems
