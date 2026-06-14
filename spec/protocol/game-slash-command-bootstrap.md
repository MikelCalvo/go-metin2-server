# In-game slash command bootstrap

This document freezes the first owned in-game slash-command behavior for the bootstrap runtime.

The goal is still narrow:
- intercept a tiny safe subset of slash commands typed in the game chat input
- stop echoing those commands back as normal talking chat
- map each supported command to honest bootstrap behavior already owned by this repository

## Covered client input

The current bootstrap runtime claims support for these slash commands when the session is already in `GAME`:
- `/quit`
- `/logout`
- `/phase_select`
- `/restart_here`
- `/restart_town`

Any other slash-prefixed text remains outside the scope of this slice.

## Root behavior difference from normal chat

Normal talking chat still uses `CLIENT_CHAT` and may be echoed/fanned out as visible talking chat.

The supported slash commands are intercepted before normal talking-chat delivery.
They are **not** echoed back as `CHAT_TYPE_TALKING`.

The current parser is deliberately exact for the owned command family:
- the input must contain exactly one slash command token
- `/quit`, `/logout`, `/phase_select`, `/restart_here`, and `/restart_town` are accepted as standalone tokens only
- argument-bearing forms such as `/restart_town 2` or `/quit now` stay outside the owned slash-command ingress and fall through to the ordinary chat policy instead of partially executing the leading token

## Current owned behavior

### `/quit`
- accepted only while the session is in `GAME`
- the session first leaves the current bootstrap shared-world registration
- the selected runtime pointer plus active merchant/combat/live-registration state are cleared immediately, so the socket stops owning live world gameplay state before the client disconnects
- returns one self-facing `CHAT_TYPE_COMMAND` delivery with message:
  - `quit`
- the server remains in `GAME`
- the bootstrap contract still relies on the client to act on that command and terminate/leave on its side; this slice only adds the earlier server-side teardown boundary before that client disconnect finishes

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

### `/restart_here`
- accepted only while the session is in `GAME`, still owns a live shared-world player entry, and the selected live runtime is already at the current retaliation-owned `0`-HP floor
- otherwise fails closed with no normal chat echo
- keeps the session in `GAME`
- rebuilds the selected live runtime from the persisted account snapshot while preserving the current in-world position
- returns the ordinary selected-character bootstrap burst on the same socket
- requires a fresh later `TARGET` before combat can resume

### `/restart_town`
- accepted only while the session is in `GAME`, still owns a live shared-world player entry, and the selected live runtime is already at the current retaliation-owned `0`-HP floor
- otherwise fails closed with no normal chat echo
- keeps the session in `GAME`
- rebuilds the selected live runtime from the persisted account snapshot at the owned legacy empire town-return target
- persists that town-return position before runtime commit, then reuses the existing transfer rebootstrap burst plus ordinary visibility deltas on the same socket
- requires a fresh later `TARGET` before combat can resume

For the current compatibility track, `/restart_here` and `/restart_town` are not temporary placeholders waiting for a guessed dedicated packet. They are the owned restart ingress unless later captures or owned fixtures prove an additional non-chat request family.

## Non-goals

This slice does **not** yet claim:
- the full legacy timed countdown behavior before quit/logout/select
- support for arbitrary slash commands
- GM/admin command parsing
- a complete legacy `interpret_command(...)` clone
- any separate non-chat revive packet or revive-menu flow; the restart-ingress follow-up note in `player-restart-request-bootstrap.md` now keeps that path unowned unless later captures or owned fixtures prove it with exact bytes
- a complete legacy death/respawn system beyond the currently owned `/restart_here` and `/restart_town` bootstrap recovery seams
- any claim that `PHASE(SELECT)` rebootstrap is compatibility-complete beyond the current client path we own and test

## Why this slice exists

In the legacy server, slash-prefixed chat input is intercepted before normal chat fanout and routed into command handling.

Before this owned bootstrap family existed, the clean-room runtime treated slash command text as ordinary talking chat, so the user saw those strings echoed into the in-game chat window instead of triggering the expected behavior.

This slice fixes that specific owned gap without pretending the whole legacy command system exists yet.
