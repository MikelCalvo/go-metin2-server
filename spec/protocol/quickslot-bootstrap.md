# Quickslot bootstrap packet codecs

This note freezes the first wire-codec and `GAME`-phase dispatch contract for quickslot packets, the first persisted character snapshot field needed to carry quickslot state from auth ticket to game session, and the first loading-time selected-character quickslot bootstrap burst. Accepted runtime quickslot edits and item-move quickslot synchronization are intentionally left for later slices.

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

`SyncQuickslot(QUICKSLOT_TYPE_ITEM, old_cell, new_cell)` updates item quickslots when carried inventory items move between carried inventory cells, and deletes matching item quickslots when `new_cell = 255`. This Go slice does **not** implement that runtime synchronization yet; it only owns the packet layouts and `GAME`-phase dispatch seam required by future runtime quickslot edit/synchronization slices.

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

Current runtime behavior: decoded and dispatched only in `GAME`; the default runtime handler is fail-closed and emits no frames until a later persistence/edit slice owns accepted mutation semantics.

### Client `QUICKSLOT_DEL` (`0x050A`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | quickslot bar index to clear |

Total frame length: `5` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the default runtime handler is fail-closed and emits no frames until a later persistence/edit slice owns accepted mutation semantics.

### Client `QUICKSLOT_SWAP` (`0x050B`)

Direction: client -> server.

Payload:

| Field | Type | Notes |
| --- | --- | --- |
| `pos` | `uint8` | first quickslot index |
| `pos_to` | `uint8` | second quickslot index |

Total frame length: `6` bytes.

Current runtime behavior: decoded and dispatched only in `GAME`; the default runtime handler is fail-closed and emits no frames until a later persistence/edit slice owns accepted mutation semantics.

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

This keeps quickslots self-only and snapshot-derived in the current runtime. It does not yet accept client-authored quickslot mutation or synchronize item quickslots after later item movement/destruction paths.

## Current scope

Implemented now:

- Go codecs for client `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Go codecs for server `QUICKSLOT_ADD`, `QUICKSLOT_DEL`, and `QUICKSLOT_SWAP`.
- Strict header and payload-size validation for those client and server packets.
- `GAME`-phase dispatch hooks for client quickslot edit packets, with default fail-closed behavior until runtime mutation is owned.
- file-backed account and login-ticket snapshot round trips for bootstrap quickslot arrays.
- loading-time selected-character `QUICKSLOT_ADD` bootstrap frames for persisted quickslot arrays, emitted after the selected-character presence/state burst and before trailing peer/static-actor visibility frames.

Not implemented yet:

- accepted runtime mutation for client-originated `CG::QUICKSLOT_ADD` / `DEL` / `SWAP`
- accepted runtime updates to persisted quickslot state
- automatic item quickslot synchronization after `ITEM_MOVE`, `ITEM_USE`, shop sell, safebox, exchange, item timeout, or destruction
- validation of quickslot ranges beyond codec payload size
