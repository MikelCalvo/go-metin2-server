# Static Actor Interaction Request

This document freezes the first client-originated interaction ingress seam for bootstrap static actors.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `client-game-entry-sequence.md`

The goal is intentionally narrow:
- let a `GAME` session ask to interact with one currently visible bootstrap static actor
- identify that target using the same client-visible `VID` already used in static-actor visibility bootstrap
- route the request through the owned game-flow dispatch boundary before richer NPC gameplay such as service interactions, dialog trees, or real shops exists

## Scope

This contract currently applies only to:
- client -> server interaction requests on the main game socket while already in `GAME`
- bootstrap static actors that were already made visible through the existing static-actor visibility contract
- a current bootstrap distance gate that still requires the player to be near enough to that visible actor before the interaction can resolve
- one target field: the actor's client-visible `VID`
- the game-flow dispatch seam in `internal/game`
- later service-style NPC behavior that still reuses this same request shape

It does **not** yet claim:
- NPC dialog windows
- branching dialogs, quests, combat, or pathing
- real shop buy/sell flows
- target selection persistence
- click-to-move choreography or path validation beyond the current direct visibility-plus-distance gate

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
- the bootstrap runtime now also applies an explicit interaction distance gate after visibility lookup; the current owned limit is a fixed `300` world-unit radius
- `internal/minimal/shared_world` now owns the first validated interaction-attempt seam, returning a structured result for the current subject/target pair before content resolution branches further
- that validated runtime result now distinguishes at least:
  - `subject_not_found`
  - `target_not_visible`
  - `target_out_of_range`
  - `target_has_no_interaction`
  - `warp_destination_invalid`
  - `warp_not_applied`
- `gamed` now also resolves authored interaction definitions by `interaction_kind + interaction_ref`
- loopback-only `GET`/`POST /local/interactions` plus `PATCH`/`PUT`/`DELETE /local/interactions/{kind}/{ref}` now author that deterministic definition catalog without hand-editing the backing JSON file
- update requests preserve stable `kind + ref` identity by requiring the full body identity to match the path exactly
- delete requests now fail closed while a bootstrap static actor still references the targeted definition
- when that definition resolves to `interaction_kind = "info"`, the interacting player now receives one self-only `GC_CHAT` delivery using `CHAT_TYPE_INFO` and the authored definition text
- when that definition resolves to `interaction_kind = "talk"`, the interacting player now receives one self-only chat-backed delivery using a deterministic speaker-prefixed multi-line payload
- known bootstrap interaction failures now also resolve to one deterministic self-only `GC_CHAT` delivery instead of silently disappearing on the socket
- the next frozen NPC gameplay families on top of this same ingress are now service-style `warp` and `shop_preview`
- malformed payloads are rejected at the codec/flow boundary
- future interaction result paths outside the currently known bootstrap rejection reasons may still resolve to no outgoing frames yet

## Loopback authoring surface

The first owned operator surface for interaction content is loopback-only:
- `GET /local/interactions`
- `POST /local/interactions`
- `PATCH /local/interactions/{kind}/{ref}`
- `PUT /local/interactions/{kind}/{ref}`
- `DELETE /local/interactions/{kind}/{ref}`

The current contract is intentionally narrow:
- request/response bodies currently use `kind`, `ref`, `text`
- `PATCH` / `PUT` are full-identity upserts, not partial nested edits
- the body `kind` + `ref` must match the path exactly on update
- delete fails closed while a bootstrap static actor still references that definition

Later slices may extend that authored payload shape for service-style NPC definitions without changing the `INTERACT` request packet itself.

## Target identity rule

The request target is the static actor's current client-visible `VID`.

For bootstrap static actors, that `VID` is currently derived from the runtime `entity_id` when it fits `uint32`.
That keeps the request aligned with the already-owned static-actor visibility bootstrap contract and avoids introducing a second target-identity scheme before real interaction behavior exists.

## Failure semantics

The current owned failure boundary is now explicit and split in two layers:
- wrong phase -> existing `GAME` flow rejection rules apply
- unexpected header at the codec -> rejected
- malformed payload size -> rejected
- once the request is decoded, the bootstrap runtime can now reject resolution as:
  - `subject_not_found`
  - `target_not_visible`
  - `target_out_of_range`
  - `target_has_no_interaction`
  - `warp_destination_invalid`
  - `warp_not_applied`
  - `interaction_definition_not_found`
  - `unsupported_interaction_kind`
- those known runtime rejection reasons now return exactly one self-only `GC_CHAT` delivery using `CHAT_TYPE_INFO` and a deterministic bootstrap message
- accepted `info` interaction currently produces exactly one self-only `GC_CHAT` delivery with `CHAT_TYPE_INFO`
- accepted `talk` interaction currently produces exactly one self-only chat-backed delivery whose payload is speaker-prefixed and multi-line
- accepted `warp` interaction currently reuses the existing self-session transfer rebootstrap path; if authored `text` is present, one self-only `CHAT_TYPE_INFO` delivery is emitted before those transfer frames

Future slices should freeze richer reporting only when dialog UI or later interaction families exist.

## Success definition

After this slice, the repository should be able to say:
- there is a first owned `GAME`-phase interaction request packet for bootstrap static actors
- the request is decoded deterministically from `target_vid`
- the game flow can dispatch that request to a dedicated interaction handler
- `internal/worldruntime` can resolve a visible bootstrap static actor by that `VID` under the active topology/AOI rules
- `internal/minimal/shared_world` can now turn that subject/target pair into a structured validated interaction attempt before later content resolution exists
- `gamed` can now resolve authored `info` and `talk` definitions behind visible static actors and answer the interacting player with one self-only chat-backed delivery carrying the authored text
- the same ingress and target-lookup contract is now explicitly frozen as the base for the next service-style NPC families (`warp` and `shop_preview`) without inventing a new client request packet first
