# Client game-entry sequence and minimum bootstrap

This document freezes the client-side expectations from character selection through stable in-world presence.

It is broader than the current narrow bootstrap-burst spec: it records the phase choreography and the minimum packets that make the client feel truly connected rather than merely past login.

## Scope

Covered path:
- selection success
- `PHASE(LOADING)`
- `MAIN_CHARACTER`
- `PLAYER_POINTS`
- client `CLIENT_VERSION`
- client `ENTERGAME`
- `PHASE(GAME)`
- minimum self bootstrap for stable spawn
- first early movement and sync expectations

## High-level sequence

A realistic client-compatible path is:

1. client sends `CHARACTER_SELECT`
2. server moves session to `LOADING`
3. server emits:
   - `PHASE(LOADING)`
   - `MAIN_CHARACTER`
   - initial point/status payload
4. client processes `MAIN_CHARACTER`
5. client sends `CLIENT_VERSION`
6. client later sends `ENTERGAME`
7. server moves session to `GAME`
8. server emits:
   - `PHASE(GAME)`
   - self bootstrap frames
   - then any trailing visible-peer frames

## Why `LOADING` matters

The client loading path is not just a cosmetic pause.
It is where the main actor identity and initial position context are established.

Observed implications:
- `MAIN_CHARACTER` is the packet that seeds main actor identity
- the client uses it to populate main actor metadata and initiate the loading window path
- `CLIENT_VERSION` is sent during this stage, not after full in-world bootstrap

## `CLIENT_VERSION` handling

The client sends `CLIENT_VERSION` during `LOADING` after processing `MAIN_CHARACTER`.

Practical server rule:
- accept it during `LOADING`
- treat it as metadata unless a stricter policy is intentionally added later
- do not let it block `ENTERGAME` unless that policy is explicitly documented and tested

## `ENTERGAME` is the real phase-advance trigger

The client does not automatically become in-world just because it received `MAIN_CHARACTER`.
The server still needs the explicit `ENTERGAME` request.

Practical server rule:
- keep `ENTERGAME` as the client-owned trigger for `LOADING -> GAME`
- do not merge loading completion and game entry into one implicit server-only step

## Secure transport continuity on the game socket

On the secure legacy game-socket path, `KEY_COMPLETE` does not end the main game-socket choreography.
It flips that same socket into encrypted post-handshake traffic.

Practical server rule:
- after the secure handshake completes, the same game socket continues carrying encrypted `LOGIN2`
- the same encrypted post-handshake path also carries `CHARACTER_SELECT`, `CLIENT_VERSION`, and `ENTERGAME`
- the server responses for `LOGIN_SUCCESS4`, `EMPIRE`, `PHASE(SELECT)`, `PHASE(LOADING)`, `MAIN_CHARACTER`, `PLAYER_POINTS`, `PHASE(GAME)`, and the self bootstrap burst stay on that encrypted game-socket path

This matters because the first honest "client standing in world" milestone is not only about packet order.
It is about proving that the selection/loading/game-entry choreography survives intact after secure transport activation.

## Minimum self bootstrap after `PHASE(GAME)`

For a stable self presence, the client expects more than a single "you are in game" signal.

A practical minimum burst is:

1. `PHASE(GAME)`
2. `CHARACTER_ADD` for the selected character
3. `CHAR_ADDITIONAL_INFO` for the same `vid`
4. `CHARACTER_UPDATE` for the same `vid`
5. `PLAYER_POINT_CHANGE` for the same character

Why this matters:
- the client does not treat every actor packet independently
- for player-like actors, insertion and final visibility depend on the expected companion metadata arriving in the right family of packets

## Peer frames should come after self bootstrap

If other visible peers already exist, append their bootstrap frames after the selected-character burst.

Practical server rule:
- self bootstrap first
- peer bootstrap second

This keeps the selected character deterministic and reduces ambiguity when reading traces.

## Loading is more permissive than a naive model suggests

The client loading dispatch table inherits the game handlers and then overrides the loading-specific pieces.

Practical meaning:
- some game-family packets may already be tolerated during `LOADING`
- but the implementation should still prefer a clean deterministic choreography rather than depending on this tolerance

## Early in-world control expectations

### Ping/pong

The client expects `PING`/`PONG` behavior to continue once in world.

Practical server rule:
- `PING`/`PONG` should be treated as part of the normal live session contract, not a one-off handshake concern only

### Move

The client emits `MOVE` in-game with:
- movement function/state
- orientation
- position
- time information derived from the synchronized server-time model

Practical server rule:
- the Go server should keep `MOVE` parsing and acknowledgement deterministic
- treat movement timestamps as meaningful compatibility data rather than decorative payload

### Sync position

The client can also emit `SYNC_POSITION` for correction/reconciliation paths.

Practical server rule:
- keep sync handling as part of the early MVP surface, not a late "polish" feature
- even a minimal server should parse it and respond deterministically

## Coordinate and time notes

Observed client behavior suggests:
- movement packets use global/runtime coordinates on the wire
- time synchronization is tied to server-provided time from control packets

Practical rule:
- movement and sync slices must stay consistent with the server-time model used by control packets

## Rejected assumptions

1. Passing login is close enough to being in world.
2. `MAIN_CHARACTER` alone is enough for a stable self spawn.
3. `ENTERGAME` can be skipped or silently implied.
4. movement can be deferred until long after world-entry compatibility is done.

## Implementation guidance for future Go slices

### Freeze separately
- `CHARACTER_SELECT -> PHASE(LOADING)`
- `MAIN_CHARACTER` + initial points path
- tolerant `CLIENT_VERSION` ingestion in `LOADING`
- `ENTERGAME -> PHASE(GAME)`
- self bootstrap burst
- peer bootstrap append rule
- minimal `PING/PONG`, `MOVE`, and `SYNC_POSITION`

### Minimum milestone definition

Do not call the client "connected to game" unless all of these are visible in traces:
- secure handshake complete on the game socket
- `PHASE(LOGIN)`
- `LOGIN2` accepted
- `PHASE(LOADING)`
- `MAIN_CHARACTER`
- client `CLIENT_VERSION`
- client `ENTERGAME`
- `PHASE(GAME)`
- self bootstrap burst

That is the first honest "standing in world" milestone.

## Suggested next slices

1. Freeze the exact `LOADING` transcript including `CLIENT_VERSION`.
2. Freeze the `ENTERGAME` transition and self bootstrap burst in one TCP E2E test.
3. Freeze movement and sync as the first post-spawn gameplay contract.
