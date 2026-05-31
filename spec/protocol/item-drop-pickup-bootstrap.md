# Item drop and pickup bootstrap

This note freezes the first clean-room packet, dispatch, and minimal runtime contract for the item ground-interaction family. The current runtime owns player-local drop mutation, a self-visible ground-add plus ownership echo, and visible-peer pickup of ground handles created by accepted drops. Broader shared ground item policy remains a later slice.

Owned in this slice family:

- client `CG::ITEM_DROP` codec shape and `GAME` dispatch seam;
- client `CG::ITEM_DROP2` codec shape and `GAME` dispatch seam;
- client `CG::ITEM_PICKUP` codec shape and `GAME` dispatch seam;
- server `GC::ITEM_GROUND_ADD` codec shape;
- server `GC::ITEM_GROUND_DEL` codec shape;
- server `GC::ITEM_OWNERSHIP` codec shape;
- server `GC::ITEM_GET` codec shape for pickup notices;
- pending bootstrap ground item snapshots in runtime/operator map occupancy and transfer-preview results.

Owned by the first runtime drop slice:

- `CG::ITEM_DROP` and `CG::ITEM_DROP2` are accepted in `GAME` only for carried inventory slots;
- whole-stack drops remove the carried item, clear item quickslots pointing at that slot, persist the selected character snapshot, and return self-only `GC::ITEM_DEL` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`;
- counted drops decrement the carried stack, persist the selected character snapshot, and return self-only `GC::ITEM_UPDATE` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`;
- the bootstrap ground item `vid` is deterministic and non-zero, derived from the selected character `VID` and source slot; it is a visible handle for the self echo / visible peer rebuilds and is not yet a durable shared-world entity.

Not owned yet:

- permanent/shared-world ground item entity IDs, ownership timers, despawn timing, trade/shop restrictions, or range/path authorization beyond current visible-world scope;
- `GC::ITEM_DROP`, timed/permission-changing ownership transitions, real party membership checks, or public ownership release.

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
| 5 | `arg` | `uint8` | `0` for normal/self pickup; party-shaped notices use the already-frozen legacy-compatible arg values |
| 6 | `from_name` | fixed `25` bytes | zero-filled for normal/self pickup; party-shaped notices carry the other participant name |

The legacy server emits this notice after accepted ordinary item pickup. The current bootstrap runtime owns the normal/self form (`arg = 0`, empty `from_name`) plus the first party-shaped owner-delivery notices when a visible bootstrap party peer collects an item still owned by another live session.

## Current runtime contract

`internal/game` now recognizes all three client packets while already in `GAME` and routes decoded requests to dedicated handlers. The default handler behavior is deny/no-response. The shipped bootstrap runtime currently accepts carried-item drops plus visible-world pickup of temporary bootstrap ground handles created by those accepted drops.

The `0x0502` header is shared by the already-owned carried-slot `ITEM_USE` request and the legacy `ITEM_DROP` request. Dispatch therefore uses the payload size: 3-byte payloads route to `ITEM_USE`, and 7-byte payloads route to `ITEM_DROP`. Other payload sizes fail closed at the codec layer.

For the first live runtime slice, accepted drops are self-facing and persistence-backed:

1. `ITEM_DROP` uses the current full carried stack count.
2. `ITEM_DROP2` uses the requested non-zero count when it fits the current stack; a zero or oversized count is normalized to the whole stack before inventory mutation, matching the observed legacy `DropItem` count normalization.
3. If the carried item's loaded template is marked `anti_drop` or `anti_give`, the drop is rejected before live inventory, quickslots, ground handles, or persistence are mutated, and the selected session receives one self-only `CHAT_TYPE_INFO` system message (`"You cannot drop this item."`). This mirrors the legacy oracle's early visible rejection for player-requested drops of bound/non-giveable items while keeping forced system-drop, localization text catalogs, and death-drop policy out of scope.
4. The selected player's live inventory is removed or decremented, then the selected character snapshot is persisted through the existing account-store path.
5. Whole-stack drops clear item quickslots pointing at the removed slot.
6. The server returns the carried-slot mutation frame first (`GC::ITEM_DEL` or `GC::ITEM_UPDATE`), then any quickslot deletes, then one self-only `GC::ITEM_GROUND_ADD` at the selected character's current coordinates followed by `GC::ITEM_OWNERSHIP` naming the dropping character.
7. The shared bootstrap runtime remembers that deterministic ground handle until it is picked up or the owning live session ends. When the owner session closes while a handle is still pending, the runtime removes the temporary handle and queues `GC::ITEM_GROUND_DEL` to currently visible peers so they do not keep a stale ground item actor.

The first gold-drop runtime slice owns the gold amount fields on `CG::ITEM_DROP` / `CG::ITEM_DROP2` as a bootstrap currency-only ground entry:

1. If the packet gold/elk field is non-zero, the runtime treats the request as a gold drop and does not mutate the carried item slot/count field.
2. The selected player's live and persisted gold are decremented by the requested amount. Zero, over-balance, and out-of-range amounts fail closed with no response and no mutation.
3. The selected session receives `GC::PLAYER_POINT_CHANGE` for point type `11` with the negative amount and updated gold total, then one `GC::ITEM_GROUND_ADD` at the selected character's current coordinates and `GC::ITEM_OWNERSHIP` naming the dropping character.
4. The bootstrap ground-add `vnum` for gold is currently fixed to `1`, matching the first owned currency marker while richer client display/count semantics remain deferred.
5. Gold ground pickup is accepted for the still-pending bootstrap gold marker when the collector is in the same visible world and within the owned 300-unit pickup reach. The collector receives `GC::ITEM_GROUND_DEL` followed by `GC::PLAYER_POINT_CHANGE` for point type `11` with the positive amount and updated gold total. The selected character snapshot is persisted before the temporary gold marker is removed, and replayed pickup fails closed.
6. Runtime/operator ground snapshots expose pending gold markers as `vnum = 1`, `gold_amount = <dropped amount>`, and no item-stack `count`; ordinary item ground snapshots continue to expose their stack `count` and omit `gold_amount`.
7. If a visible collector picks up a gold marker still owned by another live session, the bootstrap runtime treats that collector as a temporary bootstrap party member. The owner receives the gold point-change restore plus `GC::ITEM_GET(arg=1, from_name=<collector>)`, and the collector receives `GC::ITEM_GROUND_DEL` plus `GC::ITEM_GET(arg=2, from_name=<owner>)`. The collector's own gold total is not mutated while owner delivery succeeds.
8. Gold ownership timers, public ownership release, richer gold-marker display/count semantics, durable shared-world ground currency entities, and anti-give restrictions for currency markers and fallback delivery when owner currency persistence fails remain deferred.

For the first visible-peer pickup runtime slice, accepted pickup is visible-world scoped:

1. Accepted drops are registered as temporary bootstrap ground handles at the dropper's current effective map/position after the selected character mutation is persisted; zero-HP/dead owners are rejected at the shared-world ground-handle seam so death-state races cannot create new visible handles.
2. The dropper receives the same direct `GC::ITEM_GROUND_ADD` + `GC::ITEM_OWNERSHIP` already owned by the first drop slice, currently visible living peers receive one queued add/ownership pair for the same handle, and later visible sessions that enter `GAME` while the handle remains pending receive that same add/ownership pair after self bootstrap, peer-player bootstrap, and visible static-actor bootstrap frames. Zero-HP/dead peers stay registered for their own restart/teardown flow, but they are skipped as recipients for new ground-handle visibility fanout.
3. `ITEM_PICKUP` is accepted when its `vid` matches a still-pending bootstrap ground handle in the collector's visible world and the collector is within the first owned pickup reach (`300` coordinate units) of the ground handle. Visible-but-out-of-reach attempts fail closed with no frames, no inventory mutation, and the temporary handle left pending.
4. When the picked item's loaded template is valid, its `vnum` must match the ground item `vnum` before template-authored stack metadata is applied. Mismatched template metadata fails closed at the player mutation boundary before any inventory mutation.
5. When the picked item is stackable, not authored with `anti_stack`, and a carried compatible, unlocked, non-equipped stack can absorb the full ground count, pickup merges into the lowest such carried slot and refreshes it with `GC::ITEM_UPDATE`.
6. If no single stack can absorb the full count, stackable non-`anti_stack` pickup fills compatible partial stacks in carried-slot order and then places any remaining count into the original carried slot when that slot is empty, or the lowest empty carried inventory slot otherwise. Loaded `anti_stack` templates skip compatible-stack merging and restore/pick up into a fresh carried slot only.
7. Split stackable pickup returns one `GC::ITEM_UPDATE` per filled partial stack followed by `GC::ITEM_SET` for any placed remainder; if the partial stacks can absorb the full count, no `GC::ITEM_SET` is emitted.
8. If neither compatible-stack capacity nor a carried inventory slot can accept the whole picked count, pickup fails without mutating or persisting the collector inventory and leaves the temporary ground handle pending. The selected session receives the bootstrap inventory-full `CHAT_TYPE_INFO` system message (`"You have too many items."`).
9. On accepted pickup, the collector's selected character snapshot is persisted through the same account-store path used by drops before the handle is removed from the temporary ground table.
10. The collector receives self `GC::ITEM_GROUND_DEL` first, then the deterministic carried inventory refresh frames (`GC::ITEM_UPDATE` and/or `GC::ITEM_SET`), then normal/self `GC::ITEM_GET(vnum, count, arg=0, from_name="")`; other visible sessions receive one queued `GC::ITEM_GROUND_DEL`.
11. While a temporary handle remains pending, later radius-AOI `MOVE` / `SYNC_POSITION` visibility transitions rebuild it for the moving/syncing session: crossing into the handle's visible world queues `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after ordinary player/static visibility transition frames, and crossing out queues `GC::ITEM_GROUND_DEL` after ordinary transition frames. The visibility calculation is owned by `internal/worldruntime` through the same configured topology/AOI policy used by player and static-actor scopes; `internal/minimal` only maps the resulting ground-item diff back to wire frames.
12. Gameplay-triggered exact-position transfer also rebuilds pending ground-item visibility for the moved session as part of the immediate self rebootstrap result: source-map handles no longer visible to the destination emit `GC::ITEM_GROUND_DEL`, and destination handles newly visible after transfer emit `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after the existing self bootstrap, peer, and static-actor transfer frames.
13. If a visible collector picks up a ground handle still owned by another live session, the bootstrap runtime treats that collector as a temporary bootstrap party member. The runtime first attempts to deliver the item to the owner account/runtime; on success, the collector receives `GC::ITEM_GROUND_DEL` plus `GC::ITEM_GET(arg=2, from_name=<owner>)`, and the owner receives the queued peer delete, deterministic owner inventory refresh frames, and `GC::ITEM_GET(arg=1, from_name=<collector>)`.
14. Owner-delivery pickup uses the same recipient-side placement precedence as self pickup: compatible stackable inventory is filled before a fresh carried slot is used, and the resulting owner inventory mutation is persisted before the temporary ground handle is removed. If the owner is still live on another game socket, that session's selected-character runtime is refreshed with the same persisted inventory snapshot before queued owner refresh frames are delivered, so later owner-side item actions on that socket see the delivered item. The collector's carried inventory capacity is not consulted while owner delivery succeeds because the collector is not the item recipient.
15. Owner-delivery pickup persistence is keyed by the owner's active login ticket, not by the visible character name; this keeps character names and account logins independent while the visible `from_name` fields still use character names.
16. If the picked item's loaded template is marked `anti_give`, visible-party owner-delivery rejects before mutating the owner or collector inventories. The collector receives the same inventory-full `CHAT_TYPE_INFO` rejection used for fail-closed placement failures, no owner refresh or pickup-notice frames are queued, and the temporary ground handle remains pending so the owner can reclaim it.
17. If owner delivery cannot place the item into the owner's current carried inventory, the bootstrap party path falls back to the collector as the recipient, matching the observed legacy party-pickup fallback. That fallback uses the normal/self pickup notice shape (`arg=0`, empty `from_name`), persists the collector mutation, removes the temporary ground handle, and queues only the peer `GC::ITEM_GROUND_DEL` to the full owner.
18. If neither the owner nor the collector can place the item, the runtime rejects the pickup with the same inventory-full `CHAT_TYPE_INFO` message to the collector, does not enqueue owner inventory refresh or pickup notice frames, and leaves the temporary ground handle pending for a later retry.
19. Replayed, unknown, invisible, or zero-HP/dead-collector pickups fail closed with no frames; zero-HP/dead collectors also fail closed on the runtime ground-handle visibility lookup before pickup resolution; no-free-slot pickups use the inventory-full info chat path above.

Dropped ground entries are still bootstrap-scoped rather than durable shared-world entities. Reconnecting does not restore them as ground entities, and broader ownership/range/despawn policy remains future work. Party pickup is likewise bootstrap-scoped: all visible live sessions are treated as the temporary party surface until real party membership, ownership timers, and broader permission changes are owned.

Reference-oracle evidence: the TMP4-compatible client exposes `SendItemDropPacket`, `SendItemDropPacketNew`, and `SendItemPickUpPacket` on the game socket, and consumes `GC::ITEM_GROUND_ADD` / `GC::ITEM_GROUND_DEL` to create and remove client-side ground item actors plus `GC::ITEM_OWNERSHIP` to label item ownership. This repository owns only the project-written field layouts and dispatch boundaries above.

Current coverage:

- `internal/proto/item` freezes encode/decode round-trips for `ITEM_DROP`, `ITEM_DROP2`, `ITEM_PICKUP`, `ITEM_GROUND_ADD`, `ITEM_GROUND_DEL`, `ITEM_OWNERSHIP`, and normal/party-shaped `ITEM_GET`, plus unexpected-header and invalid-payload rejection for the new codecs.
- `internal/game` freezes `GAME`-phase dispatch for `ITEM_DROP`, `ITEM_DROP2`, and `ITEM_PICKUP`, including the shared-header `ITEM_USE` / `ITEM_DROP` payload-size split.
- `internal/minimal` accepts carried-item drop requests with self ground-add/ownership echoes, accepts bootstrap gold drops from the `elk`/`gold` packet field with a self point-change plus currency ground marker, accepts self/ordinary pickup of those pending gold markers with `GROUND_DEL` plus positive gold point-change, rejects loaded-template `anti_drop` / `anti_give` player-requested drops before mutation with a self-only info chat rejection, queues matching ground-add/ownership echoes to currently visible living peers for ordinary item drops, bootstraps still-pending ground handles for later visible sessions that enter `GAME`, accepts visible-world pickup of temporary bootstrap ground handles only when the collector is alive and within the owned 300-unit pickup reach, rejects zero-HP/dead collector lookup/removal before recipient mutation or ground-handle removal, emits the inventory-full info chat while leaving the pending handle and persisted recipient inventory untouched when no eligible recipient can place the item, removes pending owner-scoped handles on owner session close with visible-peer `GROUND_DEL`, supports party-shaped owner-delivery pickup notices for visible live item owners, rejects loaded-template `anti_give` visible-party owner-delivery pickup before owner/collector mutation while leaving the handle pending, supports party-shaped owner-delivery gold pickup with owner gold restoration, falls back to normal collector delivery when owner item delivery cannot place but the collector can, rebuilds still-pending ground-handle visibility for the moving/syncing session on radius-AOI `MOVE` / `SYNC_POSITION` boundary crossings and exact-position transfer self rebootstrap, and exposes pending ground handles in `/local/maps` plus relocation preview/transfer `before_map_occupancy` / `after_map_occupancy` snapshots. Snapshot payloads distinguish ordinary item stacks (`count`) from gold markers (`gold_amount`, no stack count) while durable ownership timers, real party membership, and permission changes remain deferred.
