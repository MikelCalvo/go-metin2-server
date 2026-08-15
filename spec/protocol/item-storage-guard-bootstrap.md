# Item storage guard bootstrap

This note freezes the first clean-room storage-facing item packet boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layouts for the TMP4-era safebox and mall item-transfer requests;
- own the first server response codecs the client can receive for safebox/mall open, item refresh, and narrow status updates;
- route those packets through the `GAME` phase without treating them as unknown-header disconnect edges;
- keep the shipped runtime fail-closed until a later storage/safebox slice owns opening, password, money, item placement, and persistence semantics.

This is not a completed safebox, mall, warehouse, or account-storage system.

## Evidence

The TMP4-compatible client exposes these main game-socket storage headers:

| Packet | Direction | Header | Owned shape |
| --- | --- | ---: | --- |
| `SAFEBOX_CHECKIN` | client -> server | `0x0820` | `safe_pos uint8` + packed `TItemPos` |
| `SAFEBOX_CHECKOUT` | client -> server | `0x0821` | `safe_pos uint8` + packed `TItemPos` |
| `SAFEBOX_ITEM_MOVE` | client -> server | `0x0822` | source packed `TItemPos` + destination packed `TItemPos` + `count uint8` |
| `MALL_CHECKOUT` | client -> server | `0x0840` | `mall_pos uint8` + packed `TItemPos` |
| `SAFEBOX_SET` | server -> client | `0x0830` | owned `ITEM_SET` payload under the safebox header |
| `SAFEBOX_DEL` | server -> client | `0x0831` | owned `ITEM_DEL` payload under the safebox header |
| `SAFEBOX_WRONG_PASSWORD` | server -> client | `0x0832` | header-only status frame |
| `SAFEBOX_SIZE` | server -> client | `0x0833` | `size uint8` |
| `SAFEBOX_MONEY_CHANGE` | server -> client | `0x0834` | `money int32 LE` |
| `MALL_OPEN` | server -> client | `0x0841` | `size uint8` |
| `MALL_SET` | server -> client | `0x0842` | owned `ITEM_SET` payload under the mall header |
| `MALL_DEL` | server -> client | `0x0843` | owned `ITEM_DEL` payload under the mall header |

The server response codecs are codec-owned only in this slice. The shipped runtime still emits no safebox or mall response frames for unsupported storage attempts.

## Shared field: packed `TItemPos`

Current packet tests use the same packed item-position shape already owned by the item lane:

| Offset | Field | Type |
| --- | --- | --- |
| 0 | `window_type` | `uint8` |
| 1 | `cell` | `uint16 LE` |

For the current fail-closed storage guard, the runtime decodes the supplied position but does not interpret it as an authorized storage mutation. The one owned feedback exception is `SAFEBOX_CHECKIN` for a valid carried inventory item whose resolved item template authors both `anti_safebox` and non-empty `safebox_reject_message`: that rejected request returns self-only `CHAT_TYPE_INFO` with the authored text while still performing no storage mutation.

## Client packets

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::SAFEBOX_CHECKIN` (`0x0820`)

Payload size is 4 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `safe_pos` | `uint8` | target safebox slot requested by the client |
| 1 | `item_pos` | packed `TItemPos` | client-supplied source item position |

Total frame length is 8 bytes including the common frame envelope.

### `CG::SAFEBOX_CHECKOUT` (`0x0821`)

Payload size is 4 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `safe_pos` | `uint8` | source safebox slot requested by the client |
| 1 | `item_pos` | packed `TItemPos` | client-supplied destination item position |

Total frame length is 8 bytes including the common frame envelope.

### `CG::SAFEBOX_ITEM_MOVE` (`0x0822`)

Payload size is 7 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `source_pos` | packed `TItemPos` | client-supplied source slot |
| 3 | `destination_pos` | packed `TItemPos` | client-supplied destination slot |
| 6 | `count` | `uint8` | requested move count |

The TMP4 client send helper builds the two `TItemPos` fields with the inventory window and safebox-slot byte values, but the owned codec keeps the wire field generic. Accepted safebox semantics remain deferred.

Total frame length is 11 bytes including the common frame envelope.

### `CG::MALL_CHECKOUT` (`0x0840`)

Payload size is 4 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `mall_pos` | `uint8` | source mall slot requested by the client |
| 1 | `item_pos` | packed `TItemPos` | client-supplied destination item position |

Total frame length is 8 bytes including the common frame envelope.

## Server packets

These server packets are now owned at the codec boundary so later storage slices can add runtime behavior without rediscovering the wire layouts. Owning the codecs does not mean the minimal runtime emits these frames yet.

### `GC::SAFEBOX_SET` (`0x0830`) and `GC::MALL_SET` (`0x0842`)

Payload: the same fixed owned slot-refresh payload as `GC::ITEM_SET`:

| Field | Type | Notes |
| --- | --- | --- |
| `item_pos` | packed `TItemPos` | storage/mall slot namespace supplied by the caller |
| `vnum` | `uint32 LE` | item template/id shown in the slot |
| `count` | `uint8` | stack count |
| `flags` | `uint32 LE` | owned item flag projection |
| `anti_flags` | `uint32 LE` | owned anti-flag projection |
| `highlight` | `uint8` | current highlight byte |
| `sockets` | `3 * int32 LE` | display sockets |
| `attributes` | `7 * {type uint8, value int16 LE}` | display attributes |

Total frame length is 54 bytes including the common frame envelope.

### `GC::SAFEBOX_DEL` (`0x0831`) and `GC::MALL_DEL` (`0x0843`)

Payload: the same fixed slot-clear payload as `GC::ITEM_DEL`:

| Field | Type |
| --- | --- |
| `item_pos` | packed `TItemPos` |

Total frame length is 7 bytes including the common frame envelope.

### `GC::SAFEBOX_WRONG_PASSWORD` (`0x0832`)

Header-only status frame with no payload. Total frame length is 4 bytes.

### `GC::SAFEBOX_SIZE` (`0x0833`)

Payload size is 1 byte:

| Field | Type | Notes |
| --- | --- | --- |
| `size` | `uint8` | safebox size/page count value supplied by a future storage runtime |

Total frame length is 5 bytes including the common frame envelope.

### `GC::SAFEBOX_MONEY_CHANGE` (`0x0834`)

Payload size is 4 bytes:

| Field | Type | Notes |
| --- | --- | --- |
| `money` | `int32 LE` | signed storage money amount/update value |

Total frame length is 8 bytes including the common frame envelope.

### `GC::MALL_OPEN` (`0x0841`)

Payload size is 1 byte:

| Field | Type | Notes |
| --- | --- | --- |
| `size` | `uint8` | mall size/page count value supplied by a future storage runtime |

Total frame length is 5 bytes including the common frame envelope.

## Current runtime contract

`internal/game` decodes these four packet layouts only while the session is already in `GAME` and routes each one through an explicit storage-facing handler seam. The default handlers remain fail-closed with no output, so adding the seam does not imply accepted safebox or mall mutation semantics.

The shipped minimal runtime intentionally rejects ordinary storage requests without output:

- no `GC::SAFEBOX_*` or `GC::MALL_*` storage response frames are emitted, even though their first codecs are now owned;
- no carried inventory, equipment, quickslot, point, gold, ground-handle, exchange, merchant, or peer state is mutated;
- no account snapshot is persisted;
- the session remains in `GAME` and the socket stays usable.

The only current output is template-authored rejection feedback for safebox check-in:

- `CG::SAFEBOX_CHECKIN` must reference the inventory window and a carried cell inside the owned carried-inventory range;
- the selected character must own exactly one valid, unlocked, unequipped live item in that cell;
- the loaded template for that `vnum` must be valid, must match the live item `vnum`, must bound the live stack count with `max_count`, and must author both `anti_safebox` and non-empty `safebox_reject_message`;
- if those guards pass with no active exchange shell, the runtime returns exactly one self-only `GC::CHAT` / `CHAT_TYPE_INFO` with `vid = 0` and the authored message;
- if the same socket has an active bootstrap exchange shell, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then returns the self-only authored info-chat rejection;
- inventory, equipment, quickslots, points, gold, ground handles, storage state, exchange item/gold displays, and persisted snapshots remain unchanged.

If the template omits `safebox_reject_message`, if `anti_safebox` is absent, if metadata is missing/invalid/mismatched, or if the live item is malformed, locked, duplicated, absent, or already over template `max_count`, the request preserves the older no-frame/no-mutation fail-closed behavior.

Malformed payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening storage behavior. In particular, this slice does not freeze:

- safebox open/password flow;
- safebox size/money state;
- runtime emission policy for `SAFEBOX_SET`, `SAFEBOX_DEL`, `SAFEBOX_WRONG_PASSWORD`, `SAFEBOX_SIZE`, `SAFEBOX_MONEY_CHANGE`, `MALL_OPEN`, `MALL_SET`, or `MALL_DEL`;
- accepted checkin/checkout/item-move mutation ordering;
- mall open/checkout behavior;
- storage item persistence or DB schema;
- interaction/NPC surfaces that open storage windows;
- accepted template-authority policy for `anti_safebox`, `anti_save`, mall-only items, or cash-shop metadata beyond the currently owned `ITEM_SET.anti_flags` projection and self-only `safebox_reject_message` feedback.

## Current coverage

- `internal/proto/item` freezes encode/decode behavior, exact wire bytes, unexpected-header rejection, and invalid-payload rejection for the four client storage request packets and the first eight server safebox/mall response packets.
- `internal/game` freezes `GAME`-phase decode dispatch and optional handler-frame paths for all four storage-facing packets while preserving no-frame fail-closed defaults.
- `internal/minimal` freezes both ordinary no-frame/no-mutation/no-persistence storage guards and the authored `anti_safebox` / `safebox_reject_message` info-chat feedback path through the normal session harness, including active-exchange-shell teardown before that feedback is delivered.
