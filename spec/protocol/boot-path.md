# Boot path

This document describes the first functional milestone for the server.

The milestone is complete when a compatible client can:
- connect
- pass the control-plane handshake
- authenticate
- see the character selection surface
- create a character
- select a character
- enter the world
- move the main character
- reconcile the main character position with a first `SYNC_POSITION` round-trip

## Target scope

The boot path is intentionally narrow.
It exists to prove protocol compatibility and session management before deeper gameplay work begins.

In scope:
- connection bootstrap
- handshake
- login/auth
- character list
- empire selection if required
- create character
- select character
- loading/bootstrap
- enter game
- basic movement
- first self-only sync-position reconciliation

Out of scope for this milestone:
- combat
- inventory systems beyond what the client absolutely requires to enter
- quests
- shops
- guilds
- parties
- dungeons
- multi-channel behavior
- advanced persistence rules

## Ordered flow

## 1. TCP connection opens
The server accepts the client connection and creates a new session in `HANDSHAKE`.

## 2. Control-plane handshake completes
The session exchanges the minimum control packets required to leave the initial handshake phase.

Expected outcomes:
- the client accepts the session
- the server can safely transition to `LOGIN`

## 3. Login/auth succeeds
The client submits authentication material.
The server validates it and returns the success path required by the compatibility target.

Expected outcomes:
- the session becomes authenticated
- the client reaches the selection surface

## 4. Character selection data is available
The client receives the data needed to populate character selection.
If the account has no characters yet, empire selection or creation support must still work correctly.

Expected outcomes:
- the client can display the selection screen
- the client can issue create/select actions without desynchronizing the session

## 5. Character creation works
The client can create a valid character in an empty slot.
The response path must update the selection state cleanly.

Expected outcomes:
- the new character appears in the selection surface
- duplicate names or invalid slots are rejected predictably

## 6. Character selection works
The client selects one character and the server binds that choice to the session.
The session then transitions to `LOADING`.

Expected outcomes:
- the server prepares main-character bootstrap data
- the client proceeds toward world entry

## 7. Loading/bootstrap completes
The server sends the minimum required bootstrap data for the selected character.
This typically includes the main character and player points, plus any other mandatory world bootstrap packets.

Expected outcomes:
- the client accepts the selected character context
- the client can issue the enter-game action

## 8. Enter game succeeds
The client sends the enter-game request.
The server transitions the session into `GAME`, places the main actor in a minimal world state, and emits the first visible-world bootstrap packets for the selected character.

Expected outcomes:
- the client appears in-world
- the server keeps the session stable after the transition
- the selected character is inserted into the visible world with deterministic bootstrap packets

## 9. Basic movement works
The client sends a movement packet and the server processes it in the live game phase.

Expected outcomes:
- the server updates the session-scoped selected character position
- the server emits one deterministic movement replication/ack packet using the selected character VID
- the client remains connected
- the movement path becomes the first in-world behavior covered by tests

## 10. First sync-position reconciliation works
The client sends a minimal `SYNC_POSITION` packet after entering `GAME` and the server reconciles the selected character without dropping the session.

Expected outcomes:
- the server accepts a self-only sync-position payload for the selected character VID
- the server updates the selected character coordinates in the bootstrap runtime
- the server emits one deterministic `SYNC_POSITION` reply for the selected character
- the client remains connected after the reconciliation path

## Milestone acceptance criteria

The boot-path milestone is done only when:
- the flow above is documented in repo-owned docs
- the flow is covered by automated tests at the packet and socket levels
- the real target client can complete the path without ad-hoc manual hacks
- the server exposes enough logging and profiling to debug failures quickly

## Required supporting docs

This document depends on:
- `spec/protocol/session-phases.md`
- `spec/protocol/frame-layout.md`
- `spec/protocol/packet-matrix.md`
- `spec/protocol/sync-position-bootstrap.md`
- `docs/testing-strategy.md`
