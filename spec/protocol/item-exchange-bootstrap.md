# Item exchange bootstrap

This note freezes the first clean-room `EXCHANGE` boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layout before broader player-to-player trade is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, equipment, quickslot, gold, ground-item, peer, or persistence mutation until a later exchange/trade slice owns two-party state and acceptance semantics

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
| 5 | `arg2` | `uint8` | item display slot for item-add requests, otherwise zero/ignored |
| 6 | `pos` | packed `TItemPos` | `window_type uint8`, `cell uint16 LE`; meaningful for item-add requests |

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

The repository owns only this byte layout and the current fail-closed runtime policy. It does not yet interpret item ownership, target eligibility, trade windows, accept state, or trade finalization.

## Current runtime contract

`internal/game` decodes `EXCHANGE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves exchange gameplay unsupported for now. Every `EXCHANGE` request currently fails closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

Malformed `EXCHANGE` payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- trade target/range eligibility
- exchange-window open/close server packets
- trade item placement, removal, or gold placement semantics
- accept/cancel state machines
- two-party inventory/gold mutation ordering
- anti-flag/template guard behavior inside an active exchange
- rollback, audit, or durable economic policy for exchange finalization

## Current coverage

- `internal/proto/item` freezes `CG::EXCHANGE` encode/decode behavior plus unexpected-header and invalid-payload rejection.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/minimal` freezes the shipped no-frame fail-closed behavior with persisted inventory, quickslots, and gold unchanged after an `EXCHANGE` item-add packet.
