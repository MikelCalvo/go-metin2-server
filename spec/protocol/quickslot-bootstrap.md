# Quickslot bootstrap packet codecs and runtime edits

This note freezes the first wire-codec and `GAME`-phase dispatch contract for quickslot packets, the first persisted character snapshot field needed to carry quickslot state from auth ticket to game session, the first loading-time selected-character quickslot bootstrap burst, the first accepted self-only runtime quickslot edit path, and the first automatic carried-inventory item-move quickslot synchronization path.

## Evidence

The legacy oracle exposes `TQuickslot` as two one-byte fields:

- `type uint8`
- `pos uint8`

It also uses three client packets for player-authored quickslot edits:

- `CG::QUICKSLOT_ADD = 0x0509`
- `CG::QUICKSLOT_DEL = 0x050A`
- `CG::QUICKSLOT_SWAP = 0x050B`

The client send path builds these packets only while the main actor can act, and carries the same quickslot tuple shape for `ADD` as the server refresh packet.

It also uses three server packets for quickslot refreshes:

- `GC::QUICKSLOT_ADD = 0x0519`
- `GC::QUICKSLOT_DEL = 0x051A`
- `GC::QUICKSLOT_SWAP = 0x051B`

`SyncQuickslot(QUICKSLOT_TYPE_ITEM, old_cell, new_cell)` updates item quickslots when carried inventory items move between carried inventory cells, and deletes matching item quickslots when `new_cell = 255`. The current Go slices own the first update half of that behavior for accepted carried-inventory `ITEM_MOVE` packets, including deletion of stale item quickslots that already pointed at an occupied destination cell before the source quickslot is retargeted there, plus deletion for accepted carried-to-equipment `ITEM_MOVE` equips, the bootstrap `/equip_item` command seam, accepted last-stack carried-inventory `ITEM_USE` packets, and accepted whole-stack merchant sell packets.

## Packet layouts

All multi-byte frame fields use the normal repository frame envelope. These payloads are byte-only.

### Client `QUICKSLOT_ADD` (`0x0509`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index |
| `slot.type` | `uint8` | `0 = none`, `1 = item`, `2 = skill`, `3 = command` |
| `slot.pos` | `uint8` | type-relative item cell / skill index / command index |

Total frame length: `7` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid edits for the selected live character, persists the updated quickslot snapshot, and returns self-only `GC::QUICKSLOT_ADD`. Item-type quickslots are valid only when their `slot.pos` points at exactly one occupied carried inventory item for the selected live character; empty cells and duplicate live occupancy of the requested carried cell fail closed with no frames or snapshot mutation. When a new item quickslot targets an item cell already referenced by another item quickslot, the older item quickslot is deleted first with `GC::QUICKSLOT_DEL`, then the new binding is returned with `GC::QUICKSLOT_ADD`; skill/command quickslots that happen to carry the same byte value are not deleted by this item retarget path. Skill quickslots are limited to slots `0..199`, and command quickslots are limited to slots `0..59`; out-of-range edits fail closed with no frames or snapshot mutation. Invalid edits fail closed with no frames.

### Client `QUICKSLOT_DEL` (`0x050A`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index to clear |

Total frame length: `5` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid delete requests for the selected live character, persists the updated quickslot snapshot, and returns self-only `GC::QUICKSLOT_DEL`. Invalid positions fail closed with no frames.

### Client `QUICKSLOT_SWAP` (`0x050B`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | first quickslot index |
| `pos_to` | `uint8` | second quickslot index |

Total frame length: `6` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid swaps for the selected live character, persists the updated quickslot snapshot, and returns self-only `GC::QUICKSLOT_SWAP`. Invalid positions fail closed with no frames.

### Server `QUICKSLOT_ADD` (`0x0519`)

Direction: server -> client.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index |
| `slot.type` | `uint8` | `0 = none`, `1 = item`, `2 = skill`, `3 = command` |
| `slot.pos` | `uint8` | type-relative item cell / skill index / command index |

Total frame length: `7` bytes.

### `QUICKSLOT_DEL` (`0x051A`)

Direction: server -> client.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index to clear |

Total frame length: `5` bytes.

### `QUICKSLOT_SWAP` (`0x051B`)

Direction: server -> client.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | first quickslot index |
| `pos_to` | `uint8` | second quickslot index |

Total frame length: `6` bytes.

## Snapshot ownership

The bootstrap account and login-ticket character snapshots now carry a `quickslots` array with the same byte-sized fields as the wire tuple:

| Field | Type | Notes |
| --- | --- | --- |
| `position` | `uint8` | quickslot bar index |
| `type` | `uint8` | `0 = none`, `1 = item`, `2 = skill`, `3 = command` |
| `slot` | `uint8` | type-relative item cell / skill index / command index |

Missing `quickslots` in older file-backed snapshots is normalized to an empty array. This preserves authd -> gamed ticket handoff and account-store round trips before any accepted runtime quickslot mutation is enabled.

## Loading-time bootstrap ownership

When `ENTERGAME` moves the selected character from `LOADING` to `GAME`, the current selected-character bootstrap burst now appends one server `QUICKSLOT_ADD` frame for each persisted quickslot on that selected character.

The owned bootstrap ordering is:

1. selected-character presence/state frames:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
2. selected-character persisted quickslot frames, sorted by `position` ascending:
   - `QUICKSLOT_ADD(position, {type, slot})...`
3. trailing visible peer/static-actor frames, if any

This keeps bootstrap quickslots self-only and snapshot-derived. Runtime `ADD` / `DEL` / `SWAP` edits are also self-only in this slice: the selected live character is mutated, the selected character snapshot is persisted back to the account store, and the server returns the matching quickslot refresh frame to the same session.

## Item synchronization ownership

When an accepted carried-inventory `ITEM_MOVE` leaves the source cell empty and moves the item to another carried inventory cell, the minimal runtime now scans the selected character's live quickslots for item tuples matching the old cell or the destination cell. Destination-cell item quickslots are deleted first with `GC::QUICKSLOT_DEL(position)` so stale quickslot ownership does not survive a merge or swap into that cell. Each matching source-cell item quickslot is then updated to the new cell, persisted with the same selected-character snapshot mutation as the item move, and appended to the self response as `GC::QUICKSLOT_ADD(position, {item, new_cell})` after the item delete/set refresh frames.

When an accepted carried-to-equipment `ITEM_MOVE` equips a carried item and clears the source carried cell, the minimal runtime now applies the same item-removal quickslot synchronization. Each matching item quickslot is deleted, persisted with the same selected-character point-bearing item mutation as the equip, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the item/equipment/appearance refresh frames.

The bootstrap `/equip_item <from> <equip_slot>` command seam uses the same source-slot item quickslot deletion rule as packet-originated equip. Matching item quickslots for the cleared carried source cell are deleted and persisted with the equip mutation; skill or command quickslots that happen to carry the same byte value are left unchanged.

When an accepted carried-inventory `ITEM_USE` consumes the last item in a stack and the carried slot becomes empty, the minimal runtime scans the selected character's live quickslots for item tuples matching that removed cell. Each matching item quickslot is deleted, persisted with the same selected-character snapshot mutation as the item use, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the `ITEM_DEL` for the removed stack and before the temporary `CHAT_TYPE_INFO` effect placeholder.

When an accepted merchant `SHOP SELL` / `SELL2` removes a whole carried-inventory stack, the minimal runtime now applies the same item-removal quickslot synchronization. Each matching item quickslot is deleted, persisted with the merchant sell selected-character mutation, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the `ITEM_DEL` for the sold stack and before the gold `PLAYER_POINT_CHANGE`.

The current owned synchronization is intentionally narrow:

- move synchronization applies to accepted carried-inventory mutations where the source cell becomes empty and the moved item now lives at a different carried cell, including exact counted full-stack compatible merges;
- when that destination carried cell already has matching item quickslots, only those destination quickslots are deleted before the moved source quickslot is retargeted so one carried cell does not retain multiple stale item quickslot bindings; unrelated item quickslots for other carried cells stay unchanged;
- removal synchronization applies to accepted carried-to-equipment `ITEM_MOVE` equips, the bootstrap `/equip_item` command seam, accepted last-stack carried-inventory `ITEM_USE` paths, full-source `ITEM_USE_TO_ITEM` merges, and accepted whole-stack merchant sell paths where the carried item slot becomes empty;
- removal synchronization rejects non-carried source cells fail-closed before live or persisted quickslot mutation;
- it does not rewrite or delete skill or command quickslots that happen to carry the same byte value;
- move/removal synchronization does not run for partial merges or partial-stack splits where the original item still remains at the source cell, including partial `ITEM_MOVE` counted merges, partial counted `ITEM_DROP2`, and partial `ITEM_USE_TO_ITEM` stack consolidation;
- merchant partial-stack `SELL2` does not delete quickslots, because the original item still remains at the source cell;
- it does not yet delete item quickslots when safebox, exchange, item timeout, destruction, trade, movement to non-carried storage, or other item-removal paths clear an item cell.

## Current scope

Implemented now:

- Go codecs for client `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Go codecs for server `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Strict header and payload-size validation for those client and server packets.
- `GAME`-phase dispatch hooks for client quickslot edit packets.
- file-backed account and login-ticket snapshot round trips for bootstrap quickslot arrays.
- loading-time selected-character `QUICKSLOT_ADD` bootstrap frames for persisted quickslot arrays, emitted after the selected-character presence/state burst and before trailing peer/static-actor visibility frames.
- accepted self-only runtime mutation for client-originated `CG::QUICKSLOT_ADD` / `DEL` / `SWAP`; item quickslot adds must target exactly one occupied carried inventory item, duplicate carried-cell occupancy is rejected fail-closed, and retargeting the same item cell to a new quickslot position deletes the older item quickslot first.
- accepted runtime updates to persisted quickslot state.
- automatic item quickslot update synchronization after accepted carried-inventory `ITEM_MOVE` packets that empty the source cell, including destination-cell stale quickslot deletion when needed.
- automatic item quickslot deletion synchronization after accepted carried-to-equipment `ITEM_MOVE` equips and the bootstrap `/equip_item` command seam.
- automatic item quickslot deletion synchronization after accepted last-stack carried-inventory `ITEM_USE` packets.
- automatic item quickslot deletion synchronization after full-source `ITEM_USE_TO_ITEM` stack consolidations, while partial consolidations keep source-slot item quickslots unchanged.
- automatic item quickslot deletion synchronization after accepted whole-stack carried-inventory `ITEM_DROP` / `ITEM_DROP2` packets, while partial counted drops keep source-slot item quickslots unchanged.
- automatic item quickslot deletion synchronization after accepted whole-stack merchant `SHOP SELL` / `SELL2` packets.
- validation of bootstrap quickslot positions (`0..35`), item quickslot cells (`0..89`), skill quickslot slots (`0..199`), command quickslot slots (`0..59`), and supported tuple types (`item`, `skill`, `command`).

Not implemented yet:

- automatic item quickslot deletion after safebox, exchange, item timeout, destruction, trade, movement to non-carried storage, or other item-removal paths
- automatic item quickslot synchronization for belt inventory cells beyond the current carried inventory bootstrap range
