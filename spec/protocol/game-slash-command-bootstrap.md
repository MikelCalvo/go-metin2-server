# In-game slash command bootstrap

This document freezes the first owned in-game slash-command behavior for the bootstrap runtime.

The goal is narrow:
- intercept a tiny safe subset of slash commands typed in the game chat input
- stop echoing those commands back as normal talking chat
- map each supported command to honest bootstrap behavior already owned by this repository

## Covered client input

The current bootstrap runtime only claims support for these slash commands when the session is already in `GAME`:
- `/quit`
- `/logout`
- `/phase_select`

Any other slash-prefixed text remains outside the scope of this slice.

## Root behavior difference from normal chat

Normal talking chat still uses `CLIENT_CHAT` and may be echoed/fanned out as visible talking chat.

The supported slash commands are intercepted before normal talking-chat delivery.
They are **not** echoed back as `CHAT_TYPE_TALKING`.

## Current owned behavior

### `/quit`
- accepted only while the session is in `GAME`
- returns one self-facing `CHAT_TYPE_COMMAND` delivery with message:
  - `quit`
- the server remains in `GAME`
- the bootstrap contract relies on the client to act on that command and terminate/leave on its side

### `/logout`
- accepted only while the session is in `GAME`
- the session first leaves the current bootstrap shared-world registration
- the server then transitions the socket phase to `CLOSE`
- the server emits:
  - `PHASE(CLOSE)`
- no normal chat echo is returned

### `/phase_select`
- accepted only while the session is in `GAME`
- the session first leaves the current bootstrap shared-world registration
- the selected runtime pointer is cleared so the same socket can select another character again
- the server then transitions the socket phase back to `SELECT`
- the server emits:
  - `PHASE(SELECT)`
- no full login replay is claimed in this slice; the bootstrap contract assumes the client already has the selection surface context it needs from the earlier login/select flow

## Non-goals

This slice does **not** yet claim:
- the full legacy timed countdown behavior before quit/logout/select
- support for arbitrary slash commands
- GM/admin command parsing
- a complete legacy `interpret_command(...)` clone
- any claim that `PHASE(SELECT)` rebootstrap is compatibility-complete beyond the current client path we own and test

## Why this slice exists

In the legacy server, slash-prefixed chat input is intercepted before normal chat fanout and routed into command handling.

Before this slice, the clean-room bootstrap runtime treated:
- `/quit`
- `/logout`
- `/phase_select`

as ordinary talking chat, so the user saw those strings echoed into the in-game chat window instead of triggering the expected behavior.

This slice fixes that specific owned gap without pretending the whole legacy command system exists yet.
