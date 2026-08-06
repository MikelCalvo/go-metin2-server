# Item give bootstrap

This note freezes the first clean-room `ITEM_GIVE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader player-to-player item transfer is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, quickslot, ground-item, or persistence mutation until a later exchange/trade slice owns recipient semantics

This is not a completed item-give, exchange, trade, or NPC handoff system.

## Client packet

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::ITEM_GIVE` (`0x0507`)

Direction: client -> server.

Payload size is 8 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `target_vid` | `uint32 LE` | visible actor the client is attempting to give to |
| 4 | `item_pos` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE` |
| 7 | `count` | `uint8` | requested stack count |

The layout is frozen from the TMP4-compatible client packet struct shape in project-owned terms. The repository owns only the byte layout and current fail-closed runtime policy.

## Current runtime contract

`internal/game` decodes `ITEM_GIVE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves `ITEM_GIVE` unsupported for now. For every target, source cell, window, and count:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

This keeps accidental client attempts fail-closed instead of falling into incomplete item-transfer behavior.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- target eligibility or range checks
- player-to-player give semantics
- NPC-target give semantics
- exchange/trade window choreography
- partial-stack transfer behavior
- recipient inventory placement and quickslot side effects
- template-authored acceptance/rejection text for item-give attempts
- ownership, audit, or rollback policy for two-party mutations

## Current coverage

- `internal/proto/item` freezes `ITEM_GIVE` encode/decode round trips plus unexpected-header and invalid-payload rejection.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/minimal` freezes the shipped runtime fail-closed behavior with persisted inventory and quickslots unchanged after an `ITEM_GIVE` packet.
