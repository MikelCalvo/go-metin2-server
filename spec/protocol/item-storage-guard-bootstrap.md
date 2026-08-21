# Item storage guard bootstrap

This note freezes the first clean-room storage-facing item packet boundary for the bootstrap item lane.

The goal is deliberately conservative:

- own the client packet layouts for the TMP4-era safebox and mall item-transfer requests;
- own the first server response codecs the client can receive for safebox/mall open, item refresh, and narrow status updates;
- route those packets through the `GAME` phase without treating them as unknown-header disconnect edges;
- keep the shipped runtime fail-closed for transfer/password/money/item-placement semantics until a later storage/safebox slice owns those behaviors, while freezing the local `/open_safebox` / `/close_safebox` presentation seam needed for exchange busy-window policy;

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

The TMP4 client send helper builds the two `TItemPos` fields with the inventory window and safebox-slot byte values, but the owned codec keeps the wire field generic. Accepted same-session in-memory move semantics are owned below and require both positions to name the safebox window.

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

`internal/game` decodes these four packet layouts only while the session is already in `GAME` and routes each one through an explicit storage-facing handler seam. The default handlers remain fail-closed with no output, so adding the seam does not imply accepted safebox or mall mutation semantics beyond the currently owned check-in/out/item-move paths below.

The shipped minimal runtime intentionally rejects ordinary storage requests without output except for the owned `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` / `SAFEBOX_ITEM_MOVE` mutations and `anti_safebox` feedback seams:

- no `GC::SAFEBOX_*` or `GC::MALL_*` storage response frames are emitted for unsupported storage-transfer packets (`MALL_CHECKOUT`), even though their first codecs are now owned;
- no carried inventory, equipment, quickslot, point, gold, ground-handle, exchange, merchant, or peer state is mutated by those unsupported transfer packets;
- no account snapshot is persisted by those unsupported transfer packets;
- the session remains in `GAME` and the socket stays usable.

## First bootstrap safebox-open presentation seam

A later storage/safebox slice still owns password load, durable item persistence, money, and mall. This contract freezes the smallest presentation seam needed so exchange busy-window policy can observe an open safebox, plus the first accepted same-session in-memory check-in, check-out, and item-move mutations:

- the local slash harness `/open_safebox [size]` may mark the selected character's same-socket bootstrap safebox presentation as open while the session is already in `GAME` and above the bootstrap zero-HP floor;
- optional `size` is a page count in the owned bootstrap range `1..3`; omitted size defaults to `1`; values outside that range (for example `/open_safebox 4`), non-uint8 size tokens, and extra arguments fail closed with no frames, no ordinary talking-chat fallthrough, and no open-state mutation;
- on success the runtime emits exactly one self-only `GC::SAFEBOX_SIZE` with that page count, remembers an in-memory same-socket `open safebox` presentation flag, and then re-emits any still-remembered same-session in-memory `GC::SAFEBOX_SET` rows for that open session;
- repeating `/open_safebox` while already open is idempotent presentation refresh: it re-emits `GC::SAFEBOX_SIZE` for the currently remembered size (or the newly requested in-range size), re-emits remembered in-memory `SAFEBOX_SET` rows, and leaves inventory/equipment/quickslot/gold/persistence unchanged;
- the local slash harness `/close_safebox` clears that same-socket open flag when present and emits no storage frames in this bootstrap seam; remembered in-memory safebox contents stay in the same session until logout / shared-world leave / process end, and a later reopen may re-emit them after `SAFEBOX_SIZE`;
- opening or closing this presentation seam never loads durable safebox items from password/DB, never mutates carried inventory/equipment/quickslots/points/gold/ground handles by itself, never persists safebox contents, and never opens mall/player-shop/cube state;
- once the open flag is set, exchange `START` treats that same-socket character as busy under the exchange busy-window policy frozen in `item-exchange-bootstrap.md`, using the same requester/partner info-chat strings already owned for open merchant windows;
- after `/close_safebox`, later exchange `START` attempts are no longer rejected by this safebox busy guard.

## Accepted in-memory `SAFEBOX_CHECKIN`

`CG::SAFEBOX_CHECKIN` is now accepted for one carried inventory item while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- the request must name the inventory window and a carried cell inside the owned carried-inventory range that resolves to exactly one unlocked, unequipped, well-formed live item;
- the loaded template for that `vnum` must be valid, must match the live item `vnum`, must bound the live stack count with `max_count`, and must **not** author `anti_safebox`;
- `safe_pos` must be empty and inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`);
- a carried cell currently displayed in an active exchange shell fails closed with no frames and leaves the shell open;
- on success the runtime removes the whole carried stack from inventory, syncs source item quickslots with the already-owned removal path (`GC::QUICKSLOT_DEL` when the cell is fully removed), stores the item in same-session in-memory safebox state, persists the inventory/quickslot account snapshot, and emits self-only `GC::ITEM_DEL` plus `GC::SAFEBOX_SET` for that safebox cell;
- if an active bootstrap exchange shell is open and the check-in would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the inventory/safebox refresh frames; if an active bootstrap merchant window is open and the check-in would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the inventory/safebox refresh frames;
- no gold/money/mall frames are introduced;
- reconnect / process restart / logout / shared-world leave discard those in-memory safebox contents until a later persistence slice owns durable safebox state.

## Accepted in-memory `SAFEBOX_CHECKOUT`

`CG::SAFEBOX_CHECKOUT` is now accepted for one remembered same-session in-memory safebox item while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- `safe_pos` must resolve to exactly one remembered same-session in-memory safebox item inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`);
- the loaded template for that safebox item `vnum` must be valid, must match the stored item `vnum`, and must bound the stored stack count with `max_count`;
- the destination must be a carried inventory cell (`window = INVENTORY`) inside the owned carried range that can accept the whole safebox stack: either an empty cell, or a compatible unlocked unequipped same-`vnum` stack that still fits `max_count`;
- occupied incompatible / over-max / locked / exchange-displayed destinations, missing / out-of-range / empty `safe_pos`, and closed presentation all fail closed with no frames;
- on success the runtime removes the item from same-session in-memory safebox state, places/merges it into the destination carried cell while preserving the stored item identity on fresh-cell placement, persists the inventory/quickslot account snapshot, and emits self-only `GC::SAFEBOX_DEL` plus inventory `GC::ITEM_SET` (empty destination) or `GC::ITEM_UPDATE` (compatible merge);
- if an active bootstrap exchange shell is open and the check-out would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the safebox/inventory refresh frames; if an active bootstrap merchant window is open and the check-out would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the safebox/inventory refresh frames;
- no gold/money/mall frames are introduced;
- reconnect / process restart / logout / shared-world leave discard remaining in-memory safebox contents until a later persistence slice owns durable safebox state.

## Accepted in-memory `SAFEBOX_ITEM_MOVE`

`CG::SAFEBOX_ITEM_MOVE` is now accepted for whole-stack relocate / compatible merge inside same-session in-memory safebox contents while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- source and destination must both name the safebox window (`WindowSafebox`) with distinct cells inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`);
- source must resolve to exactly one remembered same-session in-memory safebox item;
- the loaded template for that safebox item `vnum` must be valid, must match the stored item `vnum`, and must bound the stored stack count with `max_count`;
- requested `count` must be `0` or exactly the live source count (whole-stack only); partial splits that would create a new safebox item identity in an empty destination stay fail-closed with no frames;
- empty destination relocates the whole source stack while preserving item identity and emits self-only `GC::SAFEBOX_DEL` (source) plus `GC::SAFEBOX_SET` (destination);
- occupied destination must be a compatible unlocked unequipped same-`vnum` stack that still fits `max_count` after merging the whole source count; success clears the source cell and emits self-only `GC::SAFEBOX_DEL` (source) plus `GC::SAFEBOX_SET` (merged destination);
- occupied incompatible / over-max / locked destinations, missing / out-of-range / same-cell / non-safebox windows, and closed presentation all fail closed with no frames;
- if an active bootstrap exchange shell is open and the move would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the safebox refresh frames; if an active bootstrap merchant window is open and the move would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the safebox refresh frames;
- carried inventory, equipment, quickslots, gold, mall, and account persistence stay unchanged;
- reconnect / process restart / logout / shared-world leave discard remaining in-memory safebox contents until a later persistence slice owns durable safebox state.

The only current output for unsupported or rejected storage-transfer packets remains template-authored rejection feedback for safebox check-in:

- `CG::SAFEBOX_CHECKIN` must reference the inventory window and a carried cell inside the owned carried-inventory range;
- the selected character must own exactly one valid, unlocked, unequipped live item in that cell;
- the loaded template for that `vnum` must be valid, must match the live item `vnum`, must bound the live stack count with `max_count`, and must author both `anti_safebox` and non-empty `safebox_reject_message`;
- if those guards pass with no active merchant window or exchange shell, the runtime returns exactly one self-only `GC::CHAT` / `CHAT_TYPE_INFO` with `vid = 0` and the authored message;
- if the same socket has an active bootstrap merchant window, the runtime closes that local presentation shell first with self-only `GC::SHOP END`, then returns the self-only authored info-chat rejection;
- if the same socket has an active bootstrap exchange shell, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then returns the self-only authored info-chat rejection;
- if both bootstrap presentation shells are active on the same socket, the runtime emits the local `GC::SHOP END` first, then the exchange close frame(s), then the storage rejection chat;
- inventory, equipment, quickslots, points, gold, ground handles, storage state, exchange item/gold displays, and persisted snapshots remain unchanged.

If the template omits `safebox_reject_message`, if `anti_safebox` is absent, if metadata is missing/invalid/mismatched, if the live item is malformed, locked, duplicated, absent, or already over template `max_count`, if the safebox presentation is closed, or if the destination `safe_pos` is occupied/out of range, the request preserves the older no-frame/no-mutation fail-closed behavior (except the accepted open-presentation mutation path above).

That same no-frame/no-mutation rule now also covers the retaliation-owned player-death floor. Once a content practice mob has driven the selected owner's live bootstrap HP to `0`, later `SAFEBOX_CHECKIN`, `SAFEBOX_CHECKOUT`, `SAFEBOX_ITEM_MOVE`, and `MALL_CHECKOUT` requests fail closed before any storage response frame, before any template-authored `anti_safebox` info-chat feedback, and before carried inventory/equipment, quickslot, point, gold, ground-handle, or account-persistence side effects can run. This does not broaden storage itself; it only keeps the existing unsupported storage guard from becoming a post-death escape hatch.

Malformed payload sizes fail at the codec/dispatcher boundary rather than reaching runtime mutation code.

## Deferred behavior

Later slices must write a new contract before broadening storage behavior. In particular, this slice does not freeze:

- safebox password / DB load flow beyond the local slash open-presentation harness above;
- safebox money state or `SAFEBOX_MONEY_CHANGE` runtime emission;
- runtime emission policy for `SAFEBOX_WRONG_PASSWORD`, `SAFEBOX_MONEY_CHANGE`, `MALL_OPEN`, `MALL_SET`, or `MALL_DEL`;
- accepted item-move mutation ordering beyond the currently owned whole-stack same-session relocate / compatible-merge path;
- mall open/checkout behavior;
- durable storage item persistence or DB schema;
- interaction/NPC surfaces that open storage windows;
- the legacy `CloseSafebox` command-chat companion beyond the local `/close_safebox` open-flag clear above;
- accepted template-authority policy for `anti_save`, mall-only items, or cash-shop metadata beyond the currently owned `ITEM_SET.anti_flags` projection and self-only `safebox_reject_message` feedback;
- partial-count safebox splits that allocate a new item identity into an empty destination cell.

## Current coverage

- `internal/proto/item` freezes encode/decode behavior, exact wire bytes, unexpected-header rejection, and invalid-payload rejection for the four client storage request packets and the first eight server safebox/mall response packets.
- `internal/game` freezes `GAME`-phase decode dispatch and optional handler-frame paths for all four storage-facing packets while preserving no-frame fail-closed defaults.
- `internal/player` freezes template-backed `SafeboxCheckinRejectText`, whole-stack `SafeboxCheckinItem` live inventory removal for accepted check-in, and whole-stack `SafeboxCheckoutItem` empty-cell placement / compatible merge for accepted check-out.
- `internal/minimal` freezes both ordinary no-frame/no-mutation/no-persistence storage guards and the authored `anti_safebox` / `safebox_reject_message` info-chat feedback path through the normal session harness, including active merchant-window and active-exchange-shell teardown before that feedback is delivered. It also freezes the player-death-floor variant where those same storage-facing requests stay silent and non-mutating after practice-mob retaliation has already driven the selected owner to `0` HP, including the `SAFEBOX_CHECKIN` case that would otherwise be allowed to emit authored `anti_safebox` feedback while alive. The bootstrap `/open_safebox` / `/close_safebox` presentation seam is owned here as well: `/open_safebox [1..3]` emits self-only `GC::SAFEBOX_SIZE` (plus remembered same-session `SAFEBOX_SET` rows), remembers the same-socket open flag (with omitted size defaulting to `1` on first open and reusing the remembered size on later idempotent refresh), `/close_safebox` clears that flag with no frames while keeping same-session in-memory contents, and exchange `START` requester/partner busy rejects observe that open flag with the already-owned merchant busy-window chat strings. Accepted open-presentation `SAFEBOX_CHECKIN` freezes inventory removal + quickslot sync + same-session in-memory `SAFEBOX_SET`, including reopen re-emission and exchange-shell close-on-success. Accepted open-presentation `SAFEBOX_CHECKOUT` freezes same-session safebox removal + carried empty-cell `ITEM_SET` / compatible `ITEM_UPDATE`, inventory persistence, reopen without the checked-out row, and exchange-shell close-on-success. Accepted open-presentation `SAFEBOX_ITEM_MOVE` freezes whole-stack same-session relocate into an empty safebox cell and compatible same-`vnum` merge under template `max_count`, emitting self-only `SAFEBOX_DEL` + `SAFEBOX_SET` without inventory/quickslot/gold/account mutation, including closed/out-of-range/incompatible fail-closed coverage and exchange-shell close-on-success. Accepted check-in/out/item-move success also freezes merchant-window auto-close with self-only `GC::SHOP END` before refresh frames (SHOP before exchange when both shells are active).
