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
- pending bootstrap ground item snapshots in runtime/operator map occupancy and transfer-preview results;
- the schema-only `0010_bootstrap_ground_item_state` migration boundary for future durable/import tooling around currently in-memory item-shaped and gold-shaped bootstrap ground handles.

Owned by the first runtime drop slice:

- `CG::ITEM_DROP` and `CG::ITEM_DROP2` are accepted in `GAME` only for carried inventory slots;
- whole-stack drops remove the carried item, clear all item quickslots pointing at that slot in deterministic quickslot-position order, persist the selected character snapshot, and return self-only `GC::ITEM_DEL` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`;
- counted drops decrement the carried stack, persist the selected character snapshot, and return self-only `GC::ITEM_UPDATE` plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`; when template metadata is loaded for the dropped stack, the source-slot `GC::ITEM_UPDATE` preserves the authored display `sockets` and `attributes` while updating only the count;
- the bootstrap ground item `vid` is deterministic and non-zero, derived from the selected character `VID` and source slot; it is a visible handle for the self echo / visible peer rebuilds and is not yet a durable shared-world entity; if that deterministic handle already exists, the drop fails closed before inventory, quickslots, ground visibility, or persistence are mutated.

Not owned yet:

- permanent/shared-world ground item entity IDs, DB-backed runtime restoration, despawn timing, trade/shop restrictions, or range/path authorization beyond current visible-world scope;
- `GC::ITEM_DROP`, real party membership checks, or durable/restart-restored ownership timer state.

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
| 0 | `x` | `int32 LE` | world x coordinate |
| 4 | `y` | `int32 LE` | world y coordinate |
| 8 | `z` | `int32 LE` | world z coordinate |
| 12 | `vid` | `uint32 LE` | visible ground-item runtime identifier |
| 16 | `vnum` | `uint32 LE` | item template id |

The client receive path converts the global coordinates to local item-rendering coordinates before creating the client-side item actor.

This field order is intentionally frozen from the current TMP4-compatible client-facing struct shape: coordinates precede the visible item handle and template id. Earlier bootstrap tests had used the server-internal semantic order (`vid`, `vnum`, `x`, `y`, `z`); that ordering is now rejected by the project-owned golden codec fixture so ground actors decode at the intended coordinates and item identity on the real client.

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

The bootstrap runtime emits ownership immediately after every `GC::ITEM_GROUND_ADD` it creates for accepted player drops, visible peer drop fanout, radius-AOI ground re-entry rebuilds, and transfer destination ground re-entry rebuilds. While a pending handle is still inside its exclusive ownership window, that packet carries the owner's character name. When the in-memory exclusive ownership timer elapses, the runtime emits one blank `GC::ITEM_OWNERSHIP` (`owner_name` empty) to currently visible living peers around the handle and the pickup permission transitions to public ordinary collector pickup. Real party membership checks remain deferred.

### `GC::ITEM_GET` (`0x0518`)

Direction: server -> client.

Payload size: `30` bytes.

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `vnum` | `uint32 LE` | item vnum |
| 4 | `count` | `uint8` | count displayed in the pickup notice |
| 5 | `arg` | `uint8` | `0` for normal/self pickup; party-shaped notices use the already-frozen legacy-compatible arg values |
| 6 | `from_name` | fixed `25` bytes | zero-filled for normal/self pickup; party-shaped notices carry the other participant name |

The legacy server emits this notice after accepted ordinary item pickup. The current bootstrap runtime owns the normal/self form (`arg = 0`, empty `from_name`) for owner self-pickup and for public ordinary collector pickup after exclusive ownership release. Party-shaped owner-delivery notices remain deferred until a later slice owns real party membership; the previous always-on visible-peer owner-delivery approximation is retired now that exclusive ownership timers and public release exist.

## Current runtime contract

`internal/game` now recognizes all three client packets while already in `GAME` and routes decoded requests to dedicated handlers. The default handler behavior is deny/no-response. The shipped bootstrap runtime currently accepts carried-item drops plus visible-world pickup of temporary bootstrap ground handles created by those accepted drops.

The `0x0502` header is shared by the already-owned carried-slot `ITEM_USE` request and the legacy `ITEM_DROP` request. Dispatch therefore uses the payload size: 3-byte payloads route to `ITEM_USE`, and 7-byte payloads route to `ITEM_DROP`. Other payload sizes fail closed at the codec layer.

For the first live runtime slice, accepted drops are self-facing and persistence-backed:

1. `ITEM_DROP` uses the current full carried stack count.
2. `ITEM_DROP2` uses the requested non-zero count when it fits the current stack; a zero or oversized count is normalized to the whole stack before inventory mutation, matching the observed legacy `DropItem` count normalization. The minimal runtime now freezes both zero-count and oversized-count normalization through the packet path: either request shape removes the whole carried stack, clears all item quickslots pointing at that removed slot in deterministic quickslot-position order, preserves unrelated skill/command quickslots with the same byte slot value, and persists that snapshot before exposing the ground item. Partial counted drops keep the original item in the source carried slot and therefore preserve item quickslots, as well as unrelated skill/command quickslots, pointing at that slot.
3. If an authored item-template snapshot is loaded, the carried item's `vnum` must resolve to a valid template before the drop mutates inventory; missing metadata fails closed with no frames and no live inventory, quickslot, ground-handle, or persistence mutation. If a loaded template is present, it must validate under the item-template bootstrap contract, its `vnum` must match the carried item's live `vnum`, and the carried stack count must not exceed the template-authored `max_count`; malformed, mismatched, or over-template-max stack metadata fails closed before live inventory, quickslots, ground handles, or persistence are mutated. The deterministic bootstrap fallback snapshot used when no item-template file exists remains valid for local smoke/runtime startup; this missing-template guard applies only after an authored item-template snapshot has been loaded.
4. If the selected character is at the bootstrap zero-HP floor, the selected carried item is locked, the requested carried cell has duplicate live item occupancy, the deterministic bootstrap ground `vid` is already registered, or the carried item would become malformed while applying the drop mutation, the drop fails closed before live inventory, quickslots, ground handles, or persistence are mutated.
5. If the carried item's loaded template is marked `anti_drop`, `anti_give`, `anti_sell`, `anti_get`, or `anti_stack`, or carries an authored selected-character restriction (`anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_male`, `anti_female`, `anti_empire_a`, `anti_empire_b`, `anti_empire_c`, or `min_level` above the selected character's current level), the drop is rejected before live inventory, quickslots, ground handles, or persistence are mutated. Transfer-guard rejections return one self-only `CHAT_TYPE_INFO` system message sourced from the template-authored `drop_reject_message` when non-empty, otherwise the deterministic fallback text `"You cannot drop this item."`; selected-character restriction rejections emit that same self-only info chat only when `drop_reject_message` is authored and otherwise remain the older silent no-frame rejection. If an active bootstrap exchange shell is open on the same socket, this template-backed drop rejection feedback closes that shell first with self/peer `GC::EXCHANGE END`, clears exchange presentation state, and still leaves carried inventory, quickslots, ground handles, and persistence unchanged. This keeps player-requested drops of bound/non-transferable or class/sex/empire/level-restricted bootstrap items fail-closed while forced system-drop, localization catalogs beyond this one authored message field, and death-drop policy stay out of scope.
6. The selected player's live inventory is removed or decremented, then the selected character snapshot is persisted through the existing account-store path. If that account-store write fails, the drop fails closed: the live inventory/quickslot snapshot rolls back, no success frames are emitted, no temporary ground handle is registered, and the persisted account snapshot remains unchanged.
7. Whole-stack drops clear item quickslots pointing at the removed slot.
8. Without an active exchange shell, the server returns the carried-slot mutation frame first (`GC::ITEM_DEL` or `GC::ITEM_UPDATE`), then any quickslot deletes, then one self-only `GC::ITEM_GROUND_ADD` at the selected character's current coordinates followed by `GC::ITEM_OWNERSHIP` naming the dropping character.
9. The shared bootstrap runtime remembers that deterministic ground handle until it is picked up or the owning live session ends. When the owner session closes while a handle is still pending, the runtime removes the temporary handle and queues `GC::ITEM_GROUND_DEL` to currently visible peers so they do not keep a stale ground item actor.
10. If an older reclaimed socket sends an otherwise valid carried-item drop after a fresh session has become authoritative for the same selected character, the stale socket may still receive deterministic self-local carried-slot mutation frames (`GC::ITEM_DEL` / `GC::ITEM_UPDATE` plus any source quickslot deletes). It does not reserve/register a bootstrap ground handle, does not emit `GC::ITEM_GROUND_ADD` or `GC::ITEM_OWNERSHIP`, does not queue ground visibility to the fresh session or other peers, and does not mutate the authoritative persisted account snapshot.
11. If an active bootstrap exchange shell is open on the same socket, accepted carried-item `ITEM_DROP` / `ITEM_DROP2` closes that exchange shell first: the dropper receives self `GC::EXCHANGE END` before the normal drop response burst, and the paired peer receives queued `GC::EXCHANGE END` before the visible ground add/ownership pair for the new temporary handle. The item drop still follows the same inventory, quickslot, persistence, and temporary-ground-handle rules above, and no exchange finalization or trade transfer frame is produced.

The first gold-drop runtime slice owns the gold amount fields on `CG::ITEM_DROP` / `CG::ITEM_DROP2` as a bootstrap currency-only ground entry:

1. If the packet gold/elk field is non-zero, the runtime treats the request as a gold drop and does not mutate the carried item slot/count field. For this currency-only bootstrap path, the packed item position is ignored after the non-zero currency field is selected, so equipment-window or otherwise non-carried positions still follow the gold-drop path rather than being rejected by carried-item validation.
2. The selected player's live and persisted gold are decremented by the requested amount. Zero, over-balance, and out-of-range amounts fail closed with no response and no mutation.
3. The selected session receives `GC::PLAYER_POINT_CHANGE` for point type `11` with the negative amount and updated gold total, then one `GC::ITEM_GROUND_ADD` at the selected character's current coordinates and `GC::ITEM_OWNERSHIP` naming the dropping character.
4. The bootstrap ground-add `vnum` for gold is currently fixed to `1`, matching the first owned currency marker while richer client display/count semantics remain deferred.
5. Gold ground pickup is accepted for the still-pending bootstrap gold marker when the collector is in the same visible world and within that marker's owned pickup reach. The default reach is `300` coordinate units; when the loaded template index includes a valid bootstrap currency marker (`vnum = 1`) with non-zero `pickup_range`, the gold marker uses that template-authored reach just like item handles use their dropped-item template reach. The collector receives `GC::ITEM_GROUND_DEL` followed by `GC::PLAYER_POINT_CHANGE` for point type `11` with the positive amount and updated gold total. If restoring the gold would exceed the current legacy point-change carrier range (`int32` positive max), pickup fails closed before frames, persistence, or temporary marker removal; the still-pending marker remains retryable after the recipient state becomes valid. The selected character snapshot is persisted before the temporary gold marker is removed, and replayed pickup fails closed.
6. Runtime/operator ground snapshots expose pending gold markers as `vnum = 1`, `gold_amount = <dropped amount>`, and no item-stack `count`; ordinary item ground snapshots continue to expose their stack `count` and omit `gold_amount`.
7. If a visible collector picks up a gold marker while exclusive ownership is still active for another live session, the bootstrap runtime now fails closed: no frames, no owner or collector currency mutation, and the marker remains pending for the owner. After the exclusive ownership timer elapses and the blank public-release ownership frame has been emitted, the same visible collector may pick the marker up as ordinary self pickup (`GC::ITEM_GROUND_DEL`, collector gold point-change, normal/self `GC::ITEM_GET(arg=0)`). When the loaded template index includes the bootstrap currency marker `vnum = 1` and that template is marked `anti_give`, public peer pickup after release still follows the template-authored pickup rejection text path before collector currency mutation; the marker remains pending for a later valid retry. The previous always-on visible-peer gold owner-delivery approximation is retired until a later slice owns real party membership.
8. Gold despawn timing, richer gold-marker display/count semantics, durable shared-world ground currency entities, restart-restored ownership timer state, and fallback delivery when owner currency persistence fails remain deferred.

For the first visible-peer pickup runtime slice, accepted pickup is visible-world scoped:

1. Accepted drops are registered as temporary bootstrap ground handles at the dropper's current effective map/position after the selected character mutation is persisted; zero-HP/dead owners, locked item snapshots, snapshots still marked equipped, and unequipped snapshots that still carry authored equipment-slot metadata are rejected at the shared-world ground-handle seam so death-state races or stale lock/equipment-state snapshots cannot create new visible handles. The recipient-side pickup placement boundary likewise rejects locked ground snapshots before merge or fresh-slot placement, so a stale locked ground handle cannot be reclaimed into carried inventory.
2. The dropper receives the same direct `GC::ITEM_GROUND_ADD` + named `GC::ITEM_OWNERSHIP` already owned by the first drop slice, currently visible living peers receive one queued add/ownership pair for the same handle, and later visible sessions that enter `GAME` while the handle remains pending receive that same add/ownership pair after self bootstrap, peer-player bootstrap, and visible static-actor bootstrap frames. Zero-HP/dead peers stay registered for their own restart/teardown flow, but they are skipped as recipients for new ground-handle visibility fanout. Each newly registered pending handle starts an in-memory exclusive ownership window of `30` seconds from registration time (`bootstrapGroundItemOwnershipDuration`). While that window is active, only the owning session may accept `ITEM_PICKUP` for the handle. When any live session flushes pending server frames after the window elapses, the shared-world registry releases exclusive ownership for due handles, emits one blank `GC::ITEM_OWNERSHIP` (`owner_name` empty) to currently visible living peers around each released handle, and later AOI/transfer rebuilds for a publicly released handle also emit the blank ownership label instead of the original owner name.
3. `ITEM_PICKUP` is accepted when its `vid` matches a still-pending bootstrap ground handle in the collector's visible world and the collector is within that handle's owned pickup reach. The default reach is `300` coordinate units; item handles dropped from an authored template with non-zero `pickup_range` use that template-authored reach instead, including fixed drop-vnum reward handles created by the non-player reward seam, and bootstrap gold markers use the same override when the loaded currency-marker template (`vnum = 1`) authors non-zero `pickup_range`. Visible-but-out-of-reach attempts fail closed with no frames, no inventory/gold mutation, and the temporary handle left pending.
4. The picked ground item snapshot itself must validate as a well-formed, unlocked, unequipped carried item before any compatible-stack merge or fresh-slot placement is attempted. Malformed ground snapshots, including snapshots still marked `equipped`, snapshots still marked `locked`, or fallback/no-template ground stacks whose count exceeds the current legacy item-refresh byte range (`255`), fail closed before recipient inventory mutation, pickup notice, persistence, or temporary ground-handle removal.
5. When the picked item has authored loaded template metadata, that template must exist, must be valid, and its `vnum` must match the ground item `vnum` before template-authored stack metadata is applied. Authored equipment templates (`equip_slot` on a valid non-stackable `max_count = 1` template) are accepted by pickup as carried inventory items; pickup never auto-equips them. Missing, malformed, or mismatched authored template metadata still fails closed before any inventory mutation, pickup notice, persistence, or temporary ground-handle removal. Templates that author `pickup_reject_message` without an owned pickup rejection guard fail closed at item-template load/runtime boot, before any gameplay pickup path can ignore or misuse that text. The deterministic bootstrap fallback snapshot used when no item-template file exists remains valid for local smoke/runtime startup; this missing-template guard applies after an authored item-template snapshot has been loaded.
6. Template-authored selected-character and transfer restrictions fail closed before the recipient inventory mutates. The template-backed player pickup placement helper rejects `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, authored job/sex restrictions (`anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, or `anti_male`/`anti_female`), `min_level` restrictions for the recipient, and ground stacks whose live count is already above the resolved template-authored `max_count`. The minimal runtime now freezes `anti_get` / `anti_drop` / `anti_give` / `anti_sell` or restricted self pickup through the normal packet path as one `CHAT_TYPE_INFO` rejection sourced from template-authored `pickup_reject_message` when non-empty, otherwise the inventory-full fallback text, while leaving the temporary ground handle pending for a later valid retry; over-`max_count` ground stacks fail closed with no frames and also leave the pending handle available for a later valid retry.
7. If the picked item's loaded template is marked `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, or `anti_stack`, or carries an authored selected-character restriction for the collector, pickup is rejected before recipient mutation or ground-handle removal. The bootstrap runtime returns one self-only `CHAT_TYPE_INFO` message sourced from template-authored `pickup_reject_message` when present and otherwise the inventory-full fallback, then leaves the pending handle available for a later valid retry.
8. When the picked item is stackable and a carried compatible, unlocked, non-equipped stack can absorb the full ground count, pickup merges into the lowest such carried slot and refreshes it with `GC::ITEM_UPDATE`. This merge keeps existing item, skill, and command quickslots for the refreshed target carried cell unchanged and emits no quickslot synchronization frames, because pickup does not remove or retarget that target stack identity. When loaded template metadata exists for that `vnum`, the merge `GC::ITEM_UPDATE` preserves the template-authored `sockets` and `attributes` arrays while changing only the count.
9. If a compatible stack cannot absorb the pickup, the first minimal runtime chooses the original carried slot when available, or the lowest free carried inventory cell otherwise, and sends a `GC::ITEM_SET` for the placed item. The ordinary pickup notice is a small self-only `GC::ITEM_GET` with `arg = 0`.
10. If no single stack can absorb the full count, stackable pickup fills compatible, unlocked partial stacks in carried-slot order and then places any remaining count into the original carried slot when that slot is empty, or the lowest empty carried inventory slot otherwise. Locked carried stacks are skipped by pickup merging and remain unchanged. Each compatible-stack count refresh produced by this fill path uses `GC::ITEM_UPDATE` and preserves loaded template-authored `sockets` / `attributes` metadata.

11. If neither compatible-stack capacity nor a carried inventory slot can accept the whole picked count, pickup fails without mutating or persisting the collector inventory and leaves the temporary ground handle pending. The selected session receives the bootstrap inventory-full `CHAT_TYPE_INFO` system message (`"You have too many items."`).
12. On accepted pickup, the collector's selected character snapshot is persisted through the same account-store path used by drops before the handle is removed from the temporary ground table. If that account-store write fails, pickup fails closed: the collector live inventory/gold snapshot rolls back, no success frames are emitted, the temporary ground handle remains pending for a later valid retry, and the persisted account snapshot remains unchanged.
13. If the collector has an active same-socket bootstrap exchange shell, accepted pickup closes that shell before pickup frames: the collector receives self `GC::EXCHANGE END` before the `GC::ITEM_GROUND_DEL`, item refresh, point-change when gold is picked up, and `GC::ITEM_GET` frames; the paired peer receives one queued `GC::EXCHANGE END` before any queued visible-ground delete. Template-backed pickup rejection feedback and the inventory-full fallback close the active exchange shell before the self-only info chat while leaving the temporary handle pending.
14. Without an active exchange shell, the collector receives self `GC::ITEM_GROUND_DEL` first, then the deterministic carried inventory refresh frames (`GC::ITEM_UPDATE` and/or `GC::ITEM_SET`), then normal/self `GC::ITEM_GET(vnum, count, arg=0, from_name="")`; the collector does not also receive a duplicate queued `GC::ITEM_GROUND_DEL` through the async server-frame path, while other visible sessions receive one queued `GC::ITEM_GROUND_DEL`.
15. While a temporary handle remains pending, later radius-AOI `MOVE` / `SYNC_POSITION` visibility transitions rebuild it for the moving/syncing session: crossing into the handle's visible world queues `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after ordinary player/static visibility transition frames, and crossing out queues `GC::ITEM_GROUND_DEL` after ordinary transition frames. The visibility calculation is owned by `internal/worldruntime` through the same configured topology/AOI policy used by player and static-actor scopes; `internal/minimal` only maps the resulting ground-item diff back to wire frames. Malformed bootstrap ground handles with `VID == 0` are omitted from worldruntime visible-ground snapshots, visibility diffs, and map-occupancy augmentation.
16. Gameplay-triggered exact-position transfer also rebuilds pending ground-item visibility for the moved session as part of the immediate self rebootstrap result: source-map handles no longer visible to the destination emit `GC::ITEM_GROUND_DEL`, and destination handles newly visible after transfer emit `GC::ITEM_GROUND_ADD` followed by `GC::ITEM_OWNERSHIP` after the existing self bootstrap, peer, and static-actor transfer frames.
17. While exclusive ownership is still active, a non-owner visible collector's `ITEM_PICKUP` fails closed with no frames, no inventory/gold mutation, no owner refresh, and the temporary ground handle left pending. After public ownership release, the same collector uses ordinary collector-side pickup: the collector receives `GC::ITEM_GROUND_DEL`, the deterministic collector inventory refresh frames, and normal/self `GC::ITEM_GET(arg=0, from_name="")`; other visible sessions receive one queued `GC::ITEM_GROUND_DEL`. Party-shaped owner-delivery notices and real party membership remain deferred.
18. If the owner reaches the bootstrap HP floor before reclaiming a still-exclusively-owned handle, non-owner collectors continue to fail closed until public ownership release. After release, ordinary collector-side pickup applies even if the original owner is dead or gone: the collector receives `GC::ITEM_GROUND_DEL`, the deterministic collector inventory refresh frames, and normal/self `GC::ITEM_GET(arg=0, from_name="")`; no owner refresh or pickup-notice frames are queued to a dead/absent owner, and the dead owner's persisted inventory remains in its dropped/empty state.
19. If the picked item's loaded template is marked `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, or `anti_stack`, or carries authored selected-character restrictions for the collector, public ordinary pickup rejects before mutating the collector inventory. The collector receives the same template-backed pickup rejection text used for self pickup (`pickup_reject_message` when non-empty, otherwise the inventory-full fallback), and the temporary ground handle remains pending so a later valid collector or the original owner can retry.
20. If neither compatible-stack capacity nor a carried inventory slot can accept the whole picked count on ordinary public pickup, the runtime rejects the pickup with the same inventory-full `CHAT_TYPE_INFO` message to the collector and leaves the temporary ground handle pending for a later retry.
21. Replayed, unknown, invisible, exclusively-owned-by-another-session, or zero-HP/dead-collector pickups fail closed with no frames; zero-HP/dead collectors also fail closed on the runtime ground-handle visibility lookup before pickup resolution; no-free-slot pickups use the inventory-full info chat path above.

Dropped ground entries are still bootstrap-scoped rather than durable shared-world entities. Reconnecting does not restore them as ground entities, and broader despawn/range policy remains future work. Exclusive ownership timers are likewise bootstrap/in-memory only until a later persistence slice owns restart restoration. Real party membership and party-shaped owner-delivery remain deferred; the previous always-on visible-peer owner-delivery approximation is retired now that exclusive ownership plus public release are owned.

Reference-oracle evidence: the TMP4-compatible client exposes `SendItemDropPacket`, `SendItemDropPacketNew`, and `SendItemPickUpPacket` on the game socket, and consumes `GC::ITEM_GROUND_ADD` / `GC::ITEM_GROUND_DEL` to create and remove client-side ground item actors plus `GC::ITEM_OWNERSHIP` to label item ownership. The external legacy behavior oracle uses an exclusive ownership event (default `30` seconds for Europe-shaped drops / mob rewards, longer only for special system-drop cases) that rejects non-owner pickup while active and emits a blank ownership packet on expiry before public pickup. This repository owns only the project-written field layouts and the bootstrap in-memory exclusive/public transition above.

Current coverage:

- `internal/proto/item` freezes encode/decode round-trips for `ITEM_DROP`, `ITEM_DROP2`, `ITEM_PICKUP`, `ITEM_GROUND_ADD`, `ITEM_GROUND_DEL`, `ITEM_OWNERSHIP`, and normal/party-shaped `ITEM_GET`, plus unexpected-header and invalid-payload rejection for the new codecs.
- `internal/game` freezes `GAME`-phase dispatch for `ITEM_DROP`, `ITEM_DROP2`, and `ITEM_PICKUP`, including the shared-header `ITEM_USE` / `ITEM_DROP` payload-size split.
- `internal/minimal` accepts carried-item drop requests with self ground-add/ownership echoes, gold drops from the `elk`/`gold` packet field, self/ordinary pickup of pending gold markers, template-authored `pickup_range` reach overrides for authored `vnum = 1` gold markers, exclusive ownership timers with blank public-release ownership frames, fail-closed non-owner pickup during the exclusive window, and ordinary public collector pickup after release including template-backed `anti_give` rejection for peer gold pickup after release.
- The same runtime accepts visible-world pickup of temporary bootstrap item handles only when the collector is alive, in range, allowed by loaded item-template metadata, and either owns the exclusive window or the handle has already been publicly released, and now closes active same-socket exchange shells before accepted pickup frames or pickup-rejection info chat.
- Pickup merges into already-known compatible carried stacks now preserve loaded template-authored socket/attribute display metadata in `ITEM_UPDATE` refreshes while changing only the count.
- The runtime keeps existing guards for missing/malformed/mismatched authored pickup templates, loaded `anti_stack` pickup templates, template-authored `drop_reject_message` text for player-requested transfer-guarded drops, guarded template-authored `pickup_reject_message` text for template-backed pickup transfer/selected-character rejections, fail-closed runtime boot for unguarded pickup rejection text, zero-HP/dead collectors, no-placement inventory-full failures, owner-session cleanup, AOI ground-handle rebuilds, and `/local/maps` / relocation-preview / transfer ground snapshots.
- Stale/reclaimed carried-item drop sockets stay self-local: they can refresh their own old socket with item/quickslot removal frames, but they no longer register or emit bootstrap ground handles and they leave the fresh authoritative session/account snapshot unchanged.
