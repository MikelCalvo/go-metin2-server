# Session phases

This document defines the working session model for the initial TMP4-compatible boot path.

The names here describe project-owned behavior.
Exact phase byte values should be locked by tests and captures once the first control-packet implementation lands.

## Phase list

Current working phase-byte mapping for the initial target:

- `CLOSE` = `0x00`
- `HANDSHAKE` = `0x01`
- `LOGIN` = `0x02`
- `SELECT` = `0x03`
- `LOADING` = `0x04`
- `GAME` = `0x05`
- `DEAD` = `0x06`
- `AUTH` = `0x0A`

These values should be treated as compatibility data and frozen by automated tests.

## 1. `HANDSHAKE`

Purpose:
- establish the connection
- synchronize control-plane expectations
- complete the key challenge / response exchange if required by the compatibility target

Typical traffic in this phase:
- server control packets such as `PHASE`, `PING`, `KEY_CHALLENGE`, `KEY_COMPLETE`
- client control packets such as `PONG` and `KEY_RESPONSE`

Exit condition:
- the connection is ready to authenticate
- depending on the service role, the next phase is either `AUTH` or `LOGIN`

## 2. `AUTH`

Purpose:
- authenticate credentials on the auth server
- issue a login key that the client can present to `gamed`

Typical traffic in this phase:
- `LOGIN3`
- `LOGIN_FAILURE`
- `AUTH_SUCCESS`

Exit condition:
- success -> client disconnects and reconnects to `gamed` with `LOGIN2`
- failure -> stay in `AUTH` or close, depending on the final policy we standardize

## 3. `LOGIN`

Purpose:
- authenticate the account/session
- exchange any login key or secure login material needed by the target client

Typical traffic in this phase:
- login request packet(s)
- login key packet(s)
- login success or failure packet(s)

Exit condition:
- success -> `SELECT`
- failure -> close or stay in login, depending on the final behavior we standardize

## 3. `SELECT`

Purpose:
- expose the character selection surface
- support empire selection if needed
- support character creation
- support character selection

Typical traffic in this phase:
- empire packets
- character list / login success payloads
- create character request and result
- character select request

Exit condition:
- selected character accepted -> `LOADING`

## 4. `LOADING`

Purpose:
- bootstrap the chosen character
- push the minimum world state needed before the live game phase starts

Typical traffic in this phase:
- main character bootstrap packet(s)
- player points / stats
- `CLIENT_VERSION` metadata from the client
- optional time/channel/world bootstrap data
- enter-game request from the client

Exit condition:
- client has acknowledged the loading step and entered the live world -> `GAME`

## 5. `GAME`

Purpose:
- normal in-world interaction

Initial milestone scope:
- main actor present
- basic movement
- no broader gameplay systems required yet

Typical traffic in this phase:
- move
- sync position
- control-plane `PING`/`PONG` that should not disturb the live phase
- visible peer `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` / `CHARACTER_DEL`
- minimal world updates

## 6. `CLOSE`

Purpose:
- terminal state for disconnect, protocol failure, or shutdown

## Allowed transitions

The working transition graph is:

- `HANDSHAKE -> AUTH`
- `HANDSHAKE -> LOGIN`
- `LOGIN -> SELECT`
- `SELECT -> LOADING`
- `LOADING -> GAME`
- `GAME -> CLOSE`
- any phase -> `CLOSE` on unrecoverable error

The server should reject packets that belong to a different phase.
A packet decoder may still parse them, but the session layer must not process them as valid behavior.

## Phase invariants

### `HANDSHAKE`
- no gameplay packets are accepted
- no character selection packets are accepted
- the session has not been authenticated yet

### `AUTH`
- only auth-server credential packets are valid here
- no selection or in-world packets are accepted
- a successful auth result should issue a login key, not enter the world directly

### `LOGIN`
- the session is not yet bound to a selected character
- character creation and enter-game packets are invalid here

### `SELECT`
- authentication is already complete
- character creation is allowed
- character deletion is allowed
- selection is allowed
- no in-world movement is allowed

### `LOADING`
- a concrete character choice already exists
- world bootstrap is in progress
- `CLIENT_VERSION` metadata may be accepted here without changing phase
- arbitrary in-world actions are still invalid

### `GAME`
- a character is bound to the connection
- movement is allowed
- control-plane `PONG` may be accepted here without changing phase
- later milestones may enable additional systems here

## Notes for implementation

The phase model should live in the session/state layer, not inside ad-hoc packet handlers.
That keeps the rules testable and prevents phase drift across features.
