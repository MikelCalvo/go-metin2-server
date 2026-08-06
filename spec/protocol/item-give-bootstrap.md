# Item give bootstrap

This note freezes the first clean-room `ITEM_GIVE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader player-to-player item transfer is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, quickslot, ground-item, or persistence mutation until a later exchange/trade slice owns recipient semantics
- allow one template-authored guard response for already-owned `anti_give` metadata without implementing recipient transfer

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

The shipped minimal runtime intentionally leaves `ITEM_GIVE` unsupported for now. For ordinary attempts, every target, source cell, window, and count still fail closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

There is one owned guard-feedback exception. When all of these are true:

- the selected character is already in `GAME` and above the bootstrap zero-HP floor
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

That response is deliberately not a transfer attempt. It still performs no inventory, equipment, quickslot, ground-handle, peer, or persistence mutation.

Templates that author `give_reject_message` without `anti_give` are invalid at the item-template store boundary, and embedded NUL bytes in the message fail closed before runtime boot.

Zero-count or oversized-count give attempts remain ordinary no-frame/no-mutation rejections even when the item template authors `anti_give` plus `give_reject_message`. This keeps accidental client attempts fail-closed instead of falling into incomplete item-transfer behavior while allowing valid-count authored `anti_give` items to explain the rejection.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- target eligibility or range checks
- player-to-player give semantics
- NPC-target give semantics
- exchange/trade window choreography
- partial-stack transfer behavior
- recipient inventory placement and quickslot side effects
- item-give acceptance text or recipient-facing rejection text beyond the self-only `anti_give` guard message
- ownership, audit, or rollback policy for two-party mutations

## Current coverage

- `internal/proto/item` freezes `ITEM_GIVE` encode/decode round trips plus unexpected-header and invalid-payload rejection.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes `give_reject_message` round-trip and fail-closed validation: it is valid only with `anti_give` and rejects embedded NUL bytes.
- `internal/player` freezes the metadata-driven, no-mutation `anti_give` rejection lookup, including the non-zero / not-over-stack requested-count guard.
- `internal/minimal` freezes the shipped runtime fail-closed behavior with persisted inventory and quickslots unchanged after an `ITEM_GIVE` packet, plus the self-only `CHAT_TYPE_INFO` rejection frame when the carried item's template authors `anti_give` and `give_reject_message` and the requested count is valid for the live stack.
