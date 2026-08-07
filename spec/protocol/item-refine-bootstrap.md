# Item refine bootstrap

This note freezes the first clean-room `REFINE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader refine gameplay is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, equipment, quickslot, point, ground-item, peer, or persistence mutation until a later refine-system slice owns material, cost, success/failure, and result semantics

This is not a completed refine, upgrade, scroll, metin-stone, bonus-changer, or dragon-soul refine system.

## Client packet

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::REFINE` (`0x050C`)

Direction: client -> server.

Payload size is 2 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `pos` | `uint8` | client-selected refine slot / position value |
| 1 | `type` | `uint8` | client-selected refine request type |

Total frame length is 6 bytes including the common `header` and `length` fields.

The layout is frozen from the TMP4-compatible client packet struct shape in project-owned terms. The repository owns only the byte layout and current fail-closed runtime policy.

## Current runtime contract

`internal/game` decodes `REFINE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves `REFINE` unsupported for now. Every packet currently fails closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- refine material, cost, or catalyst semantics
- success, failure, downgrade, destroy, or safe-refine outcomes
- item socket, metin-stone, attribute, or bonus-changing behavior
- refine window/open/close choreography
- server `REFINE_INFORMATION`, `REFINE_INFORMATION_NEW`, or dragon-soul refine packets
- inventory/equipment refresh ordering for accepted refine results
- audit, rollback, or durable economic policy for refine attempts

## Current coverage

- `internal/proto/item` freezes `REFINE` encode/decode behavior plus unexpected-header and invalid-payload rejection.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/minimal` freezes the shipped runtime fail-closed behavior with persisted inventory, quickslots, and points unchanged after a `REFINE` packet.
