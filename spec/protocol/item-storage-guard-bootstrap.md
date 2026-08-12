# Item storage guard bootstrap

This note freezes the first clean-room storage-facing item packet boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layouts for the TMP4-era safebox and mall item-transfer requests;
- route those packets through the `GAME` phase without treating them as unknown-header disconnect edges;
- keep the shipped runtime fail-closed until a later storage/safebox slice owns opening, password, money, item placement, and persistence semantics.

This is not a completed safebox, mall, warehouse, or account-storage system.

## Evidence

The TMP4-compatible client exposes these main game-socket client headers:

| Packet | Direction | Header | Client send shape |
| --- | --- | ---: | --- |
| `SAFEBOX_CHECKIN` | client -> server | `0x0820` | `safe_pos uint8` + packed `TItemPos` |
| `SAFEBOX_CHECKOUT` | client -> server | `0x0821` | `safe_pos uint8` + packed `TItemPos` |
| `SAFEBOX_ITEM_MOVE` | client -> server | `0x0822` | source packed `TItemPos` + destination packed `TItemPos` + `count uint8` |
| `MALL_CHECKOUT` | client -> server | `0x0840` | `mall_pos uint8` + packed `TItemPos` |

The same client source has server receive handlers for safebox/mall open and item refresh families, but this repository does not yet own an accepted runtime/storage model for those server packets.

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

## Current runtime contract

`internal/game` decodes these four packet layouts only while the session is already in `GAME` and routes each one through an explicit storage-facing handler seam. The default handlers remain fail-closed with no output, so adding the seam does not imply accepted safebox or mall mutation semantics.

The shipped minimal runtime intentionally rejects ordinary storage requests without output:

- no `GC::SAFEBOX_*` or `GC::MALL_*` frames are emitted;
- no carried inventory, equipment, quickslot, point, gold, ground-handle, exchange, merchant, or peer state is mutated;
- no account snapshot is persisted;
- the session remains in `GAME` and the socket stays usable.

The only current output is template-authored rejection feedback for safebox check-in:

- `CG::SAFEBOX_CHECKIN` must reference the inventory window and a carried cell inside the owned carried-inventory range;
- the selected character must own exactly one valid, unlocked, unequipped live item in that cell;
- the loaded template for that `vnum` must be valid, must match the live item `vnum`, must bound the live stack count with `max_count`, and must author both `anti_safebox` and non-empty `safebox_reject_message`;
- if those guards pass, the runtime returns exactly one self-only `GC::CHAT` / `CHAT_TYPE_INFO` with `vid = 0` and the authored message;
- inventory, equipment, quickslots, points, gold, peers, ground handles, storage state, and persisted snapshots remain unchanged.

If the template omits `safebox_reject_message`, if `anti_safebox` is absent, if metadata is missing/invalid/mismatched, or if the live item is malformed, locked, duplicated, absent, or already over template `max_count`, the request preserves the older no-frame/no-mutation fail-closed behavior.

Malformed payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening storage behavior. In particular, this slice does not freeze:

- safebox open/password flow;
- safebox size/money state;
- accepted checkin/checkout/item-move mutation ordering;
- mall open/checkout behavior;
- storage item persistence or DB schema;
- interaction/NPC surfaces that open storage windows;
- accepted template-authority policy for `anti_safebox`, `anti_save`, mall-only items, or cash-shop metadata beyond the currently owned `ITEM_SET.anti_flags` projection and self-only `safebox_reject_message` feedback.

## Current coverage

- `internal/proto/item` freezes encode/decode behavior, exact wire bytes, unexpected-header rejection, and invalid-payload rejection for the four client storage request packets.
- `internal/game` freezes `GAME`-phase decode dispatch and optional handler-frame paths for all four storage-facing packets while preserving no-frame fail-closed defaults.
- `internal/minimal` freezes both ordinary no-frame/no-mutation/no-persistence storage guards and the authored `anti_safebox` / `safebox_reject_message` info-chat feedback path through the normal session harness.
