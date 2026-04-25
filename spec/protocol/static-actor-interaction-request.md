# Static Actor Interaction Request

This document freezes the first client-originated interaction ingress seam for bootstrap static actors.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `client-game-entry-sequence.md`

The goal is intentionally narrow:
- let a `GAME` session ask to interact with one currently visible bootstrap static actor
- identify that target using the same client-visible `VID` already used in static-actor visibility bootstrap
- route the request through the owned game-flow dispatch boundary before any real `info` / `talk` behavior exists

## Scope

This contract currently applies only to:
- client -> server interaction requests on the main game socket while already in `GAME`
- bootstrap static actors that were already made visible through the existing static-actor visibility contract
- one target field: the actor's client-visible `VID`
- the game-flow dispatch seam in `internal/game`

It does **not** yet claim:
- any guaranteed response packet family
- NPC dialog windows
- shops, quests, combat, or pathing
- target selection persistence
- click-to-move or range/path validation beyond existing visibility ownership

## Packet

The first owned request is:

- name: `INTERACT`
- direction: client -> server
- header: `0x0501`
- phase: `GAME`
- payload: little-endian `uint32 target_vid`

Owned Go codec boundary:
- `internal/proto/interact`
  - `RequestPacket { target_vid }`
  - `EncodeRequest(...)`
  - `DecodeRequest(...)`

## Current owned behavior

At this stage the repository owns only the request ingress seam:
- `internal/game` accepts `INTERACT` while the session is already in `GAME`
- the decoded request is dispatched to a dedicated interaction handler
- malformed payloads are rejected at the codec/flow boundary
- unsupported/unhandled interactions may still resolve to no outgoing frames

This slice does **not** yet claim what the server does with a valid request after dispatch.

## Target identity rule

The request target is the static actor's current client-visible `VID`.

For bootstrap static actors, that `VID` is currently derived from the runtime `entity_id` when it fits `uint32`.
That keeps the request aligned with the already-owned static-actor visibility bootstrap contract and avoids introducing a second target-identity scheme before real interaction behavior exists.

## Failure semantics

The current owned failure boundary is narrow:
- wrong phase -> existing `GAME` flow rejection rules apply
- unexpected header at the codec -> rejected
- malformed payload size -> rejected
- accepted request with no implemented behavior -> may legally produce no outgoing frames yet

Future slices should freeze richer failure/reporting semantics only when the runtime actually resolves visible targets and authored `info` / `talk` content.

## Success definition

After this slice, the repository should be able to say:
- there is a first owned `GAME`-phase interaction request packet for bootstrap static actors
- the request is decoded deterministically from `target_vid`
- the game flow can dispatch that request to a dedicated interaction handler
- the protocol surface is ready for the next slice to resolve visible static actors by `VID` without redesigning the ingress contract
