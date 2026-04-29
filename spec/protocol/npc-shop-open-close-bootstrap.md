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

The current browse-only `shop_preview` contract still matters here:
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
- the bound merchant actor/catalog becomes invalid before a later `BUY`

When the socket is still live and still in a state where merchant-specific frames can be delivered, the runtime should treat `GC::SHOP END` as the close companion for the currently open merchant window.

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

The repository can now say only this much honestly:
- a valid merchant interaction should open through `GC::SHOP START`
- later `BUY` and `END` requests belong to the `SHOP` family, not to ad hoc chat commands
- explicit merchant close should use `SHOP END`
- successful or failed buys may still require a minimal compatibility-facing `GC::SHOP` acknowledgement sequence in addition to the already-owned self-only `ITEM_SET` / `ITEM_DEL` / `PLAYER_POINT_CHANGE` refresh families

The exact mandatory role of:
- `OK`
- `NOT_ENOUGH_MONEY`
- `INVENTORY_FULL`
- `INVALID_POS`
- `UPDATE_ITEM`
- `UPDATE_PRICE`

is still capture-gated before the repository claims full merchant-window choreography ownership.

## Explicit unknowns before codec/runtime GREEN

The following remain intentionally unfrozen until the next packet and runtime slices prove them:
- the exact payload layout of `GC::SHOP START`
- whether later compatibility work will force `START_EX` instead of the currently planned `START` open path
- the final semantic meaning of the first trailing byte in client `SHOP BUY`
- the exact minimal success/failure `GC::SHOP` sequence needed to keep the TMP4 merchant UI stable after a `BUY`
- whether `GC::SHOP END` is required on every explicit close path or only while the socket remains otherwise stable in `GAME`
- whether any merchant-side refresh frames must accompany a successful `BUY` beyond the already-owned self-facing state refresh packets

These unknowns are the gate for the next codec and session-flow slices.

## Explicit non-goals

This slice does **not** yet freeze:
- `SELL`
- `SELL2`
- `START_EX`
- full merchant catalog payload bytes for open frames
- multi-tab merchant indexing
- cash/coin shop semantics
- merchant stock depletion or refresh timers
- personal shop flows

## Success definition

After this slice, the repository should be able to say:
- the first owned merchant window family is no longer only implied in project docs
- merchant open still starts from the already-owned `INTERACT` ingress and structured merchant resolution path
- `GC::SHOP START` is now the planned first merchant open response
- client `SHOP BUY` and `SHOP END` are now the planned client actions inside that merchant session
- `GC::SHOP END` is now the planned explicit merchant close companion when the runtime can still deliver merchant-specific frames
- the project still does not pretend that the final wire payloads or the full success/failure response choreography are already capture-confirmed
