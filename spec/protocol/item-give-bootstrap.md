# Item give bootstrap

This note freezes the first clean-room `ITEM_GIVE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout and `GAME` dispatch seam
- keep the owned template-authored `anti_give` self-only reject path
- accept one player-to-player whole-stack or partial-stack transfer onto a currently visible live peer
- fail closed with the already-owned requester busy-window info-chat when the giver is paired in a bootstrap exchange shell, including when the source stack is currently displayed in that shell
- leave NPC-target give, transferring a currently displayed exchange item, a second exchange finalize path, and two-party rollback/audit policy deferred

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

The layout is frozen from the TMP4-compatible client packet struct shape in project-owned terms. The repository owns the byte layout, the `anti_give` guard, and the first whole-stack plus partial-stack player-to-player transfer.

## Current runtime contract

`internal/game` decodes `ITEM_GIVE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

### Owned `anti_give` guard

There is one owned guard-feedback exception. When all of these are true:

- the selected character is already in `GAME` and above the bootstrap zero-HP floor
- `target_vid` identifies a currently connected visible player target from the selected character's live shared-world visibility set
- the source position is a carried inventory cell (`window = INVENTORY`, `cell < 90`)
- the carried item resolves through the loaded item-template snapshot
- the template `vnum` matches the carried item and validates normally
- the live carried item is unlocked, well-formed, unique in that carried cell, and its live count does not exceed `template.max_count`
- the requested `count` is non-zero and does not exceed the live carried stack count
- the template authors `anti_give = true`
- the template authors non-empty `give_reject_message`

then the minimal runtime accepts only the guard response and returns one self-only `CHAT_TYPE_INFO` frame:

- `vid = 0`
- `message = template.give_reject_message`

If the same socket has an active bootstrap merchant window, the server first returns self-only `GC::SHOP END`, clears the active merchant context, and then returns the self-only `ITEM_GIVE` rejection chat.

If the requester is paired in the current bootstrap exchange shell, the server first returns self `GC::EXCHANGE END` and queues peer `GC::EXCHANGE END`, clears the in-memory exchange display/accept state, and then returns the self-only `ITEM_GIVE` rejection chat. When both merchant and exchange shells are active, the merchant close is ordered before the exchange close, and both precede the item-give feedback frame.

That response is deliberately not a transfer attempt. Apart from the optional active-merchant / active-exchange closes above, it still performs no inventory, equipment, quickslot, ground-handle, peer-transfer, merchant-buy/sell, or persistence mutation, and the named visible target receives no queued item-transfer or item-give rejection frames.

Templates that author `give_reject_message` without one owned exchange-display / give rejection guard (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, job/sex/empire anti flags, or `min_level`) are invalid at the item-template store boundary, and embedded NUL bytes in the message fail closed before runtime boot. The `ITEM_GIVE` runtime feedback path still requires `anti_give` specifically; the broader guard set only authorizes store validation and the separately owned active-shell `EXCHANGE ITEM_ADD` feedback path.

Zero-target, unknown/invisible-target, zero-count, or oversized-count give attempts remain ordinary no-frame/no-mutation rejections even when the item template authors `anti_give` plus `give_reject_message`.

Once the selected owner has reached the retaliation-owned bootstrap zero-HP floor frozen in `player-death-bootstrap.md`, `ITEM_GIVE` fails closed before this `anti_give` feedback path. The dead-owner attempt emits no self chat, queues no peer frames, and still performs no inventory, equipment, quickslot, ground-handle, or persistence mutation.

### First owned player-to-player whole-stack and partial-stack transfer

When the `anti_give` guard does not apply, the shipped runtime accepts one player-to-player transfer when all of these are true:

- the selected giver is already in `GAME`, owns a live shared-world session, and is above the bootstrap zero-HP floor
- `target_vid` names a currently connected visible live player other than the giver, also above the bootstrap zero-HP floor
- the source position is a carried inventory cell (`window = INVENTORY`, `cell < 90`)
- the source cell holds one unlocked, unequipped, well-formed stack whose template resolves, validates, matches the live `vnum`, and is not transfer-guarded (`anti_get` / `anti_drop` / `anti_give` / `anti_sell` / `anti_stack`) or equipment-shaped
- both giver and recipient satisfy selected-character job/sex/empire/`min_level` use of that template
- the requested `count` is non-zero and does not exceed the live source stack count
- a count smaller than the live source stack is accepted only for a stackable template (matching counted `ITEM_DROP2` remainder)
- the live source count does not exceed `template.max_count`
- the giver is not currently paired in a bootstrap exchange shell (an open shell, including a currently displayed source cell, uses the busy-window companion below instead of transferring)
- the recipient has room to place that count through the already-owned ground-pickup placement helper (merge into a compatible carried stack, otherwise first free cell preferring the source cell)

On success the runtime:

- for a whole-stack count (`count` equals the live source count): removes the giver's whole source stack and syncs source item quickslots, then places the same instance identity onto the recipient
- for a partial-stack count (`count` smaller than the live source count): decrements the giver remainder in place (same instance identity, independent presence clone) and places a fresh-identity clone of that count onto the recipient, allocated above both giver and recipient live inventory/equipment identities so pickup cannot collide; source item quickslots stay on the still-occupied cell
- places the transferred instance through already-owned `PickupGroundItem` (no new `internal/player` API), cloning the live recipient character so presence-aware sockets/attributes on the rest of that bag and equipment stay intact (including explicit zero; omitted keeps template-fallback encode). The transferred clone also carries an independent presence copy so it cannot alias the giver remainder
- persists both selected-character account snapshots
- applies the recipient live snapshot and updates both shared-world characters
- returns self-only giver inventory refresh (`ITEM_DEL` for a whole-stack removal, or remainder `ITEM_UPDATE` for a partial count) plus any source `GC::QUICKSLOT_DEL` only when the source cell is fully removed
- queues recipient inventory refresh (`ITEM_SET` for a newly created cell, or `ITEM_UPDATE` when the stack merges) plus one `GC::ITEM_GET` notice (`arg = 0`) for the given count
- does not invent a second exchange finalize path; successful transfer still requires that the giver is not paired in the bootstrap exchange shell

### Owned exchange-window busy companion

When the `anti_give` guard does not apply, `target_vid` names a currently visible live player, the source is an otherwise valid carried inventory give, and the giver is currently paired in the bootstrap exchange shell — including when that source cell is currently displayed in the shell — the runtime accepts only the busy-window companion and returns one self-only `CHAT_TYPE_INFO` frame:

- `vid = 0`
- `message = You cannot trade while another trade window is open.` (the already-owned requester busy-window string)

That response is deliberately not a transfer attempt and not a second exchange finalize path. It leaves the exchange shell open and cancellable, queues no peer frames, and performs no inventory, equipment, quickslot, gold, ground-handle, exchange display/accept, or persistence mutation.

Authored `anti_give` / `give_reject_message` still closes the same-socket exchange shell before that self-only rejection chat as already owned above. Zero-target, unknown/invisible-target, zero-count, oversized-count, non-stackable partial counts, death-floor, and open-private-shop give attempts remain ordinary no-frame/no-mutation rejections even while an exchange shell is open. Recipient inventory capacity is not consulted on this companion because no transfer is attempted.

If the recipient cannot place the count, the requested count is zero or larger than the live stack, a partial count targets a non-stackable template, the target is not a visible live player, the giver is dead, the giver has an open private shop, or any persist/apply step fails, the request stays fail-closed: no frames, no giver or recipient inventory/quickslot mutation, and no persistence change. There is no owned two-party rollback/audit policy beyond restoring the giver live snapshot when a later step fails before both accounts are committed.

## Deferred behavior

Later slices must write a new contract before broadening this packet. In particular, this slice does not freeze:

- NPC-target give semantics
- transferring a currently displayed exchange item, or any other second exchange finalize/result path besides the already-owned mutual-accept finalize
- recipient-facing rejection text, give-success chat, or richer `ITEM_GET` party arguments
- durable two-party rollback/audit policy after both accounts have been committed

## Current coverage

- `internal/proto/item` freezes `ITEM_GIVE` encode/decode round trips plus unexpected-header and invalid-payload rejection.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes `give_reject_message` round-trip and fail-closed validation: it is valid with one owned exchange-display / give rejection guard (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, job/sex/empire anti flags, or `min_level`) and rejects embedded NUL bytes.
- `internal/player` freezes the metadata-driven, no-mutation `anti_give` rejection lookup, including the non-zero / not-over-stack requested-count guard.
- `internal/minimal` freezes the self-only `CHAT_TYPE_INFO` rejection frame when the request names a currently visible player target, the carried item's template authors `anti_give` and `give_reject_message`, and the requested count is valid for the live stack, active same-socket merchant-window and exchange-shell teardown before that authored rejection feedback, the no-frame/no-mutation guard for missing/invisible targets and the post-floor dead-owner guard that denies `ITEM_GIVE` before that feedback path can run, the first accepted whole-stack and partial-stack player-to-player transfers onto a visible live peer with dual persistence, giver `ITEM_DEL` / source quickslot clear for a whole stack or remainder `ITEM_UPDATE` without clearing still-occupied source item quickslots for a smaller count, queued recipient `ITEM_SET` or `ITEM_UPDATE` plus `ITEM_GET`, a fresh transferred identity on partial counts, and presence-aware sockets/attributes kept on the remainder, the transferred clone, and the rest of the recipient bag/equipment, plus the fail-closed self-only requester busy-window info-chat when the giver is paired in a bootstrap exchange shell (including a currently displayed source cell) that leaves the shell open without a second finalize path.
