# Quickslot bootstrap packet codecs

This note freezes only the first wire-codec contract for server-originated quickslot refresh packets. Runtime quickslot persistence, client-originated quickslot edits, and item-move quickslot synchronization are intentionally left for later slices.

## Evidence

The legacy oracle exposes `TQuickslot` as two one-byte fields:

- `type uint8`
- `pos uint8`

It also uses three server packets for quickslot refreshes:

- `GC::QUICKSLOT_ADD = 0x0519`
- `GC::QUICKSLOT_DEL = 0x051A`
- `GC::QUICKSLOT_SWAP = 0x051B`

`SyncQuickslot(QUICKSLOT_TYPE_ITEM, old_cell, new_cell)` updates item quickslots when carried inventory items move between carried inventory cells, and deletes matching item quickslots when `new_cell = 255`. This Go slice does **not** implement that runtime synchronization yet; it only owns the packet layouts required by a future synchronization slice.

## Packet layouts

All multi-byte frame fields use the normal repository frame envelope. These payloads are byte-only.

### `QUICKSLOT_ADD` (`0x0519`)

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

## Current scope

Implemented now:

- Go codecs for server `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Strict header and payload-size validation for those server packets.

Not implemented yet:

- client-originated `CG::QUICKSLOT_ADD` / `DEL` / `SWAP` handlers (`0x0509`, `0x050A`, `0x050B`)
- account snapshot persistence for quickslot state
- loading-time quickslot bootstrap frames
- automatic item quickslot synchronization after `ITEM_MOVE`, `ITEM_USE`, shop sell, safebox, exchange, item timeout, or destruction
- validation of quickslot ranges beyond codec payload size
