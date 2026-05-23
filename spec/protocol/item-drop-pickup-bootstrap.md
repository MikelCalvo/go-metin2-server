# Item drop and pickup bootstrap

This note freezes the first clean-room packet, dispatch, and minimal runtime contract for the item ground-interaction family. The current runtime owns player-local drop mutation, a self-visible ground-add plus ownership echo, and visible-peer pickup of ground handles created by accepted drops. Broader shared ground item policy remains a later slice.

Owned in this slice family:

- client `CG::ITEM_DROP` codec shape and `GAME` dispatch seam;
- client `CG::ITEM_DROP2` codec shape and `GAME` dispatch seam;
- client `CG::ITEM_PICKUP` codec shape and `GAME` dispatch seam;
- server `GC::ITEM_GROUND_ADD` codec shape;
- server `GC::ITEM_GROUND_DEL` codec shape;
- server `GC::ITEM_OWNERSHIP` codec shape;
- server `GC::ITEM_GET` codec shape for pickup notices.

Owned by the first runtime drop slice:

- `CG::ITEM_DROP` and `CG::ITEM_DROP2` are accepted in `GAME` only for carried inventory slots;
- whole-stack drops remove the carried item, clear item quickslots pointing at that slot, persist the selected character snapshot, and return self-only `GC::ITEM_DEL` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`;
- counted drops decrement the carried stack, persist the selected character snapshot, and return self-only `GC::ITEM_UPDATE` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`;
- the bootstrap ground item `vid` is deterministic and non-zero, derived from the selected character `VID` and source slot; it is a visible handle for the self echo / visible peer rebuilds and is not yet a durable shared-world entity.

Not owned yet:

- permanent/shared-world ground item entity IDs, ownership timers, despawn timing, anti-drop policy, trade/shop restrictions, or range/path authorization beyond current visible-world scope;
- gold-drop semantics beyond freezing the client packet fields;
- `GC::ITEM_DROP`, timed/permission-changing ownership transitions, or party-delivery `GC::ITEM_GET` behavior.

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

### `GC::ITEM_OWNERSHIP` (`0x0517`)

Direction: server -> client.

Payload size: `29` bytes.

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vid` | `uint32 LE` | visible bootstrap ground-item handle |
| 4 | `owner_name` | fixed `25` bytes | drop owner's character name, zero-padded/truncated to the fixed legacy field |

The bootstrap runtime emits ownership immediately after every `GC::ITEM_GROUND_ADD` it creates for accepted player drops, visible peer drop fanout, radius-AOI ground re-entry rebuilds, and transfer destination ground re-entry rebuilds. The packet currently labels ownership only; it does not yet implement ownership timers, permission changes, party pickup rules, or delayed public ownership release.

### `GC::ITEM_GET` (`0x0518`)

Direction: server -> client.

Payload size: `30` bytes.

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vnum` | `uint32 LE` | item vnum |
| 4 | `count` | `uint8` | count displayed in the pickup notice |
| 5 | `arg` | `uint8` | `0` for normal/self pickup in the current runtime |
| 6 | `from_name` | fixed `25` bytes | zero-filled for normal/self pickup; reserved for later party-delivery variants |

The legacy server emits this notice after accepted ordinary item pickup. The current bootstrap runtime owns only the normal/self form (`arg = 0`, empty `from_name`).

## Current runtime contract

`internal/game` now recognizes all three client packets while already in `GAME` and routes decoded requests to dedicated handlers. The default handler behavior is deny/no-response. The shipped bootstrap runtime currently accepts carried-item drops plus visible-world pickup of temporary bootstrap ground handles created by those accepted drops.

The `0x0502` header is shared by the already-owned carried-slot `ITEM_USE` request and the legacy `ITEM_DROP` request. Dispatch therefore uses the payload size: 3-byte payloads route to `ITEM_USE`, and 7-byte payloads route to `ITEM_DROP`. Other payload sizes fail closed at the codec layer.

For the first live runtime slice, accepted drops are self-facing and persistence-backed:

1. `ITEM_DROP` uses the current full carried stack count.
2. `ITEM_DROP2` uses the requested non-zero count and rejects counts larger than the stack.
3. The selected player's live inventory is removed or decremented, then the selected character snapshot is persisted through the existing account-store path.
4. Whole-stack drops clear item quickslots pointing at the removed slot.
5. The server returns the carried-slot mutation frame first (`GC::ITEM_DEL` or `GC::ITEM_UPDATE`), then any quickslot deletes, then one self-only `GC::ITEM_GROUND_ADD` at the selected character's current coordinates followed by `GC::ITEM_OWNERSHIP` naming the dropping character.
6. The same session remembers that deterministic ground handle until it is picked up or the session ends.

For the first visible-peer pickup runtime slice, accepted pickup is visible-world scoped:

1. Accepted drops are registered as temporary bootstrap ground handles at the dropper's current effective map/position after the selected character mutation is persisted.
2. The dropper receives the same direct `GC::ITEM_GROUND_ADD` + `GC::ITEM_OWNERSHIP` already owned by the first drop slice, and currently visible peers receive one queued add/ownership pair for the same handle.
3. `ITEM_PICKUP` is accepted when its `vid` matches a still-pending bootstrap ground handle in the collector's visible world.
4. The item is restored into the collector's original carried slot when that slot is empty; if that original slot is occupied, the bootstrap runtime falls back to the lowest empty carried inventory slot.
5. If no carried inventory slot is free, pickup fails closed and leaves the temporary ground handle pending.
6. The collector's selected character snapshot is persisted through the same account-store path used by drops before the handle is removed from the temporary ground table.
7. The collector receives self `GC::ITEM_GROUND_DEL` first, then `GC::ITEM_SET` for the restored carried slot, then normal/self `GC::ITEM_GET(vnum, count, arg=0, from_name="")`; other visible sessions receive one queued `GC::ITEM_GROUND_DEL`.
8. While a temporary handle remains pending, later radius-AOI `MOVE` / `SYNC_POSITION` visibility transitions rebuild it for the moving/syncing session: crossing into the handle's visible world queues `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after ordinary player/static visibility transition frames, and crossing out queues `GC::ITEM_GROUND_DEL` after ordinary transition frames.
9. Gameplay-triggered exact-position transfer also rebuilds pending ground-item visibility for the moved session as part of the immediate self rebootstrap result: source-map handles no longer visible to the destination emit `GC::ITEM_GROUND_DEL`, and destination handles newly visible after transfer emit `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after the existing self bootstrap, peer, and static-actor transfer frames.
10. Replayed, unknown, invisible, or no-free-slot pickups fail closed with no frames.

The dropped ground item is still bootstrap-scoped rather than a durable shared-world entity. Reconnecting does not restore it as a ground entity, and broader ownership/range/despawn policy remains future work.

Reference-oracle evidence: the TMP4-compatible client exposes `SendItemDropPacket`, `SendItemDropPacketNew`, and `SendItemPickUpPacket` on the game socket, and consumes `GC::ITEM_GROUND_ADD` / `GC::ITEM_GROUND_DEL` to create and remove client-side ground item actors plus `GC::ITEM_OWNERSHIP` to label item ownership. This repository owns only the project-written field layouts and dispatch boundaries above.

Current coverage:

- `internal/proto/item` freezes encode/decode round-trips for `ITEM_DROP`, `ITEM_DROP2`, `ITEM_PICKUP`, `ITEM_GROUND_ADD`, `ITEM_GROUND_DEL`, `ITEM_OWNERSHIP`, and normal/party-shaped `ITEM_GET`, plus unexpected-header and invalid-payload rejection for the new codecs.
- `internal/game` freezes `GAME`-phase dispatch for `ITEM_DROP`, `ITEM_DROP2`, and `ITEM_PICKUP`, including the shared-header `ITEM_USE` / `ITEM_DROP` payload-size split.
- `internal/minimal` accepts carried-item drop requests with self ground-add/ownership echoes, queues matching ground-add/ownership echoes to currently visible peers, accepts visible-world pickup of temporary bootstrap ground handles, and rebuilds still-pending ground-handle visibility for the moving/syncing session on radius-AOI `MOVE` / `SYNC_POSITION` boundary crossings and exact-position transfer self rebootstrap while durable ownership timers and permission changes remain deferred.
