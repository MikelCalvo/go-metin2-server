# Item exchange bootstrap

This note freezes the first clean-room `EXCHANGE` boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layout before broader player-to-player trade is implemented
- reuse the owned shared server response packet layout for the first runtime exchange-window shell
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- own the first two-party exchange-window shell (`START` / display-only `ITEM_ADD` / display-only `ITEM_DEL` / display-only `GOLD_ADD` / display-only `ACCEPT` / accept reset / accept-time requester plus already-accepted partner displayed-item and displayed-gold revalidation / second-accept receiver inventory-capacity and gold-overflow precondition guard / `END`) without mutating inventory, equipment, quickslots, gold, ground items, or persisted account state, and close that display shell before successful carried-item use, carried/equipment item moves, drag-to-item stack consolidation, carried-item drop refresh frames, or template-backed carried-drop rejection feedback is exposed for an item already shown in it
- keep item ownership transfer/locking, accepted gold mutation, full finalize/result semantics, and trade mutation fail-closed until a later exchange/trade slice owns those semantics
- allow one active-shell template-authored guard response for already-owned `anti_give` metadata on `ITEM_ADD` without implementing item transfer

This is not a completed exchange, trade, safebox, or player-shop system.

## Evidence

The TMP4-compatible client exposes `CG::EXCHANGE = 0x0508` on the main game socket. The client send helpers use one fixed client packet shape for exchange start, item add, item delete, gold add, accept, and cancel requests. The subheader selects the requested exchange action. Its `GC::EXCHANGE ITEM_DEL` receive path clears one exchange display slot from the receiver's self/peer exchange-side model, so this bootstrap slice treats `ITEM_DEL.arg1` as the currently owned display slot to clear. Its gold-add path uses the same `GC::EXCHANGE` family with `arg1` carrying the displayed gold amount, which this slice owns only as display state. Its accept receive path sets the self/peer exchange-window accepted indicator from `arg1`; the legacy server clears both accept indicators with `GC::EXCHANGE ACCEPT(arg1 = 0)` before accepted item/gold display changes, so this bootstrap slice owns that reset marker while still deliberately avoiding final trade mutation.

## Client packet

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::EXCHANGE` (`0x0508`)

Direction: client -> server.

Payload size is 9 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `subheader` | `uint8` | exchange action selector |
| 1 | `arg1` | `uint32 LE` | target `vid`, gold amount, display/delete slot, or zero depending on subheader |
| 5 | `arg2` | `uint8` | item display slot for item-add requests (`0..11` for the current TMP4 exchange UI), otherwise zero/ignored |
| 6 | `pos` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE`; meaningful for item-add requests |

The current owned exchange item-display surface has 12 slots. Runtime guard feedback for `ITEM_ADD` therefore treats `arg2 >= 12` as malformed for the current bootstrap exchange boundary and falls back to the ordinary no-frame/no-mutation rejection.

Total frame length is 13 bytes including the common `header` and `length` fields.

Current owned client subheaders:

| Name | Value | Current runtime policy |
| --- | ---: | --- |
| `START` | `0` | starts the current visible-peer exchange-window shell when the peer is connected and visible |
| `ITEM_ADD` | `1` | display-only item placement inside the current bootstrap exchange shell |
| `ITEM_DEL` | `2` | display-only removal inside the current bootstrap exchange shell |
| `ELK_ADD` | `3` | display-only gold placement inside the active shell when the requested amount is non-zero and not above the requester's live gold; otherwise `LESS_GOLD` or fail-closed |
| `ACCEPT` | `4` | display-only accept marker inside the active shell after the requester's remembered item/gold display state still matches live state and, when the partner had already accepted, the partner's remembered display state also still matches live state and both receivers satisfy the currently frozen finalization preconditions; does not finalize trade |
| `CANCEL` | `5` | closes the current paired exchange-window shell |

The repository owns only this client byte layout, the current visible-peer open/close shell, the first display-only `ITEM_ADD` / `ITEM_DEL` companions, the first display-only `ELK_ADD` companion, the first display-only `ACCEPT` companion, the accept-marker reset emitted before accepted display changes, accept-time requester and already-accepted partner displayed item/gold revalidation, second-accept receiver inventory-capacity and gold-overflow precondition guards, and the current fail-closed policy for final trade-result actions. It does not yet interpret exchange item ownership transfer, accepted gold removal, complete result semantics, or trade finalization.

## Server packet

### `GC::EXCHANGE` (`0x051C`)

Direction: server -> client.

The shared server exchange response frame uses one fixed payload shape for all currently documented server subheaders:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `subheader` | `uint8` | server exchange action/status selector |
| 1 | `is_me` | `uint8` | non-zero when the response describes the receiver's own exchange side |
| 2 | `arg1` | `uint32 LE` | `vnum`, `gold`, accept flag, peer `vid`, display slot, or status-specific scalar |
| 6 | `arg2` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE`; exchange item display positions currently use `RESERVED_WINDOW` |
| 9 | `arg3` | `uint32 LE` | count or status-specific scalar |
| 13 | `sockets` | `[3]int32 LE` | copied from item display metadata for `ITEM_ADD`, zero otherwise |
| 25 | `attributes` | `[7]{type uint8, value int16 LE}` | copied from item display metadata for `ITEM_ADD`, zero otherwise |

Total frame length is 50 bytes including the common `header` and `length` fields.

Current owned server subheaders:

| Name | Value | Current runtime policy |
| --- | ---: | --- |
| `START` | `0` | emitted for the current visible-peer open shell |
| `ITEM_ADD` | `1` | emitted for the current display-only item-add shell while no item ownership transfer is performed |
| `ITEM_DEL` | `2` | emitted for the current display-only item-removal shell while no item ownership transfer is performed |
| `GOLD_ADD` | `3` | emitted for the current display-only gold placement shell while no gold ownership transfer is performed |
| `ACCEPT` | `4` | display-only accepted marker for one side of the active shell |
| `END` | `5` | emitted for the current cancel/close shell |
| `ALREADY` | `6` | emitted self-only when `START` targets a visible connected peer that is already paired in the bootstrap exchange shell |
| `LESS_GOLD` | `7` | emitted self-only when an active-shell gold placement requests more than the requester's live gold, or when an accept attempt finds the requester's previously displayed gold is now above live gold |

The shipped runtime now emits `START`, display-only `ITEM_ADD`, display-only `ITEM_DEL`, display-only `GOLD_ADD`, display-only `ACCEPT`, accept-marker reset packets with `arg1 = 0` before accepted item/gold display changes, self-only `LESS_GOLD`, `END`, and the narrow busy-target `ALREADY` response for the visible-peer exchange-window shell described below. `LESS_GOLD` is used both for over-budget gold display requests and for the current accept-time stale displayed-gold branch of the requester's display-state revalidation guard. Stale requester displayed carried items are rejected silently at accept time instead of emitting a finalization/result frame. When the paired partner had already accepted, the second accept also revalidates the accepted partner's remembered displayed items and displayed gold against the partner's current shared-world character state; partner drift rejects the second accept silently with no result frame and leaves the shell cancellable. The same second-accept seam now also checks the currently frozen receiver-side finalization preconditions before emitting any new accept marker: each receiver must have inventory capacity for the other side's displayed items under the loaded templates, and any incoming displayed gold must fit the receiver's current gold point-change carrier. Receiver precondition failures reject silently with no result frame and leave the shell cancellable. `ACCEPT` is still only a presentation marker; a later trade-state slice must own two-party result semantics and finalization.

## Current runtime contract

`internal/game` decodes `EXCHANGE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime owns only a small in-memory exchange-window shell:

- `START` succeeds only when the requester owns a live `GAME` shared-world session above the bootstrap zero-HP floor, `arg1` identifies another connected visible player `VID`, and neither player is already paired in this bootstrap exchange shell.
- A successful `START` returns one self-only `GC::EXCHANGE START` to the requester with `arg1 = target_vid` and queues one `GC::EXCHANGE START` to the visible target with `arg1 = requester_vid`.
- If `START` targets a visible connected player that is already paired, the requester receives one self-only `GC::EXCHANGE ALREADY`; the existing pair receives no frames and no exchange state changes.
- `CANCEL` succeeds only while the requester is currently paired in that shell.
- A successful `CANCEL` returns one self-only `GC::EXCHANGE END` to the cancelling player and queues one `GC::EXCHANGE END` to the paired player.
- If practice-mob retaliation drives either paired player's live bootstrap HP to `0` while the shell is open, the death edge closes the shell with the same `GC::EXCHANGE END` family: the dying owner receives one self-only `END`, the paired live peer receives one queued `END`, and the in-memory pairing/display state is cleared before later exchange requests can run.
- If same-socket `/quit`, `/logout`, or `/phase_select` begins while the requester is paired in this bootstrap shell, the runtime closes the shell before the lifecycle command completes: the command sender receives one self-only `GC::EXCHANGE END` before the command delivery or phase-transition frame, the paired peer receives one queued `GC::EXCHANGE END`, and the in-memory pairing/display/accept state is cleared without inventory, equipment, quickslot, gold, ground-item, or persisted-account mutation.
- If same-socket `/use_item <slot>` or packet `ITEM_USE` succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the item-use response burst is delivered: the item-use requester receives one self-only `GC::EXCHANGE END` before the slash or packet item-use frames, the paired peer receives one queued `GC::EXCHANGE END`, and the in-memory exchange pairing/display/accept state is cleared. The item-use mutation itself then follows the separately owned `item-use-bootstrap.md` persistence rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket `/use_item <slot>` or packet `ITEM_USE` returns template-authored `use_reject_message` feedback instead of mutating state, the runtime now closes the exchange shell before the self-only info-chat frame is delivered: the requester receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, the requester then receives the authored `CHAT_TYPE_INFO` rejection, and carried inventory, quickslots, points, gold, ground handles, exchange item/gold displays, and persisted account snapshots remain unchanged.
- If same-socket carried-to-carried `ITEM_MOVE` or the local `/inventory_move <from> <to>` harness succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the inventory refresh burst is delivered: the requester receives one self-only `GC::EXCHANGE END` before the item and quickslot refresh frames, the paired peer receives one queued `GC::EXCHANGE END`, and the in-memory exchange pairing/display/accept state is cleared. The carried-slot move itself then follows the separately owned `item-move-bootstrap.md` persistence and quickslot rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket carried-to-equipment or equipment-to-carried `ITEM_MOVE` succeeds while the requester is paired in this bootstrap shell, or the requester uses the local `/equip_item <from> <equip_slot>` or `/unequip_item <equip_slot> <to>` harness, the runtime closes the exchange shell before the equipment mutation refresh burst is delivered: the requester receives one self-only `GC::EXCHANGE END` before the item/equipment/appearance/quickslot refresh frames, the paired peer receives one queued `GC::EXCHANGE END` before any queued peer appearance update, and the in-memory exchange pairing/display/accept state is cleared. The equipment mutation itself then follows the separately owned item-move/equipment persistence and quickslot rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket equipment `ITEM_MOVE` or the local `/equip_item` / `/unequip_item` harness returns template-authored equipment rejection feedback instead of mutating state, the runtime now closes the exchange shell before the self-only info-chat frame is delivered. This covers selected-character/template-guarded equip rejection text and `irremovable` unequip rejection text: the requester receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, the requester then receives the authored `CHAT_TYPE_INFO` rejection, and carried inventory, equipment, quickslots, points, ground handles, exchange item/gold displays, and persisted account snapshots remain unchanged.
- If same-socket `ITEM_USE_TO_ITEM` stack consolidation succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the stack-merge refresh burst is delivered: the requester receives one self-only `GC::EXCHANGE END` before the item/quickslot frames, the paired peer receives one queued `GC::EXCHANGE END`, and the in-memory exchange pairing/display/accept state is cleared. The stack mutation itself then follows the separately owned `item-use-bootstrap.md` persistence rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket carried-item `ITEM_DROP` / `ITEM_DROP2` succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the drop response burst is delivered: the drop requester receives one self-only `GC::EXCHANGE END` before the item/quickslot/ground/ownership frames, and the paired peer receives one queued `GC::EXCHANGE END` before the normal visible ground add/ownership frames for that drop. The carried-item drop mutation itself then follows the separately owned `item-drop-pickup-bootstrap.md` persistence and temporary-ground-handle rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket carried-item `ITEM_DROP` / `ITEM_DROP2` returns template-backed transfer-guard or selected-character rejection feedback while the requester is paired in this bootstrap shell, the runtime now closes the exchange shell before that self-only info-chat frame is delivered: the requester receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, the requester then receives the authored or fallback `CHAT_TYPE_INFO` rejection, and carried inventory, quickslots, points, gold, ground handles, exchange item/gold displays, and persisted account snapshots remain unchanged.
- If same-socket `ITEM_PICKUP` succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the pickup response burst is delivered: the requester receives one self-only `GC::EXCHANGE END` before `GC::ITEM_GROUND_DEL`, item refresh, point-change when gold is picked up, and `GC::ITEM_GET` frames; the paired peer receives one queued `GC::EXCHANGE END` before any queued visible-ground delete frame. The pickup mutation itself follows the separately owned `item-drop-pickup-bootstrap.md` persistence and temporary-ground-handle rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket `ITEM_PICKUP` returns template-backed pickup rejection feedback or the inventory-full fallback while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before that self-only info-chat frame is delivered: the requester receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, the requester then receives the authored or fallback `CHAT_TYPE_INFO` rejection, and carried inventory, quickslots, points, gold, ground handles, exchange item/gold displays, and persisted account snapshots remain unchanged.
- If same-socket merchant `SHOP BUY`, `SHOP SELL`, or `SHOP SELL2` succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the merchant transaction refresh burst is delivered: the requester receives one self-only `GC::EXCHANGE END` before the item refresh / gold point-change frames, and the paired peer receives one queued `GC::EXCHANGE END`. The merchant transaction itself follows the separately owned `npc-shop-transaction-bootstrap.md` inventory, quickslot, gold, and persistence rules; no exchange item/gold transfer or finalization result is produced.
- If same-socket `REFINE` returns a template-authored non-refineable rejection chat or a template-authored `REFINE_INFORMATION_NEW` preview while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the refine feedback frame is delivered: the requester receives one self-only `GC::EXCHANGE END` before the self refine chat/preview frame, and the paired peer receives one queued `GC::EXCHANGE END`. The refine path remains mutation-free and follows `item-refine-bootstrap.md`; no exchange item/gold transfer or finalization result is produced.
- If same-socket `ITEM_GIVE` returns a template-authored `anti_give` / `give_reject_message` self-only rejection while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the item-give feedback frame is delivered: the requester receives one self-only `GC::EXCHANGE END` before the self rejection chat, and the paired peer receives one queued `GC::EXCHANGE END`. The item-give path remains mutation-free and follows `item-give-bootstrap.md`; no exchange item/gold transfer or finalization result is produced.
- If same-socket `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, or `QUICKSLOT_SWAP` succeeds while the requester is paired in this bootstrap shell, the runtime closes the exchange shell before the self-only quickslot refresh frame(s) are delivered: the requester receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, and the accepted quickslot edit then follows `quickslot-bootstrap.md` persistence rules. No exchange item/gold transfer or finalization result is produced.

The shell is deliberately not an item/gold transfer system. It performs no inventory, equipment, quickslot, gold, ground-item, or persisted-account mutation. It now emits display-only exchange `ITEM_ADD` responses for valid carried items in the active shell, display-only `ITEM_DEL` responses for occupied display slots in that shell, display-only `GOLD_ADD` responses for non-zero active-shell gold placement requests that do not exceed the requester's live gold, display-only `ACCEPT` responses for active-shell accept requests whose requester-side remembered displayed items and gold still match live state, whose already-accepted partner-side remembered display state still matches that partner's current shared-world character state, and whose currently frozen receiver-side finalization preconditions pass, and one self-only `LESS_GOLD` response when an accept attempt finds that the requester's own previously displayed gold amount no longer fits the requester's current live gold.

The display-only `ITEM_ADD` path is accepted only when the requester is already paired in the bootstrap exchange shell and the requested display slot is in the current owned `0..11` range. The source must be a carried inventory cell, resolve through the loaded item-template snapshot, and contain exactly one unlocked, unequipped, well-formed item whose `vnum` and count fit the resolved template. The resolved template must also be usable by the selected character and must not author the current transfer guards (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`). On success, the requester receives one direct `GC::EXCHANGE ITEM_ADD` with `is_me = 1`, and the paired peer receives one queued `GC::EXCHANGE ITEM_ADD` with `is_me = 0`. Both packets carry `arg1 = item.vnum`, `arg2 = RESERVED_WINDOW + display_slot`, `arg3 = item.count`, and the template-authored `sockets` / `attributes` display arrays. The selected carried item is not removed or locked, no quickslot changes are made, no gold changes, and no account snapshot is persisted.

Each side owns its own display-slot namespace in this bootstrap shell. Reusing the same display slot for a second `ITEM_ADD` from the same side fails closed with no frames and no mutation. Reusing the same carried item instance in a second display slot from the same side also fails closed with no frames and no mutation, so the current display-only shell cannot show one live item identity twice at once. `ITEM_DEL` succeeds only when the requester is currently paired and `arg1` identifies one occupied display slot in the requester's own namespace. A successful `ITEM_DEL` returns one direct `GC::EXCHANGE ITEM_DEL` with `is_me = 1`, queues one peer `GC::EXCHANGE ITEM_DEL` with `is_me = 0`, carries the cleared display slot in `arg1`, and frees that in-memory display slot plus its carried item identity so a later display-only `ITEM_ADD` can reuse either. If either side had previously displayed `ACCEPT`, the accepted-marker reset packets described below are emitted before the `ITEM_DEL` clear frame for each receiver. Cancelling or closing the exchange clears the in-memory display-slot occupancy together with the paired shell.

The display-only `GOLD_ADD` path is accepted only when the requester is currently paired in the bootstrap exchange shell and `arg1` is non-zero. When `arg1` is less than or equal to the requester's current live gold, the requester receives one direct `GC::EXCHANGE GOLD_ADD` with `is_me = 1` and the paired peer receives one queued `GC::EXCHANGE GOLD_ADD` with `is_me = 0`; both packets carry the displayed gold amount in `arg1`, and the runtime remembers that displayed amount for later accept-time revalidation. If either side had previously displayed `ACCEPT`, the gold display response first includes the reset packets described below. When `arg1` is greater than the requester's current live gold, the requester receives one direct `GC::EXCHANGE LESS_GOLD` and the peer receives no frame. Both outcomes leave live gold and persisted account snapshots unchanged.

The display-only `ACCEPT` path is accepted only when the requester is currently paired in the bootstrap exchange shell and the requester's remembered display state still matches live state. The current selected-character identity must still match the shared-world exchange owner by character id, `VID`, and normalized name. For displayed carried items, each remembered display slot must still resolve through the current loaded item-template snapshot and exactly one live carried inventory item with the same item instance id, `vnum`, count, and carried slot; the item must remain unlocked, unequipped, well-formed, must not exceed the currently loaded template `max_count`, must still pass selected-character and transfer-guard template restrictions (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`), and the live carried inventory snapshot must not contain duplicate carried-slot occupancy or duplicate item instance ids. If this requester item revalidation fails, `ACCEPT` fails closed silently: no self accept marker, no peer accept marker, no finalization/result frame, no mutation, and the shell remains cancellable. If the requester previously displayed gold, the displayed amount must also still be less than or equal to the requester's current live gold. When the paired partner had already accepted, the same accept attempt also revalidates the partner's remembered displayed carried items and displayed gold against the partner's current shared-world character snapshot before emitting any new accept marker. A stale accepted-partner displayed carried item or stale accepted-partner displayed gold rejects this second accept silently: no self accept marker, no peer accept marker, no `LESS_GOLD`, no finalization/result frame, no mutation, and the shell remains cancellable. Before a second accept displays a new marker, the runtime also evaluates currently frozen receiver-side finalization preconditions: incoming displayed items must fit each receiver's carried inventory under loaded templates (merging into compatible unlocked stacks first and otherwise requiring fresh carried slots), and incoming displayed gold must fit the receiver's current gold point-change carrier. Receiver inventory-capacity or gold-overflow failures reject the second accept silently with no self/peer accept marker, no result frame, no mutation, and the shell remains cancellable. When all revalidation passes, the requester receives one direct `GC::EXCHANGE ACCEPT` with `is_me = 1`, and the paired peer receives one queued `GC::EXCHANGE ACCEPT` with `is_me = 0`; both packets carry `arg1 = 1` and leave the remaining fields zero. When the requester's remembered displayed gold is now greater than current live gold, the requester instead receives one direct `GC::EXCHANGE LESS_GOLD`, no peer accept marker is queued, and the accepted marker is not changed. None of these branches closes the exchange, transfers items or gold, emits an exchange result, or persists trade state. The shell remains cancellable after both sides have displayed `ACCEPT`, after a stale requester-side accept rejection, after a stale accepted-partner accept rejection, or after a stale requester-gold accept rejection.

When an accepted shell receives a successful display-changing `ITEM_ADD`, `ITEM_DEL`, or in-budget `GOLD_ADD`, the runtime clears any previously displayed accept markers before the display-change frame. For the requester, a previously accepted requester side emits self `GC::EXCHANGE ACCEPT(is_me = 1, arg1 = 0)` and peer `GC::EXCHANGE ACCEPT(is_me = 0, arg1 = 0)`. A previously accepted partner side emits peer `GC::EXCHANGE ACCEPT(is_me = 1, arg1 = 0)` and self `GC::EXCHANGE ACCEPT(is_me = 0, arg1 = 0)`. The reset order is requester side first and then partner side when both were accepted, followed by the display-change frame for the receiver. These reset frames change only the in-memory/presentation accept markers; they still perform no item, gold, quickslot, ground-handle, or persisted-account mutation.

Unsupported or malformed `EXCHANGE` contexts still fail closed:

- no final result server frames are emitted; stale requester displayed items or displayed-item template metadata at accept time emit no frames, stale already-accepted partner displayed items, displayed-item template metadata, or gold also emit no frames on the second accept, receiver inventory-capacity or gold-overflow precondition failures emit no frames on the second accept, and the owned stale requester displayed-gold accept exception emits only the same self-only `LESS_GOLD` status frame already used by over-budget `GOLD_ADD`
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing final result frames are queued
- no selected-character account snapshot is persisted

Once a selected character is already at the bootstrap zero-HP floor, new `START`, active-shell `ITEM_ADD`, `ITEM_DEL`, `GOLD_ADD`, `ACCEPT`, and `CANCEL` attempts fail closed with no exchange frames. If the zero-HP floor is reached while a shell is already open, the death-edge close above is the only owned post-floor exchange frame; later stale exchange requests then fail closed.

There is one owned guard-feedback exception for `ITEM_ADD`. When all of these are true:

- the selected character is already in `GAME` and above the bootstrap zero-HP floor
- the requester owns a live shared-world session that is already paired in the bootstrap exchange shell
- the exchange subheader is `ITEM_ADD`
- the source position is a carried inventory cell (`window = INVENTORY`, `cell < 90`)
- the requested exchange item display slot is in the current owned `0..11` range
- the carried item resolves through the loaded item-template snapshot
- the template `vnum` matches the carried item and validates normally
- the live carried item is unlocked, well-formed, unique in that carried cell, and its live count does not exceed `template.max_count`
- the template authors `anti_give = true`
- the template authors non-empty `give_reject_message`

then the minimal runtime keeps the existing guard response instead of emitting `GC::EXCHANGE ITEM_ADD`, and returns one self-only `CHAT_TYPE_INFO` frame:

- `vid = 0`
- `message = template.give_reject_message`

That response is deliberately not an exchange-window item placement or transfer attempt and is available only inside an active paired shell. No-shell `ITEM_ADD` attempts remain ordinary no-frame/no-mutation rejections even when the carried template authors `anti_give` plus `give_reject_message`. The guard response still performs no inventory, equipment, quickslot, gold, ground-handle, peer, or persistence mutation.

Templates that author `give_reject_message` without `anti_give` are invalid at the item-template store boundary, and embedded NUL bytes in the message fail closed before runtime boot.

Malformed `EXCHANGE` payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- richer trade target/range eligibility beyond the current visible-player check
- exchange-window item/gold/accept updates beyond the current `START` / display-only `ITEM_ADD` / display-only `ITEM_DEL` / display-only `GOLD_ADD` / display-only `ACCEPT` / accept reset / accept-time displayed-item and displayed-gold revalidation / `LESS_GOLD` / `END` shell
- trade item removal, carried-item locking/removal, or accepted gold mutation semantics beyond the current display-only item/gold shell
- real finalize/result state machines beyond the current accept-marker presentation/reset shell
- two-party inventory/gold mutation ordering
- accepted anti-flag/template guard behavior inside an active exchange beyond the active-shell self-only `anti_give` / `give_reject_message` `ITEM_ADD` rejection and the silent display suppression guards described above
- rollback, audit, or durable economic policy for exchange finalization

## Current coverage

- `internal/proto/item` freezes `CG::EXCHANGE` encode/decode behavior and the first shared `GC::EXCHANGE` response codec, plus unexpected-header and invalid-payload rejection for both directions.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/player` freezes both the metadata-driven, no-mutation exchange item-add `anti_give` rejection lookup and the valid carried-item display descriptor that copies template-authored sockets/attributes without mutating live or persisted state.
- `internal/minimal` freezes the visible-peer `START` / `CANCEL` shell with paired `GC::EXCHANGE START` / `END` frames and no persisted inventory, quickslot, equipment, or gold mutation; display-only `ITEM_ADD` / `ITEM_DEL` / `GOLD_ADD` / `ACCEPT`; duplicate display-slot/source-item suppression; accept-marker reset before accepted display changes; accept-time live item/gold revalidation; template-backed accept-time displayed-item revalidation against the currently loaded template metadata; second-accept receiver inventory-capacity and gold-overflow precondition guards; `ALREADY` / `LESS_GOLD`; unsupported final-result no-frame guards; active-shell `anti_give` item-add feedback; and teardown-before-feedback/mutation coverage for owned lifecycle, item-use, item-move/equipment, drag-to-item, drop, merchant, refine, storage, item-give, and accepted quickslot-edit paths.
