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

The TMP4 client send helper builds the two `TItemPos` fields with the inventory window and safebox-slot byte values, but the owned codec keeps the wire field generic. Accepted open-presentation move semantics are owned below: both positions may use either `WindowInventory` (TMP4 wire) or `WindowSafebox` (explicit tooling), and the cells are always interpreted as open-presentation safebox slot indices while the presentation is open.

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

This contract freezes the presentation seam needed so exchange busy-window policy can observe an open safebox, plus durable same-account rematerialize, the first warehouse password challenge, durable `/safebox_change_password`, and durable warehouse money via open-burst `SAFEBOX_MONEY_CHANGE` plus `/safebox_money_save` / `/safebox_money_withdraw`. Mall remains deferred:

- the local slash harness `/open_safebox [size]` remains the no-password lab/debug opener: while the session is already in `GAME` and above the bootstrap zero-HP floor it may mark the selected character's same-socket bootstrap safebox presentation as open without challenging a password and emits `GC::SAFEBOX_SIZE` (+ in-range `SAFEBOX_SET`) plus self-only `GC::SAFEBOX_MONEY_CHANGE`; lab `/open_safebox` never consults reopen cooldown or open-anchor distance so existing mutation proofs remain usable;
- an authored static-actor interaction with `interaction_kind = "open_safebox"` does **not** auto-open the presentation. Ordinary `INTERACT (0x0501)` against a visible in-range warehouse NPC remembers a same-socket pending password challenge with the authored effective size, records the selected character's current live `X`/`Y` as the same-socket open anchor (overwriting any prior anchor), and emits optional authored info chat plus self-only `CHAT_TYPE_COMMAND` `ShowMeSafeboxPassword`; pending challenge does **not** set the exchange busy flag and does not emit `SAFEBOX_SIZE` / `SAFEBOX_SET`; warehouse `INTERACT` itself does **not** reject on reopen cooldown or open-anchor distance (those gates run on `/safebox_password` only);
- when that warehouse `INTERACT` applies while a bootstrap merchant window is still open on the same socket, the runtime prepends one self-only `GC::SHOP END` and clears the merchant context before the optional authored warehouse info chat and `ShowMeSafeboxPassword` frames;
- optional authored `size` is a page count in the owned bootstrap range `1..3`; omitted / `0` size defaults to `1`; slash values outside that range (for example `/open_safebox 4`), non-uint8 size tokens, and extra arguments fail closed with no frames, no ordinary talking-chat fallthrough, and no open-state mutation; store/content validation likewise rejects authored `open_safebox` definitions whose `size` is outside `0..3`;
- optional authored `text` on an `open_safebox` definition emits one self-only `CHAT_TYPE_INFO` acknowledgement before `ShowMeSafeboxPassword` when the warehouse interaction applies;
- optional non-mutating quest gates on `open_safebox` reuse the ordinary service-gate mismatch text `Quest requirements are not met.` and leave both presentation and pending challenge closed;
- `/safebox_password <pwd>` while a pending challenge exists opens the presentation on match after the owned reopen-cooldown and open-anchor distance gates: clear pending, set the open/busy flag for the remembered size, hydrate durable cells from the same-account safebox FileStore, and emit `GC::SAFEBOX_SIZE` plus in-range `SAFEBOX_SET` rows plus one self-only `GC::SAFEBOX_MONEY_CHANGE` for the durable warehouse gold (default `0`). Durable optional `password` on the character row is matched when present; omitted / empty password resolves to bootstrap default `000000`. Password mismatch emits header-only `GC::SAFEBOX_WRONG_PASSWORD` and clears pending without opening. Empty / missing / over-6-char passwords emit self-only `CHAT_TYPE_INFO` `You have entered the wrong password.` and clear pending without emitting `SAFEBOX_WRONG_PASSWORD`. Already-open presentation emits self-only `CHAT_TYPE_INFO` `The warehouse is already open.`. While a same-socket reopen cooldown is armed (`now < closedAt + 10s` after any path that cleared an open presentation with `CloseSafebox`), `/safebox_password` emits self-only `CHAT_TYPE_INFO` `You cannot open the warehouse again so soon after closing it.`, leaves pending intact, and performs no open. When a remembered open anchor exists and `ApproxDistance(currentXY, openAnchorXY) > 1000` (reuse the owned exchange distance helper / bound), `/safebox_password` emits self-only `CHAT_TYPE_INFO` `You are too far from the warehouse to open it.`, leaves pending intact, and performs no open. Gate ordering is death-floor / no-selected → already-open chat → no-pending consume → cooldown reject chat → distance reject chat → malformed/wrong-password paths → success open. Password attempts with no pending challenge stay fail-closed-consume with no frames / no ordinary talking-chat fallthrough;
- `/safebox_change_password <old> <new>` while the selected character is in `GAME` and above the bootstrap zero-HP floor mutates only the durable password for that login + character id: missing / empty / over-6-char old or new passwords, and old-password mismatch against the durable effective password (blank/missing still means `000000`), emit self-only `CHAT_TYPE_INFO` `You have entered the wrong password.` with no durable mutation and no ordinary talking-chat fallthrough; matching old + valid new persist `password = <new>` (preserving durable cells), emit self-only `CHAT_TYPE_INFO` `The warehouse password has been changed.`, and leave open/pending presentation state unchanged. Change-password does **not** require an open safebox presentation and does **not** open one; a pending `ShowMeSafeboxPassword` challenge, if any, stays untouched. Persist failure stays fail-closed with the same wrong-password info chat and no partial password write. This bootstrap slice does not invent a `SAFEBOX_CHANGE_PASSWORD` client/server packet;
- repeating `/open_safebox` while already open is idempotent presentation refresh: it re-emits `GC::SAFEBOX_SIZE` for the currently remembered size (or the newly requested in-range size), re-hydrates from the durable store, re-emits in-range `SAFEBOX_SET` rows plus one self-only `GC::SAFEBOX_MONEY_CHANGE`, clears any stale pending challenge, and leaves inventory/equipment/quickslot/gold/account persistence unchanged;
- the local slash harness `/close_safebox` (and the TMP4-compatible client companion `/safebox_close`) clears that same-socket open flag when present and emits exactly one self-only `GC::CHAT` / `CHAT_TYPE_COMMAND` with message `CloseSafebox` so the client hides the safebox window; that close (and every other path that clears an open presentation with `CloseSafebox`, including practice-mob floor, transfer/warp rebootstrap, `/phase_select` / `/quit` / `/logout`, and the shared prepend/append close helpers) arms a same-socket 10-second reopen cooldown from session `now`; durable safebox contents stay on disk and the same-session presentation map stays until logout / shared-world leave / process teardown clears the presentation map only; a later reopen may re-hydrate and re-emit them after `SAFEBOX_SIZE`; already-closed close attempts stay fail-closed-consume with no frames and no ordinary talking-chat fallthrough;
- the same self-only `CloseSafebox` companion is also emitted when the open presentation is cleared by practice-mob retaliation reaching the bootstrap `0`-HP floor, by exact-position transfer / warp rebootstrap, or by same-socket `/phase_select` / `/quit` / `/logout` teardown: floor close appends it after any merchant `GC::SHOP END` and before exchange close frames; transfer / phase-lifecycle paths prepend it after merchant `SHOP END` and before exchange `END` when those shells are also closed by the same path; those teardown paths also clear any pending password challenge; durable safebox contents are not mutated by the companion itself, and Leave/teardown clears only the presentation map;
- opening or closing this presentation seam never mutates carried inventory/equipment/quickslots/points/gold/ground handles by itself and never opens mall/player-shop/cube state; open hydrates durable cells into the presentation table and emits durable warehouse money via `SAFEBOX_MONEY_CHANGE` only while the presentation is open (closed / pending-only paths emit no money frame);
- `/safebox_money_save <amount>` / `/safebox_money_withdraw <amount>` while the selected character is in `GAME`, above the bootstrap zero-HP floor, and the same-socket safebox presentation is already open deposit/withdraw durable warehouse gold against carried gold: missing / non-positive / non-uint32-parseable / over-`MaxInt32` amounts, insufficient carried/warehouse gold, warehouse or carried gold overflow past `MaxInt32` / exchange gold carrier max (`1<<31-1`), closed presentation, pending-only password challenge, death-floor, and unrecognized extra args stay fail-closed-consume with no frames / no ordinary talking-chat fallthrough; success deducts/credits carried gold, upserts durable money (preserving password + cells), persists account gold together with the safebox write, and emits self-only gold `PLAYER_POINT_CHANGE` plus `GC::SAFEBOX_MONEY_CHANGE` for the new warehouse total; write failure rolls back live gold and durable money fail-closed with no frames. This bootstrap slice does not invent a TMP4 CG `SAFEBOX_MONEY` request header;
- once the open flag is set, exchange `START` treats that same-socket character as busy under the exchange busy-window policy frozen in `item-exchange-bootstrap.md`, using the same requester/partner info-chat strings already owned for open merchant windows;
- after `/close_safebox`, later exchange `START` attempts are no longer rejected by this safebox busy guard.

## Accepted durable `SAFEBOX_CHECKIN`

`CG::SAFEBOX_CHECKIN` is now accepted for one carried inventory item while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- the request must name the inventory window and a carried cell inside the owned carried-inventory range that resolves to exactly one unlocked, unequipped, well-formed live item;
- the loaded template for that `vnum` must be valid, must match the live item `vnum`, must bound the live stack count with `max_count`, and must **not** author `anti_safebox`;
- `safe_pos` must be empty and inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`);
- a carried cell currently displayed in an active exchange shell fails closed with no frames and leaves the shell open;
- on success the runtime removes the whole carried stack from inventory, syncs source item quickslots with the already-owned removal path (`GC::QUICKSLOT_DEL` when the cell is fully removed), stores the item in the same-socket presentation table and the durable same-account safebox FileStore (login + character id), persists the inventory/quickslot account snapshot together with that safebox write, and emits self-only `GC::ITEM_DEL` plus `GC::SAFEBOX_SET` for that safebox cell; write failure rolls back live inventory and the presentation/durable safebox cells fail-closed;
- if an active bootstrap exchange shell is open and the check-in would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the inventory/safebox refresh frames; if an active bootstrap merchant window is open and the check-in would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the inventory/safebox refresh frames;
- no gold/money/mall frames are introduced;
- reconnect / process restart / logout / shared-world leave clear the presentation map and open flag, but durable cells rematerialize on the next successful open for the same login + character id.

## Accepted durable `SAFEBOX_CHECKOUT`

`CG::SAFEBOX_CHECKOUT` is now accepted for one remembered open-presentation safebox item while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- `safe_pos` must resolve to exactly one remembered open-presentation safebox item inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`);
- the loaded template for that safebox item `vnum` must be valid, must match the stored item `vnum`, and must bound the stored stack count with `max_count`;
- the destination must be a carried inventory cell (`window = INVENTORY`) inside the owned carried range that can accept the whole safebox stack: either an empty cell, or a compatible unlocked unequipped same-`vnum` stack that still fits `max_count`;
- occupied incompatible / over-max / locked / exchange-displayed destinations, missing / out-of-range / empty `safe_pos`, and closed presentation all fail closed with no frames;
- on success the runtime removes the item from the presentation table and durable safebox FileStore, places/merges it into the destination carried cell while preserving the stored item identity on fresh-cell placement, persists the inventory/quickslot account snapshot together with that safebox write, and emits self-only `GC::SAFEBOX_DEL` plus inventory `GC::ITEM_SET` (empty destination) or `GC::ITEM_UPDATE` (compatible merge); write failure rolls back live inventory and the presentation/durable safebox cells fail-closed;
- if an active bootstrap exchange shell is open and the check-out would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the safebox/inventory refresh frames; if an active bootstrap merchant window is open and the check-out would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the safebox/inventory refresh frames;
- no gold/money/mall frames are introduced;
- reconnect / process restart / logout / shared-world leave clear the presentation map and open flag, but remaining durable cells rematerialize on the next successful open for the same login + character id.

## Accepted durable `SAFEBOX_ITEM_MOVE`

`CG::SAFEBOX_ITEM_MOVE` is now accepted for whole-stack relocate / compatible merge and for partial-count split / compatible partial merge inside open-presentation safebox contents while the bootstrap `/open_safebox` presentation is already open:

- the selected character must be above the bootstrap zero-HP floor;
- source and destination must both use an accepted window type (`WindowInventory` or `WindowSafebox`) with distinct cells inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`); those cells are always interpreted as same-session safebox slot indices, never as carried-inventory cells;
- source must resolve to exactly one remembered open-presentation safebox item;
- the loaded template for that safebox item `vnum` must be valid, must match the stored item `vnum`, and must bound the stored stack count with `max_count`;
- requested `count` of `0` or exactly the live source count keeps the whole-stack path: empty destination relocates the whole source stack while preserving item identity and emits self-only `GC::SAFEBOX_DEL` (source) plus `GC::SAFEBOX_SET` (destination); occupied compatible unlocked unequipped same-`vnum` destination that still fits `max_count` after merging the whole source count clears the source cell and emits self-only `GC::SAFEBOX_DEL` (source) plus `GC::SAFEBOX_SET` (merged destination);
- requested `count` in `1..source_count-1` is the partial path: empty destination decrements the source count in place, allocates a new item identity for the destination stack (maximum ID across the selected character's live inventory + equipment + remembered open-presentation safebox cells, then `+ 1`), places the split count there, and emits self-only `GC::SAFEBOX_SET` (source remainder) plus `GC::SAFEBOX_SET` (destination split); occupied compatible unlocked unequipped same-`vnum` destination that still fits `max_count` after adding the requested count decrements the source and emits self-only `GC::SAFEBOX_SET` (source remainder) plus `GC::SAFEBOX_SET` (merged destination);
- occupied incompatible / over-max / locked destinations, zero/oversize counts other than whole-stack `0`, identity-allocation overflow (`max ID == ^uint64(0)`), missing / out-of-range / same-cell / unsupported windows (`WindowMall`, `WindowEquipment`, reserved, and any window other than `WindowInventory` / `WindowSafebox`), and closed presentation all fail closed with no frames;
- if an active bootstrap exchange shell is open and the move would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the safebox refresh frames; if an active bootstrap merchant window is open and the move would otherwise succeed, the runtime closes that local presentation shell first with self-only `GC::SHOP END` (before any exchange close frames when both shells are active), then emits the safebox refresh frames;
- carried inventory, equipment, quickslots, gold, mall, and account inventory persistence stay unchanged; the durable safebox FileStore is updated for the mutated cells, and persist failure restores the previous presentation map fail-closed with no frames;
- reconnect / process restart / logout / shared-world leave clear the presentation map and open flag, but remaining durable cells rematerialize on the next successful open for the same login + character id.

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

- TMP4 CG `SAFEBOX_MONEY` request packet / header ownership;
- warehouse reopen cooldown / distance-from-warehouse gates beyond the currently owned `/safebox_password` 10-second post-`CloseSafebox` cooldown and `ApproxDistance > 1000` open-anchor distance gates (auto-close of an already-open warehouse when the player walks beyond the anchor stays deferred);
- runtime emission policy for `MALL_OPEN`, `MALL_SET`, or `MALL_DEL`;
- client packet `SAFEBOX_CHANGE_PASSWORD` / DB answer frames / mall password change;
- accepted item-move mutation ordering beyond the currently owned whole-stack relocate / compatible-merge and partial-count split / partial-merge open-presentation paths;
- mall open/checkout behavior;
- SQL import/backfill from quarantined safebox exports;
- accepted template-authority policy for `anti_save`, mall-only items, or cash-shop metadata beyond the currently owned `ITEM_SET.anti_flags` projection and self-only `safebox_reject_message` feedback.

## Current coverage

- `internal/proto/item` freezes encode/decode behavior, exact wire bytes, unexpected-header rejection, and invalid-payload rejection for the four client storage request packets and the first eight server safebox/mall response packets.
- `internal/game` freezes `GAME`-phase decode dispatch and optional handler-frame paths for all four storage-facing packets while preserving no-frame fail-closed defaults.
- `internal/player` freezes template-backed `SafeboxCheckinRejectText`, whole-stack `SafeboxCheckinItem` live inventory removal for accepted check-in, and whole-stack `SafeboxCheckoutItem` empty-cell placement / compatible merge for accepted check-out.
- `internal/safeboxstore` freezes dedicated durable same-account safebox FileStore round-trip (including optional durable `password`, defaulting blank/missing to `000000`, and optional durable `money` omitted when zero), malformed fail-closed validation, deterministic JSON, and backup/restore/crash-temp operator hooks keyed by account login + character id.
- `internal/minimal` freezes both ordinary no-frame/no-mutation/no-persistence storage guards and the authored `anti_safebox` / `safebox_reject_message` info-chat feedback path through the normal session harness, including active merchant-window and active-exchange-shell teardown before that feedback is delivered. It also freezes the player-death-floor variant where those same storage-facing requests stay silent and non-mutating after practice-mob retaliation has already driven the selected owner to `0` HP, including the `SAFEBOX_CHECKIN` case that would otherwise be allowed to emit authored `anti_safebox` feedback while alive. The bootstrap `/open_safebox` / `/close_safebox` presentation seam is owned here as well: `/open_safebox [1..3]` remains the no-password lab opener and emits self-only `GC::SAFEBOX_SIZE` (plus rematerialized in-range `SAFEBOX_SET` rows from the durable same-account safebox FileStore), remembers the same-socket open flag (with omitted size defaulting to `1` on first open and reusing the remembered size on later idempotent refresh), warehouse `open_safebox` `INTERACT` emits `ShowMeSafeboxPassword` and `/safebox_password` opens on match or emits `SAFEBOX_WRONG_PASSWORD` / malformed info-chat, `/safebox_change_password` persists a durable password change with success/wrong-password info chat without opening presentation, `/close_safebox` / `/safebox_close` clear that flag with self-only `CHAT_TYPE_COMMAND` `CloseSafebox` while keeping durable cells on disk and clearing only the presentation map on Leave/teardown, and exchange `START` requester/partner busy rejects observe that open flag with the already-owned merchant busy-window chat strings. Accepted open-presentation `SAFEBOX_CHECKIN` freezes inventory removal + quickslot sync + durable `SAFEBOX_SET`, including reconnect/restart reopen rematerialize, exchange-shell close-on-success for non-displayed sources, and fail-closed no-frame rejection when the source carried cell is currently exchange-displayed. Accepted open-presentation `SAFEBOX_CHECKOUT` freezes durable safebox removal + carried empty-cell `ITEM_SET` / compatible `ITEM_UPDATE`, inventory+safebox persistence, reopen without the checked-out row, exchange-shell close-on-success for non-displayed destinations, and fail-closed no-frame rejection when the destination carried cell is currently exchange-displayed. Accepted open-presentation `SAFEBOX_ITEM_MOVE` freezes whole-stack same-session relocate into an empty safebox cell and compatible same-`vnum` merge under template `max_count`, plus partial-count empty-destination split with new item-identity allocation and compatible partial merge, emitting self-only `SAFEBOX_DEL` + `SAFEBOX_SET` for whole-stack success or dual `SAFEBOX_SET` for partial success without inventory/quickslot/gold/account mutation, including the TMP4 inventory-window wire and explicit `WindowSafebox` tooling path for packed positions, closed/out-of-range/unsupported-window/incompatible fail-closed coverage, and exchange-shell close-on-success. Accepted check-in/out/item-move success also freezes merchant-window auto-close with self-only `GC::SHOP END` before refresh frames (SHOP before exchange when both shells are active).
