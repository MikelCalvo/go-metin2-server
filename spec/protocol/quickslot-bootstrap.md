# Quickslot bootstrap packet codecs and runtime edits

This note freezes the first wire-codec and `GAME`-phase dispatch contract for quickslot packets, the first persisted character snapshot field needed to carry quickslot state from auth ticket to game session, the first loading-time selected-character quickslot bootstrap burst, the first accepted self-only runtime quickslot edit path, the first automatic carried-inventory item-move quickslot synchronization path, and the same-socket active exchange-shell teardown boundary for accepted quickslot edits.

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

`SyncQuickslot(QUICKSLOT_TYPE_ITEM, old_cell, new_cell)` updates item quickslots when carried inventory items move between carried inventory cells, and deletes matching item quickslots when `new_cell = 255`. The current Go slices own the first update half of that behavior for accepted carried-inventory `ITEM_MOVE` packets and the bootstrap `/inventory_move <from> <to>` compatibility seam when the source item moves to another carried cell, including deletion of stale item quickslots that already pointed at the destination cell before the source quickslot is retargeted there, plus deletion for accepted carried-to-equipment `ITEM_MOVE` equips, the bootstrap `/equip_item` command seam, accepted last-stack carried-inventory `ITEM_USE` packets, and accepted whole-stack merchant sell packets.

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

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid edits for the selected live character, persists the updated quickslot snapshot, and returns self-only quickslot refresh frames. If that account-store write fails, the edit fails closed: live quickslots roll back, no refresh frames are emitted, and the persisted account snapshot remains unchanged. If the same socket is paired in the bootstrap exchange shell, an accepted edit first closes that shell with self `GC::EXCHANGE END` plus one queued peer `GC::EXCHANGE END`, clears the in-memory exchange display/accept state, and then returns the self-only quickslot refresh frames. Item-type quickslots are valid only when their `slot.pos` points at exactly one unlocked, well-formed occupied carried inventory item for the selected live character. When an authored item-template snapshot is loaded and contains metadata for that carried item, the item `vnum` must match that resolved template and the live stack count must not exceed the template-authored `max_count`; when an authored snapshot is loaded and omits that carried item `vnum`, the binding fails closed rather than falling back to unowned metadata. The deterministic missing-file/empty-store bootstrap fallback keeps older local smoke fixtures usable without requiring every ad-hoc test `vnum` to be templated. Empty cells, locked carried items, malformed item snapshots, missing/mismatched authored template metadata, over-template-max live stacks, and duplicate live occupancy of the requested carried cell fail closed with no frames or snapshot mutation. When a new quickslot targets a tuple already referenced by another quickslot of the same type, the older binding is deleted first with `GC::QUICKSLOT_DEL`, then the new binding is returned with `GC::QUICKSLOT_ADD`; tuple retargeting is type-scoped, so an item binding for slot byte `5` does not delete an unrelated skill or command binding whose byte payload is also `5`. A `slot.type = 0` / `QUICKSLOT_TYPE_NONE` add with `slot.pos = 0` acts as a compatibility clear for an existing quickslot at `pos`: the runtime persists the deletion and returns self-only `GC::QUICKSLOT_DEL(pos)` instead of a `GC::QUICKSLOT_ADD`. A type-none add with non-zero `slot.pos` fails closed with no frames and no snapshot mutation, matching the file-backed snapshot rule that none-tuples cannot carry stale payload bytes. Skill quickslots are limited to slots `0..199`, and command quickslots are limited to slots `0..59`; out-of-range edits fail closed with no frames or snapshot mutation. Invalid edits fail closed with no frames. If bootstrap combat retaliation has already driven the selected live character to the zero-HP floor, `QUICKSLOT_ADD` fails closed with no frames and no persisted quickslot or inventory mutation until the character is restarted.

### Client `QUICKSLOT_DEL` (`0x050A`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index to clear |

Total frame length: `5` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid delete requests for an existing quickslot on the selected live character, persists the updated quickslot snapshot, and returns self-only `GC::QUICKSLOT_DEL`. If the same socket is paired in the bootstrap exchange shell, an accepted delete first closes that shell with self/peer `GC::EXCHANGE END`, clears the exchange display/accept state, and then returns the self-only quickslot delete refresh. Invalid positions and empty-position deletes fail closed with no frames or snapshot mutation. If bootstrap combat retaliation has already driven the selected live character to the zero-HP floor, `QUICKSLOT_DEL` fails closed with no frames and no persisted quickslot or inventory mutation until the character is restarted.

### Client `QUICKSLOT_SWAP` (`0x050B`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | first quickslot index |
| `pos_to` | `uint8` | second quickslot index |

Total frame length: `6` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the minimal runtime accepts valid swaps for the selected live character when at least one side is occupied, persists the updated quickslot snapshot, and returns self-only `GC::QUICKSLOT_SWAP`. If the same socket is paired in the bootstrap exchange shell, an accepted swap first closes that shell with self/peer `GC::EXCHANGE END`, clears the exchange display/accept state, and then returns the self-only quickslot swap refresh. Swapping an occupied position with an empty valid position moves the occupied quickslot to the empty target position. Invalid positions, same-position no-op swaps, and swaps where both valid positions are empty fail closed with no frames or snapshot mutation. If bootstrap combat retaliation has already driven the selected live character to the zero-HP floor, `QUICKSLOT_SWAP` fails closed with no frames and no persisted quickslot or inventory mutation until the character is restarted.

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

Missing `quickslots` in older file-backed snapshots is normalized to an empty array. Durable account and one-shot login-ticket snapshots fail closed on malformed quickslot state: duplicate quickslot positions, duplicate non-item same-type `{type, slot}` bindings for skill/command quickslots at different bar positions, positions outside the owned `0..35` bar range, unknown quickslot types, stale `type = 0` slots with non-zero payload, item slots outside carried inventory, skill slots outside `0..199`, and command slots outside `0..59` are rejected on both save and load. The duplicate-tuple guard is type-scoped, so item, skill, and command quickslots may still share the same byte `slot` value when their `type` differs. Duplicate item quickslot tuples remain loadable for now so existing item-removal cleanup/recovery fixtures that clear every item quickslot for a removed carried cell remain covered. This keeps authd -> gamed ticket handoff and account-store round trips deterministic before any accepted runtime quickslot mutation is replayed from disk.

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

This keeps bootstrap quickslots self-only and snapshot-derived. Runtime `ADD` / `DEL` / `SWAP` edits are also self-only in this slice: the selected live character is mutated, the selected character snapshot is persisted back to the account store, and the server returns the matching quickslot refresh frame to the same session. If the same socket has an active bootstrap exchange shell, accepted quickslot edits close that shell before the quickslot refresh, queue the paired peer's `GC::EXCHANGE END`, and perform no exchange item/gold transfer or finalization result. If an older reclaimed socket sends a valid-looking quickslot edit after a fresh session has become authoritative for the same selected character, the stale socket may still receive its self-local quickslot refresh frames, but the authoritative persisted account snapshot and fresh live session state stay unchanged.

## Item synchronization ownership

When an accepted carried-inventory `ITEM_MOVE` leaves the source cell empty and moves the item to another carried inventory cell, including incompatible occupied-destination full-stack swaps, the minimal runtime scans the selected character's live quickslots for item tuples matching the old cell or the destination cell. The bootstrap `/inventory_move <from> <to>` compatibility seam now applies the same scan only after the same authored-template incompatible-swap guard boundary used by packet `ITEM_MOVE` has accepted the carried-cell move. Destination-cell item quickslots are deleted first with `GC::QUICKSLOT_DEL(position)` so stale quickslot ownership does not survive a merge, move, or swap into that cell. Each matching source-cell item quickslot is then updated to the new cell, persisted with the same selected-character snapshot mutation as the item move, and appended to the self response as `GC::QUICKSLOT_ADD(position, {item, new_cell})` after the item delete/set refresh frames.

When an accepted carried-to-equipment `ITEM_MOVE` equips a carried item and clears the source carried cell, the minimal runtime now applies the same item-removal quickslot synchronization. Each matching item quickslot is deleted, persisted with the same selected-character point-bearing item mutation as the equip, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the item/equipment/appearance refresh frames.

The bootstrap `/equip_item <from> <equip_slot>` command seam uses the same source-slot item quickslot deletion rule as packet-originated equip. Matching item quickslots for the cleared carried source cell are deleted and persisted with the equip mutation; skill or command quickslots that happen to carry the same byte value are left unchanged.

When an accepted carried-inventory `ITEM_USE` consumes the last item in a stack and the carried slot becomes empty, the minimal runtime scans the selected character's live quickslots for item tuples matching that removed cell. Each matching item quickslot is deleted, persisted with the same selected-character snapshot mutation as the item use, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the `ITEM_DEL` for the removed stack and before the temporary `CHAT_TYPE_INFO` effect placeholder.

When an accepted merchant `SHOP SELL` / `SELL2` removes a whole carried-inventory stack, the minimal runtime now applies the same item-removal quickslot synchronization. Each matching item quickslot is deleted, persisted with the merchant sell selected-character mutation, and appended to the self response as `GC::QUICKSLOT_DEL(position)` after the `ITEM_DEL` for the sold stack and before the gold `PLAYER_POINT_CHANGE`.

The current owned synchronization is intentionally narrow:

- move synchronization applies to accepted carried-inventory mutations where the source cell becomes empty and the moved item now lives at a different carried cell, including exact counted full-stack compatible merges, incompatible occupied-destination full-stack swaps whose source and target cells both pass authored template-count guards, and the bootstrap `/inventory_move <from> <to>` compatibility seam for full-stack carried-cell moves after authored source/target template metadata has passed the same incompatible-swap guard boundary;
- when that destination carried cell already has matching item quickslots, only those destination quickslots are deleted before the moved source quickslot is retargeted so one carried cell does not retain multiple stale item quickslot bindings; unrelated item quickslots for other carried cells stay unchanged;
- removal synchronization applies to accepted carried-to-equipment `ITEM_MOVE` equips, the bootstrap `/equip_item` command seam, accepted last-stack carried-inventory `ITEM_USE` paths, full-source `ITEM_USE_TO_ITEM` merges, accepted whole-stack merchant sell paths where the carried item slot becomes empty, accepted `probability = 100` refine confirm when a material carried cell is fully consumed, mutual-accept exchange finalize when a displayed whole stack leaves its source carried cell, and accepted open-presentation `SAFEBOX_CHECKIN` when the whole carried stack leaves its inventory cell for same-session in-memory safebox storage;
- removal synchronization rejects non-carried source cells fail-closed before live or persisted quickslot mutation;
- it does not rewrite or delete skill or command quickslots that happen to carry the same byte value;
- move/removal synchronization does not run for partial merges or partial-stack splits where the original item still remains at the source cell, including partial `ITEM_MOVE` counted merges, partial counted `ITEM_DROP2`, partial `ITEM_USE_TO_ITEM` stack consolidation, and partial refine-confirm material stack decrements;
- merchant partial-stack `SELL2` does not delete quickslots, because the original item still remains at the source cell;
- accepted `SAFEBOX_CHECKOUT` and same-session `SAFEBOX_ITEM_MOVE` do not delete or retarget carried-inventory item quickslots (check-out places/merges into carried cells without clearing a carried source binding; item-move mutates only in-memory safebox cells);
- it does not yet delete item quickslots when item timeout, destruction, mall, or other item-removal paths outside the currently owned removal set clear an item cell.

## Current scope

Implemented now:

- Go codecs for client `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Go codecs for server `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Strict header and payload-size validation for those client and server packets.
- `GAME`-phase dispatch hooks for client quickslot edit packets.
- file-backed account and login-ticket snapshot round trips for bootstrap quickslot arrays.
- loading-time selected-character `QUICKSLOT_ADD` bootstrap frames for persisted quickslot arrays, emitted after the selected-character presence/state burst and before trailing peer/static-actor visibility frames.
- accepted self-only runtime mutation for client-originated `CG::QUICKSLOT_ADD` / `DEL` / `SWAP`; item quickslot adds must target exactly one well-formed occupied carried inventory item and, when authored metadata exists for that item or an authored snapshot omits it, stay within the template-backed guard above, so missing/mismatched authored metadata and over-template-max live stacks now reject fail-closed alongside malformed item snapshots and duplicate carried-cell occupancy; retargeting the same item/skill/command tuple to a new quickslot position deletes the older same-type tuple binding first without deleting other quickslot types that share the same byte payload; durable snapshots now reject duplicate skill/command tuple bindings so file-backed account/login-ticket state cannot bypass those non-item retarget invariants; `QUICKSLOT_ADD` with `slot.type = 0` and `slot.pos = 0` clears an existing quickslot by returning `GC::QUICKSLOT_DEL`; non-zero type-none payloads fail closed without deleting that bar position; client-originated deletes must target an existing quickslot position; and client-originated swaps require at least one occupied quickslot position.
- accepted runtime updates to persisted quickslot state.
- active same-socket bootstrap exchange-shell teardown before accepted quickslot add/delete/swap refresh frames, without exchange transfer/finalization mutation.
- player-death/floor gating for quickslot edits: once bootstrap combat retaliation has driven the selected live character to the zero-HP floor, client-originated quickslot add/delete/swap edits fail closed with no frames and no persisted quickslot or inventory mutation until restart recovery.
- stale reclaimed quickslot edit sockets remain self-local: they can receive deterministic quickslot refresh frames for their own socket, but they do not replace the authoritative persisted account snapshot or fresh live session state.
- automatic item quickslot update synchronization after accepted carried-inventory `ITEM_MOVE` packets that empty the source cell, including compatible full-stack merges, incompatible occupied-destination swaps, and destination-cell stale quickslot deletion when needed.
- automatic item quickslot update synchronization after accepted bootstrap `/inventory_move <from> <to>` full-stack carried-cell moves that empty the source cell and place the moved item in another carried cell, including deletion of stale destination-cell item quickslots before source quickslots are retargeted.
- automatic item quickslot deletion synchronization after accepted carried-to-equipment `ITEM_MOVE` equips and the bootstrap `/equip_item` command seam.
- automatic item quickslot deletion synchronization after accepted last-stack carried-inventory `ITEM_USE` packets.
- automatic item quickslot deletion synchronization after full-source `ITEM_USE_TO_ITEM` stack consolidations, while partial consolidations keep source-slot item quickslots unchanged.
- automatic item quickslot deletion synchronization after accepted whole-stack carried-inventory `ITEM_DROP` / `ITEM_DROP2` packets, while partial counted drops keep source-slot item quickslots unchanged.
- stale/reclaimed carried-item drop sockets keep that deletion synchronization self-local only: the old socket can receive `GC::QUICKSLOT_DEL` for its own cleared source-cell item quickslots, but the authoritative persisted account snapshot and fresh live session quickslots stay unchanged and no bootstrap ground handle is registered from the stale mutation.
- accepted ground-item pickup stack merges refresh compatible target carried stacks without deleting or retargeting existing item quickslots for that target cell; pickup does not emit quickslot synchronization frames unless a later dedicated pickup/removal path owns one.
- automatic item quickslot deletion synchronization after accepted whole-stack merchant `SHOP SELL` / `SELL2` packets.
- automatic item quickslot deletion synchronization after accepted `probability = 100` refine confirm when a material carried cell is fully consumed: each matching item quickslot is deleted with the refine mutation, persisted with the selected-character snapshot, and emitted as self-only `GC::QUICKSLOT_DEL(position)` after the material `ITEM_DEL` / `ITEM_UPDATE` refreshes and before the result-cell `ITEM_SET` and gold `PLAYER_POINT_CHANGE`; partial material decrements leave that cell's item quickslots unchanged.
- automatic item quickslot deletion synchronization after mutual-accept exchange finalize when a displayed whole stack leaves its source carried cell: each matching source item quickslot is deleted with the trade mutation, persisted on both selected-character snapshots, and emitted as self/queued `GC::QUICKSLOT_DEL(position)` after that side's inventory refresh frames and before any gold `PLAYER_POINT_CHANGE` / shell `END` frames for the finalize burst.
- automatic item quickslot deletion synchronization after accepted open-presentation `SAFEBOX_CHECKIN` when the whole carried stack leaves its inventory cell: each matching source item quickslot is deleted with the check-in mutation, persisted with the inventory/quickslot account snapshot, and emitted as self-only `GC::QUICKSLOT_DEL(position)` after the carried `ITEM_DEL` and before `GC::SAFEBOX_SET`; skill/command quickslots that share the same byte payload remain unchanged.
- validation of bootstrap quickslot positions (`0..35`), item quickslot cells (`0..89`), skill quickslot slots (`0..199`), command quickslot slots (`0..59`), and supported tuple types (`item`, `skill`, `command`).

Not implemented yet:

- automatic item quickslot deletion after item timeout, destruction, mall checkout/removal, or other item-removal paths outside the currently owned removal set
- automatic item quickslot synchronization for belt inventory cells beyond the current carried inventory bootstrap range
