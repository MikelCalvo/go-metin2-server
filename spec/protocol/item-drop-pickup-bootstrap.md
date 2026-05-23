# Item drop and pickup bootstrap

This note freezes the first clean-room packet contract for the item ground-interaction family. It is intentionally protocol-only for now: the runtime still rejects these packets until a later slice owns world item state, visibility, ownership, and persistence.

Owned in this slice:

- client `CG::ITEM_DROP` codec shape;
- client `CG::ITEM_DROP2` codec shape;
- client `CG::ITEM_PICKUP` codec shape;
- server `GC::ITEM_GROUND_ADD` codec shape;
- server `GC::ITEM_GROUND_DEL` codec shape.

Not owned yet:

- accepting item drop or pickup in `GAME`;
- mutating carried inventory as a result of drop/pickup;
- ground item entity ownership, visibility fanout, despawn timing, anti-drop policy, trade/shop restrictions, or pickup authorization;
- gold-drop semantics beyond freezing the client packet fields;
- `GC::ITEM_DROP`, `GC::ITEM_OWNERSHIP`, or `GC::ITEM_GET` behavior.

## Client packets

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::ITEM_DROP` (`0x0502`)

Payload size is 7 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `pos` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE` |
| 3 | `elk` | `uint32 LE` | gold amount field used by the legacy client send path |

### `CG::ITEM_DROP2` (`0x0503`)

Payload size is 8 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `pos` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE` |
| 3 | `gold` | `uint32 LE` | gold amount field |
| 7 | `count` | `uint8` | item count requested by the newer drop path |

### `CG::ITEM_PICKUP` (`0x0505`)

Payload size is 4 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vid` | `uint32 LE` | visible ground-item runtime identifier |

## Server packets

### `GC::ITEM_GROUND_ADD` (`0x0515`)

Payload size is 20 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vid` | `uint32 LE` | visible ground-item runtime identifier |
| 4 | `vnum` | `uint32 LE` | item template id |
| 8 | `x` | `int32 LE` | world x coordinate |
| 12 | `y` | `int32 LE` | world y coordinate |
| 16 | `z` | `int32 LE` | world z coordinate |

The client receive path converts the global coordinates to local item-rendering coordinates before creating the client-side item actor.

### `GC::ITEM_GROUND_DEL` (`0x0516`)

Payload size is 4 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vid` | `uint32 LE` | visible ground-item runtime identifier to remove |

## Current runtime contract

These codecs are not yet wired into `internal/game` or `internal/minimal`. Until a later runtime slice owns ground item state, client-originated drop and pickup packets remain unsupported by the live `GAME` flow and should fail closed with no inventory mutation.

Reference-oracle evidence: the TMP4-compatible client exposes `SendItemDropPacket`, `SendItemDropPacketNew`, and `SendItemPickUpPacket` on the game socket, and consumes `GC::ITEM_GROUND_ADD` / `GC::ITEM_GROUND_DEL` to create and remove client-side ground item actors. This repository owns only the project-written field layouts above.

Current coverage:

- `internal/proto/item` freezes encode/decode round-trips for `ITEM_DROP`, `ITEM_DROP2`, `ITEM_PICKUP`, `ITEM_GROUND_ADD`, and `ITEM_GROUND_DEL`, plus unexpected-header and invalid-payload rejection for the new codecs.
