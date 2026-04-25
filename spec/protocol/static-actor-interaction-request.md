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

At this stage the repository owns a narrow but real first response vertical:
- `internal/game` accepts `INTERACT` while the session is already in `GAME`
- the decoded request is dispatched to a dedicated interaction handler
- `internal/worldruntime` can now resolve a bootstrap static actor by that client-visible `VID`
- that runtime lookup is now also visibility-gated, so only actors that currently share visible world with the subject are eligible targets
- `internal/minimal/shared_world` now owns the first validated interaction-attempt seam, returning a structured result for the current subject/target pair before content resolution branches further
- that validated runtime result now distinguishes at least:
  - `subject_not_found`
  - `target_not_visible`
  - `target_has_no_interaction`
- `gamed` now also resolves authored interaction definitions by `interaction_kind + interaction_ref`
- when that definition resolves to `interaction_kind = "info"`, the interacting player now receives one self-only `GC_CHAT` delivery using `CHAT_TYPE_INFO` and the authored definition text
- when that definition resolves to `interaction_kind = "talk"`, the interacting player now receives one self-only chat-backed delivery using a deterministic speaker-prefixed multi-line payload
- malformed payloads are rejected at the codec/flow boundary
- other unsupported interaction kinds may still resolve to no outgoing frames yet

## Target identity rule

The request target is the static actor's current client-visible `VID`.

For bootstrap static actors, that `VID` is currently derived from the runtime `entity_id` when it fits `uint32`.
That keeps the request aligned with the already-owned static-actor visibility bootstrap contract and avoids introducing a second target-identity scheme before real interaction behavior exists.

## Failure semantics

The current owned failure boundary is now explicit and still fail-closed:
- wrong phase -> existing `GAME` flow rejection rules apply
- unexpected header at the codec -> rejected
- malformed payload size -> rejected
- once the request is decoded, the bootstrap runtime can now fail closed as:
  - `subject_not_found`
  - `target_not_visible`
  - `target_has_no_interaction`
  - `interaction_definition_not_found`
  - `unsupported_interaction_kind`
- failed interaction resolution currently produces no outgoing frames
- accepted `info` interaction currently produces exactly one self-only `GC_CHAT` delivery with `CHAT_TYPE_INFO`
- accepted `talk` interaction currently produces exactly one self-only chat-backed delivery whose payload is speaker-prefixed and multi-line

Future slices should freeze richer reporting only when dialog UI or later interaction families exist.

## Success definition

After this slice, the repository should be able to say:
- there is a first owned `GAME`-phase interaction request packet for bootstrap static actors
- the request is decoded deterministically from `target_vid`
- the game flow can dispatch that request to a dedicated interaction handler
- `internal/worldruntime` can resolve a visible bootstrap static actor by that `VID` under the active topology/AOI rules
- `internal/minimal/shared_world` can now turn that subject/target pair into a structured validated interaction attempt before later content resolution exists
- `gamed` can now resolve authored `info` and `talk` definitions behind visible static actors and answer the interacting player with one self-only chat-backed delivery carrying the authored text
- the protocol surface is ready for the next slice to add loopback authoring and richer QA/runtime introspection without redesigning the ingress or target-lookup contract
