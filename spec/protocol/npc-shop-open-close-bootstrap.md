# NPC Shop Open / Close Bootstrap

This document freezes the first client-visible merchant window contract for `go-metin2-server`.

It is intentionally narrow:
- keep the existing `INTERACT (0x0501)` ingress as the way a player targets a visible merchant actor
- define the smallest server-owned `GC::SHOP` open/close surface that can sit on top of that interaction result
- define the smallest client-owned `SHOP` session actions that follow from an already-open merchant window: `BUY` and `END`
- keep the runtime/state contract aligned with the structured merchant catalog and buy-only transaction gate already frozen elsewhere

It sits on top of:
- `npc-service-interactions-bootstrap.md`
- `npc-shop-preview-bootstrap.md`
- `npc-shop-catalog-bootstrap.md`
- `static-actor-interaction-request.md`

## Scope

This contract currently applies only to:
- a connected selected character already in `GAME`
- a visible bootstrap static actor whose interaction resolves to a valid structured merchant catalog
- the existing `INTERACT (0x0501)` request as the merchant-open trigger
- one active merchant session per live selected character session
- buy-only browsing and purchase flow on top of that active merchant session
- explicit merchant-window close through `SHOP END`

This contract does **not** yet apply to:
- sell-back
- `SELL2`
- personal shops / `MYSHOP`
- basket or multi-buy quantity UI
- safebox / mall / storage surfaces
- multi-tab cash-shop semantics
- quest-driven merchant dialogs or branching NPC windows

## First owned merchant packet family

Current compatibility references already indicate the merchant family headers:
- client -> server: `SHOP`, header `0x0801`
- server -> client: `SHOP`, header `0x0810`

For the first owned merchant-window contract, the repository now freezes only these subheader roles:

### Client -> server
- `BUY`
  - buy one authored merchant catalog entry while an active merchant session exists
- `END`
  - explicitly close the currently open merchant session without buying anything further

### Server -> client
- `START`
  - open the first owned merchant window for the interacting player
- `END`
  - close the first owned merchant window on the live game socket when the runtime can still deliver merchant-specific frames

The repository acknowledges other compatibility-oriented subheaders already seen in references:
- client-side: `SELL`, `SELL2`
- server-side: `START_EX`, `UPDATE_ITEM`, `UPDATE_PRICE`, `OK`, `NOT_ENOUGH_MONEY`, `SOLDOUT`, `INVENTORY_FULL`, `INVALID_POS`, `SOLD_OUT`, `NOT_ENOUGH_MONEY_EX`

However, those remain outside the first fully frozen open/close contract unless they are called out explicitly below.

## Selected exact wire shapes for the first codec slice

The first codec slice now freezes one narrow set of exact packet shapes for the merchant family.

The owned Go surface for that slice now lives in `internal/proto/shop` through:
- `EncodeClientBuy` / `DecodeClientBuy`
- `EncodeClientEnd` / `DecodeClientEnd`
- `EncodeServerStart` / `DecodeServerStart`
- `EncodeServerEnd` / `DecodeServerEnd`
- `EncodeServerOK` / `DecodeServerOK`
- `EncodeServerNotEnoughMoney` / `DecodeServerNotEnoughMoney`
- `EncodeServerInventoryFull` / `DecodeServerInventoryFull`

These shapes are intentionally small and buy-only:
- client `SHOP END`
- client `SHOP BUY`
- server `GC::SHOP START`
- server `GC::SHOP END`
- server `GC::SHOP OK`
- server `GC::SHOP NOT_ENOUGH_MONEY`
- server `GC::SHOP INVENTORY_FULL`

They are enough to own the current bootstrap merchant-window packet family without pretending that richer merchant-window choreography is already final.

### Common envelope

All currently frozen merchant frames use the repository's standard little-endian frame envelope:
- `header:uint16`
- `length:uint16`
- `payload...`

Within that payload, the first byte is always the merchant `subheader:uint8`.

### Client `SHOP END`

The first owned close request is the smallest possible client merchant frame:
- header: `0x0801`
- total length: `5`
- payload bytes:
  - `subheader = END`

No trailing bytes are currently owned after the client `END` subheader.

### Client `SHOP BUY`

The first owned buy request freezes this exact payload shape:
- header: `0x0801`
- total length: `7`
- payload bytes:
  - `subheader = BUY`
  - `raw_leading_byte:uint8`
  - `catalog_slot:uint8`

The current clean-room contract now distinguishes two facts clearly:
- the last byte is the zero-based authored `catalog_slot`
- the first buy-specific byte is preserved as an opaque raw byte in the owned codec surface for now

That means the repository now owns the exact byte layout of the first `BUY` frame without overstating the final gameplay meaning of that leading buy-specific byte.

### Server `GC::SHOP START`

The first owned merchant open response freezes this exact payload shape:
- header: `0x0810`
- payload bytes:
  - `subheader = START`
  - `owner_vid:uint32` (little-endian)
  - `items[40]`

Each `items[i]` entry is a fixed-width packed record:
- `vnum:uint32`
- `price:uint32`
- `count:uint8`
- `display_pos:uint8`
- `sockets[3]:int32`
- `attributes[7]`
  - each attribute = `type:uint8` + `value:int16`

This yields:
- per-item wire size = `43` bytes
- `START` item block size = `40 * 43 = 1720` bytes
- `START` payload size = `1 + 4 + 1720 = 1725` bytes
- total `GC::SHOP START` frame length = `1729`

For the first owned fixtures, the repository may deliberately keep many trailing entries zeroed.
That still counts as a fully exact wire shape because the item array size, field order, and packing are now frozen.

### Server `GC::SHOP END`

The first owned close response is the smallest possible server merchant frame:
- header: `0x0810`
- total length: `5`
- payload bytes:
  - `subheader = END`

No trailing bytes are currently owned after the server `END` subheader.

### Packet-path failure companions

The live merchant-window runtime now owns one narrow failure-ack seam too.

When a live packet `SHOP BUY` request fails for one of the already-owned authoritative causes below, the packet-path response now uses one bare merchant-family error frame:
- insufficient gold -> `GC::SHOP NOT_ENOUGH_MONEY`
- no valid carried placement -> `GC::SHOP INVENTORY_FULL`
- unknown authored `catalog_slot` inside the still-bound merchant snapshot -> `GC::SHOP INVALID_POS`

The bootstrap wire shape for those three error companions is intentionally tiny:
- header: `0x0810`
- total length: `5`
- payload bytes:
  - `subheader = NOT_ENOUGH_MONEY`, `INVENTORY_FULL`, or `INVALID_POS`

No trailing payload bytes are owned for those three error frames in the current slice.

This freeze is intentionally narrower than full merchant-window choreography:
- it applies only to packet `SHOP BUY` on a still-open merchant session
- it does not yet freeze `UPDATE_ITEM`, `UPDATE_PRICE`, `SOLDOUT`, or `START_EX`
- the local `/shop_buy <catalog_slot>` debug harness still continues to use the current placeholder info-chat failure surface for insufficient-gold / no-valid-placement and keeps silent unknown-slot failure until a later cleanup slice says otherwise

### Packet-path success companion

The live merchant-window runtime now owns the narrowest honest packet-path success companion without claiming broader update choreography yet.

When a live packet `SHOP BUY` request succeeds on a still-open merchant session, the packet-path response now:
- keeps the existing self-only authoritative carried-slot refreshes (`ITEM_SET` per changed carried slot in carried-slot order)
- stops using the older placeholder packet-path success chat as the terminal success signal for that packet flow
- appends one bare merchant-family `GC::SHOP OK` after those carried-slot refreshes

The bootstrap wire shape for that success companion is intentionally tiny:
- header: `0x0810`
- total length: `5`
- payload bytes:
  - `subheader = OK`

No trailing payload bytes are owned for `GC::SHOP OK` in the current slice.

This success freeze is still narrower than full merchant-window choreography:
- it applies only to successful packet `SHOP BUY` while an active merchant session still exists
- it does not yet freeze `UPDATE_ITEM`, `UPDATE_PRICE`, `SOLDOUT`, or `START_EX`
- the temporary local `/shop_buy <catalog_slot>` debug harness may keep the current placeholder success info chat until a later cleanup slice says otherwise

## Open rule

The project still does **not** freeze a new client-originated “open shop” request packet.

Instead, merchant open stays anchored to the already-owned NPC interaction ingress:
1. the player targets a visible merchant actor through `INTERACT (0x0501)`
2. runtime validation resolves that actor to a deterministic structured merchant catalog
3. the runtime binds that resolved merchant snapshot to the interacting session as the current merchant window context
4. the runtime then emits the first owned `GC::SHOP START` open response on the same live game socket

The open rule must fail closed when:
- the interaction target is not currently visible
- the interaction target is out of range
- the target has no merchant interaction metadata
- the referenced merchant definition cannot be resolved
- the resolved catalog is malformed or cannot be rendered against the current item-template store

The current structured `shop_preview` contract still matters here:
- authored merchant identity still comes from the same structured catalog model
- the open contract does not introduce a second merchant-definition source of truth
- the preview-style resolution path remains the authoritative way to decide which merchant the player is opening

## Close rule

The first owned merchant close contract is intentionally small.

An active merchant session may close in only these owned ways:
- the client sends `SHOP END`
- the live session leaves `GAME`
- the session disconnects or is closed
- the session transfers or otherwise loses the merchant interaction context
- the selected live owner reaches the current practice-mob retaliation floor at `0` HP while a merchant window is still open
- the bound merchant actor/catalog becomes invalid before a later `BUY`

When the socket is still live and still in a state where merchant-specific frames can be delivered, the runtime should treat `GC::SHOP END` as the close companion for the currently open merchant window.

The current bootstrap runtime now owns one explicit stale-window revalidation close too:
- if a still-open merchant window becomes stale because the live actor no longer resolves as an interactable merchant or the bound `shop_preview` snapshot no longer matches the current authored definition, the next packet `SHOP BUY` now answers with one self-only `GC::SHOP END`
- that revalidation-driven close clears the active merchant context immediately, so a later explicit client `SHOP END` or another packet `SHOP BUY` on the same stale window now fails closed until the player opens a fresh merchant window again

The current bootstrap runtime now owns one explicit transfer-triggered close too:
- if a successful warp interaction or exact-position transfer trigger relocates the still-live selected owner while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the self transfer rebootstrap burst on that same socket
- that transfer-triggered close clears the active merchant context immediately, so later `SHOP END` / `SHOP BUY` on the destination side fail closed until the player opens a fresh merchant window again

The current bootstrap runtime now owns one explicit same-socket select-phase close too:
- if that same still-live selected owner sends `/phase_select` while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the outgoing select-phase transition frame on that same socket
- that select-phase close clears the active merchant context immediately, so later merchant requests stay fail-closed until a future character is selected and opens a fresh merchant window again

The current bootstrap runtime now owns one explicit same-socket slash-command teardown pair too:
- if that same still-live selected owner sends `/quit` while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the existing self `CHAT_TYPE_COMMAND quit` delivery on that same socket
- if that same still-live selected owner sends `/logout` while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the outgoing close-phase transition frame on that same socket
- both slash-command closes clear the active merchant context immediately, so later merchant requests stay fail-closed until a future selected session opens a fresh merchant window again

The current bootstrap runtime now owns one explicit post-floor teardown case too:
- if an already-open merchant window belongs to the same selected live owner session whose immediate or delayed practice-mob retaliation beat just reached `0` HP, the owner still receives the ordinary retaliation floor transition first (`GC PLAYER_POINT_CHANGE`, `GC DEAD`, `GC TARGET(0, 0)`) and then one self-only `GC::SHOP END`
- that same floor transition also clears the active merchant context immediately, so a later client `SHOP END` request on the same dead owner session now fails closed until a future slice owns broader revive / reopen behavior

This document does **not** yet claim that every teardown path must always emit a visible merchant close frame before other phase or disconnect behavior takes over.

## Session rule

The first owned merchant window model is one-session-at-a-time and one-merchant-at-a-time:
- one selected character session may hold at most one active merchant window context
- opening a new merchant window replaces any prior merchant context for that same live session
- `BUY` and `END` are only valid while that active merchant window context exists
- the active merchant window context must be cleared on transfer, disconnect, logout, close, or any other loss of selected-session runtime ownership

This preserves the same fail-closed ownership style already used for transfer and interaction state.

## BUY path relationship

This document does not redefine the buy-state contract.
That remains owned by:
- `npc-shop-transaction-bootstrap.md`

What this document adds is the session choreography around that state mutation:
- `BUY` is now explicitly the client-side action that follows a successful merchant open
- `BUY` is invalid before `GC::SHOP START` opens a merchant window context
- `BUY` remains buy-only and catalog-slot-addressed
- the live bootstrap runtime now accepts real client `SHOP BUY` ingress directly, while the temporary `/shop_buy <catalog_slot>` harness remains only as a local debug seam that reuses the same state contract

The currently frozen addressing fact still applies unchanged:
- in client `SHOP BUY`, the second trailing byte after the common `SHOP` envelope selects the zero-based authored `catalog_slot`

## Success and failure refresh expectations

The first client-visible merchant contract is now honest about two separate layers:

### 1. Authoritative state layer
Successful `BUY` still means:
- gold is debited exactly once
- the requested item count is granted exactly once
- persistence/writeback succeeds before the new live state is committed

### 2. Merchant-window/UI layer
The merchant family is now expected to own the open/close session boundary, but not yet every success/failure byte sequence inside the window.

The repository can now say this much honestly:
- a valid merchant interaction now opens through `GC::SHOP START` on the live bootstrap runtime
- explicit merchant close now uses client `SHOP END` plus server `GC::SHOP END` while the session still holds an active merchant context in `GAME`
- if a still-open merchant window becomes stale because the live actor or authored `shop_preview` snapshot changed underneath it, the next packet `SHOP BUY` now auto-closes that stale window with one self-only `GC::SHOP END`
- if a successful warp interaction or exact-position transfer trigger relocates that same still-live selected owner while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the self transfer rebootstrap burst and clears the active merchant context immediately
- if that same still-live selected owner sends `/phase_select` while a merchant window is open, the runtime now prepends one self-only `GC::SHOP END` before the outgoing select-phase transition frame and clears the active merchant context immediately
- if that same selected live owner reaches the current practice-mob retaliation floor at `0` HP while a merchant window is open, the runtime now also tears that merchant window down with one self-only `GC::SHOP END` after the owned death + target-clear transition
- the owned `SHOP BUY` packet shape is now also the primary live bootstrap merchant-buy ingress, while `/shop_buy <catalog_slot>` remains only a local debug harness for the same state contract
- successful packet buys now also end on one bare merchant-family `GC::SHOP OK` after the already-owned self-only `ITEM_SET` refreshes for changed carried slots

The exact mandatory role of:
- `INVALID_POS`
- `UPDATE_ITEM`
- `UPDATE_PRICE`

is still capture-gated before the repository claims full merchant-window choreography ownership.

The current live runtime now narrows one packet-path success seam and two error seams:
- `OK` is the frozen merchant-family success companion for packet `SHOP BUY` after self-only `ITEM_SET` refreshes on successful authoritative mutation
- `NOT_ENOUGH_MONEY` is the frozen merchant-family failure companion for packet `SHOP BUY` when the selected character lacks enough gold
- `INVENTORY_FULL` is the frozen merchant-family failure companion for packet `SHOP BUY` when no valid carried placement exists
- broader merchant-window update choreography still remains unfrozen beyond those three bare packet companions

## Explicit remaining unknowns after the first runtime GREEN

The following remain intentionally unfrozen for the next merchant packet/runtime slices:
- whether later compatibility work will force `START_EX` instead of the currently owned `START` open path
- the final gameplay semantic meaning of the opaque leading buy-specific byte in client `SHOP BUY`
- the exact minimal success-side `GC::SHOP` sequence needed to keep the TMP4 merchant UI stable after a `BUY` once the two frozen bare packet-path error frames are no longer the only owned merchant-window buy companions
- whether teardown paths beyond explicit `SHOP END`, stale-window auto-close, transfer-triggered rebootstrap close, same-socket `/phase_select` close, and the current retaliation-floor close also need a visible `GC::SHOP END` before phase/disconnect behavior takes over
- whether any merchant-side refresh frames must accompany a successful `BUY` beyond the already-owned self-facing state refresh packets

These unknowns are the gate for the next merchant buy runtime slice.

## Explicit non-goals

This slice does **not** yet freeze:
- `SELL`
- `SELL2`
- `START_EX`
- multi-tab merchant indexing
- cash/coin shop semantics
- merchant stock depletion or refresh timers
- personal shop flows

## Success definition

After this slice, the repository should be able to say:
- the first owned merchant window family is no longer only implied in project docs
- merchant open still starts from the already-owned `INTERACT` ingress and structured merchant resolution path
- `GC::SHOP START` is now the live merchant open response on the bootstrap runtime
- client `SHOP END` plus server `GC::SHOP END` are now the live explicit close pair for an active bootstrap merchant session
- client `SHOP BUY` is now both an owned codec shape and the live bootstrap merchant-buy ingress, while `/shop_buy <catalog_slot>` remains only a local debug harness for QA/recovery
- the project still does not pretend that the final wire payloads or the full success/failure response choreography are already capture-confirmed
