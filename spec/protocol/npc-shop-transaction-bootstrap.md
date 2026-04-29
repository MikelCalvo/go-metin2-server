# NPC Shop Transaction Bootstrap

This document freezes the first real merchant-transaction gate for `go-metin2-server`.

The goal is intentionally narrow:
- move from read-only structured `shop_preview` catalogs toward one real purchase path
- record the buy request contract inside the now-frozen minimal merchant packet family without pretending the project already owns the full final merchant-window choreography
- make the buy-only implementation gate explicit enough that the next RED tests can stay small and honest
- keep sell-back, storage, and richer merchant UI semantics out of scope

It sits on top of:
- `npc-shop-open-close-bootstrap.md`
- `npc-shop-preview-bootstrap.md`
- `npc-shop-catalog-bootstrap.md`
- `inventory-equipment-bootstrap.md`
- `item-use-bootstrap.md`
- `static-actor-interaction-request.md`

## Scope

This first transaction contract applies only to:
- a connected selected character already in `GAME`
- a visible bootstrap static actor whose interaction resolves to a valid structured `shop_preview` catalog
- one buy-only merchant path that debits gold and grants exactly one authored catalog entry per request
- self-only state mutation for currency and carried inventory
- deterministic validation against the already-owned item-template catalog

This slice does **not** yet apply to:
- sell-back
- personal shops / `MYSHOP`
- safebox / mall / storage
- multi-tab or drag-drop basket semantics
- quest-scripted merchant branching
- stock depletion, restock timers, or shared merchant state

## Runtime prerequisites already satisfied

The first buy-only merchant path is intentionally gated behind runtime/state seams the repository already owns:
- carried inventory state
- equipped-state bookkeeping
- persisted/live currency (`gold`)
- deterministic item templates keyed by stable `vnum`
- deterministic structured merchant catalogs keyed by stable `kind + ref`

This slice does not reopen those earlier contracts.
It defines how the first merchant transaction path is allowed to depend on them.

## Known merchant packet families

Session open/close choreography is now frozen separately in:
- `npc-shop-open-close-bootstrap.md`

This section focuses only on the merchant packet-family facts that the buy-only transaction gate depends on.

Current compatibility references already indicate the merchant family names and top-level headers:
- client -> server: `SHOP`, header `0x0801`
- server -> client: `SHOP`, header `0x0810`

Current compatibility references also indicate these subheader families:
- client-side `SHOP` subheaders:
  - `END`
  - `BUY`
  - `SELL`
  - `SELL2`
- server-side `SHOP` subheaders:
  - `START`
  - `END`
  - `UPDATE_ITEM`
  - `UPDATE_PRICE`
  - `OK`
  - `NOT_ENOUGH_MONEY`
  - `SOLDOUT`
  - `INVENTORY_FULL`
  - `INVALID_POS`
  - `SOLD_OUT`
  - `START_EX`
  - `NOT_ENOUGH_MONEY_EX`

This document freezes only the first buy-only path.
It does **not** claim that every listed subheader is already capture-confirmed or implemented by this repository.

## First owned transaction gate

The first owned merchant transaction path is anchored to the repository's already-owned merchant interaction resolution, not to a claimed full client-window model.

The gate is:
- the player must first resolve a visible merchant actor through the existing merchant interaction path
- that actor must resolve to a valid structured `shop_preview` definition
- the runtime must bind the resulting catalog snapshot to the interacting session as the current buyable merchant context
- only then may a later merchant `BUY` request be interpreted against that catalog

The gate must fail closed when:
- no current merchant context exists for the session
- the merchant actor is no longer visible or interactable
- the bound interaction definition can no longer resolve
- the bound catalog snapshot is stale or inconsistent with the current authored definition
- the session leaves `GAME`, disconnects, transfers maps, or otherwise loses the current merchant interaction context

This keeps the first real buy path tied to the existing authored merchant surface instead of inventing a second unrelated NPC economy entry seam.

## First BUY request contract

The first buy-only merchant request freezes only the minimum the repository can state honestly today:
- packet family: client -> server `SHOP`
- header: `0x0801`
- required subheader: `BUY`
- the request is only valid while the session currently holds an active merchant transaction gate

Current compatibility references indicate that the `BUY` request carries exactly two trailing bytes after the common `SHOP` envelope.
This document freezes only one semantic fact from that shape:
- the **second trailing byte** is the zero-based merchant `catalog_slot` to purchase

The **first trailing byte** is still treated as unknown for project-owned protocol purposes.
It must be present for compatibility, but its final meaning remains capture-gated before full wire-level ownership is claimed.

The purchased slot must address the same stable merchant entry identity already frozen in `npc-shop-catalog-bootstrap.md`:
- `catalog[].slot`
- dense zero-based ordering
- template-backed `item_vnum`
- authored `price`
- authored `count`

## Server-side validation rules

When a gated `BUY` request arrives, the runtime must validate all of the following before mutating state:
- a current merchant transaction gate exists for the session
- the requested `catalog_slot` exists in the bound catalog snapshot
- the resolved catalog entry still refers to a valid owned item template
- the entry `price` is greater than zero
- the entry `count` is greater than zero
- the selected character has at least that much gold available
- the selected character has enough carried-inventory capacity to receive the item count for that template
- persistence/writeback can succeed before the new live state is committed

The first buy-only contract intentionally remains single-entry and immediate:
- one request buys one catalog entry
- no basket
- no multi-buy quantity chooser
- no sell-side inventory input
- no shared merchant stock decrement

## Success and failure semantics

### Success path

When validation succeeds:
1. exactly the requested entry price is debited from the selected character's gold
2. exactly the requested entry count of that template is granted into carried inventory
3. the updated selected-character snapshot is persisted before the new live state is committed
4. the transaction commits atomically from the perspective of the selected runtime

This slice freezes the success path primarily at the **state** level.
It does **not** yet claim the final client-visible merchant-window choreography.

### Failure path

The first buy-only path must fail closed when any of these are true:
- no active merchant transaction gate exists
- the requested slot is unknown or stale
- the catalog/template resolution fails
- the player has insufficient gold
- no valid carried inventory placement exists
- persistence/writeback fails

Failure behavior in this bootstrap contract:
- no partial live mutation may remain committed
- no gold may be debited on failure
- no item may be granted on failure
- the runtime must preserve the pre-request selected-character state

Compatibility-oriented server `SHOP` failure subheaders are now acknowledged as likely relevant, especially:
- `NOT_ENOUGH_MONEY`
- `INVENTORY_FULL`
- `INVALID_POS`

However, the exact mapping between server-side failure causes and final client-visible `GC::SHOP` responses remains capture-gated.

## Explicit unknowns before full GREEN ownership

The following are still intentionally unknown and must be captured or pinned by RED tests before broader implementation claims:
- the final semantic meaning of the first trailing byte in client `SHOP BUY`
- the exact payload layout of the planned `GC::SHOP START` open response
- whether later compatibility work must switch from the currently planned `GC::SHOP START` path to `GC::SHOP START_EX`
- the exact minimal `GC::SHOP` success/failure sequence the client expects to keep its merchant UI stable
- whether successful purchase requires additional merchant-side item/update frames beyond the authoritative state mutation
- whether explicit `GC::SHOP END` is mandatory on every close path while the socket remains alive in `GAME`
- whether multi-tab addressing changes the future meaning of `catalog_slot`

These unknowns are the implementation gate.
The repository should not pretend they are solved before tests or captures prove them.

## Explicit non-goals

This slice does **not** yet freeze:
- `SELL`
- `SELL2`
- sell-price rules or vendor trash flow
- personal-shop (`MYSHOP`) behavior
- merchant stock depletion
- merchant refresh timers
- multi-tab cash/coin shops
- safebox, mall, or storage integration
- quest-driven merchant dialogs or special-case shop scripts

## Temporary RED harness before wire ownership

The next RED is allowed to exercise the first buy-only merchant path through a **temporary local-only harness**:
- resolve merchant context through the already-owned `INTERACT` path against a visible structured `shop_preview`
- trigger a local-only buy attempt by stable authored `catalog_slot`
- assert authoritative **state-level** outcomes first: gold debit, inventory grant, insufficient-gold rejection, and no-free-slot rejection

For the current bootstrap runtime, that harness may temporarily ride the existing talking-chat command seam if that keeps the next GREEN slice tiny.

This does **not** replace the long-term ownership target:
- the final compatibility-facing ingress is still the client `SHOP` family
- the temporary harness must stay local/bootstrap-scoped
- later packet-binding slices must be free to swap the ingress without changing the state contract frozen above

## Success definition

After this slice, the repository should be able to say:
- the first real merchant transaction family is no longer undefined in project-owned docs
- the project now knows the buy-only gate sits on top of the existing structured `shop_preview` merchant surface
- the known `SHOP` family names and headers are recorded without overstating full UI ownership
- the minimum stable `BUY` addressing fact is frozen: the second trailing byte selects the authored catalog slot
- the next RED tests can target gold debit, inventory grant, insufficient-gold rejection, and no-free-slot rejection without pretending sell or full merchant-window choreography already exist
