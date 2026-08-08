# Item exchange bootstrap

This note freezes the first clean-room `EXCHANGE` boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layout before broader player-to-player trade is implemented
- own the shared server response packet layout before any runtime exchange-window emission is introduced
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, equipment, quickslot, gold, ground-item, peer, or persistence mutation until a later exchange/trade slice owns two-party state and acceptance semantics
- allow one template-authored guard response for already-owned `anti_give` metadata on `ITEM_ADD` without implementing two-party trade

This is not a completed exchange, trade, safebox, or player-shop system.

## Evidence

The TMP4-compatible client exposes `CG::EXCHANGE = 0x0508` on the main game socket. The client send helpers use one fixed client packet shape for exchange start, item add, item delete, gold add, accept, and cancel requests. The subheader selects the requested exchange action.

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
| `START` | `0` | decoded and fail-closed |
| `ITEM_ADD` | `1` | decoded and fail-closed |
| `ITEM_DEL` | `2` | decoded and fail-closed |
| `ELK_ADD` | `3` | decoded and fail-closed |
| `ACCEPT` | `4` | decoded and fail-closed |
| `CANCEL` | `5` | decoded and fail-closed |

The repository owns only this client byte layout and the current fail-closed runtime policy. It does not yet interpret item ownership, target eligibility, trade windows, accept state, or trade finalization.

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
| `START` | `0` | codec/documentation only |
| `ITEM_ADD` | `1` | codec/documentation only |
| `ITEM_DEL` | `2` | codec/documentation only |
| `GOLD_ADD` | `3` | codec/documentation only |
| `ACCEPT` | `4` | codec/documentation only |
| `END` | `5` | codec/documentation only |
| `ALREADY` | `6` | codec/documentation only |
| `LESS_GOLD` | `7` | codec/documentation only |

This codec slice is deliberately presentation-only. The shipped runtime still does not emit `GC::EXCHANGE`; the only current exchange guard feedback remains the self-only `CHAT_TYPE_INFO` packet described below.

## Current runtime contract

`internal/game` decodes `EXCHANGE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves exchange gameplay unsupported for now. Ordinary `EXCHANGE` requests still fail closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

There is one owned guard-feedback exception for `ITEM_ADD`. When all of these are true:

- the selected character is already in `GAME` and above the bootstrap zero-HP floor
- the exchange subheader is `ITEM_ADD`
- the source position is a carried inventory cell (`window = INVENTORY`, `cell < 90`)
- the requested exchange item display slot is in the current owned `0..11` range
- the carried item resolves through the loaded item-template snapshot
- the template `vnum` matches the carried item and validates normally
- the live carried item is unlocked, well-formed, unique in that carried cell, and its live count does not exceed `template.max_count`
- the template authors `anti_give = true`
- the template authors non-empty `give_reject_message`

then the minimal runtime accepts only the guard response and returns one self-only `CHAT_TYPE_INFO` frame:

- `vid = 0`
- `message = template.give_reject_message`

That response is deliberately not an exchange-window or transfer attempt. It still performs no inventory, equipment, quickslot, gold, ground-handle, peer, or persistence mutation.

Templates that author `give_reject_message` without `anti_give` are invalid at the item-template store boundary, and embedded NUL bytes in the message fail closed before runtime boot.

Malformed `EXCHANGE` payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- trade target/range eligibility
- exchange-window open/close server packets
- trade item placement, removal, or gold placement semantics
- accept/cancel state machines
- two-party inventory/gold mutation ordering
- accepted anti-flag/template guard behavior inside an active exchange beyond the self-only `anti_give` / `give_reject_message` `ITEM_ADD` rejection described above
- rollback, audit, or durable economic policy for exchange finalization

## Current coverage

- `internal/proto/item` freezes `CG::EXCHANGE` encode/decode behavior and the first shared `GC::EXCHANGE` response codec, plus unexpected-header and invalid-payload rejection for both directions.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/player` freezes the metadata-driven, no-mutation exchange item-add `anti_give` rejection lookup.
- `internal/minimal` freezes the shipped no-frame fail-closed behavior with persisted inventory, quickslots, and gold unchanged after an ordinary `EXCHANGE` item-add packet, plus the self-only `CHAT_TYPE_INFO` rejection frame when the carried item's template authors `anti_give` and `give_reject_message`.
