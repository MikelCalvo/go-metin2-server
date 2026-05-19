# NPC Shop Transaction Bootstrap

This document freezes the first real merchant-transaction gate for `go-metin2-server`.

The goal is intentionally narrow:
- move from read-only structured `shop_preview` catalogs toward one real purchase path
- record the buy request contract inside the now-frozen minimal merchant packet family without pretending the project already owns the full final merchant-window choreography
- make the buy-only implementation gate explicit enough that the next RED tests can stay small and honest
- keep sell-back, storage, and richer merchant UI semantics out of scope, while still decoding the client sell ingress through a dedicated fail-closed game-flow seam

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
- while the session still owns live shared-world state, each later `BUY` must re-resolve that same merchant target and current `shop_preview` definition before mutating inventory or gold

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

Where the local `/shop_buy <slot>` debug harness exists, it must resolve through the same owned validation and carried-placement path as the current packet `SHOP BUY` gate for those same authored slots.

## Server-side validation rules

When a gated `BUY` request arrives, the runtime must validate all of the following before mutating state:
- a current merchant transaction gate exists for the session
- the requested `catalog_slot` exists in the bound catalog snapshot
- the resolved catalog entry still refers to a valid owned item template
- the entry `price` is greater than zero
- the entry `count` is greater than zero
- the selected character has at least that much gold available
- the selected character has a valid carried-inventory placement for that template/count under `item-stack-bootstrap.md`
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
2. exactly the requested entry count of that template is granted into carried inventory according to `item-stack-bootstrap.md`
3. the updated selected-character snapshot is persisted before the new live state is committed
4. the transaction commits atomically from the perspective of the selected runtime

This slice freezes the success path primarily at the **state** level.
It does **not** yet claim the final client-visible merchant-window choreography.

### Packet-path success companion

The live merchant-window success step is now owned explicitly:
- successful packet `SHOP BUY` keeps the existing self-only `ITEM_SET` refreshes for every changed carried slot in carried-slot order
- that packet-path success then appends one bare self-only `GC::SHOP OK`
- the packet-path success no longer ends on the older placeholder `CHAT_TYPE_INFO("Merchant purchase complete.")`

That owned seam remains intentionally small:
- it applies to successful packet `SHOP BUY` while the merchant session is still active
- the temporary local `/shop_buy <slot>` debug harness now reuses that same success surface (`ITEM_SET` refreshes + bare `GC::SHOP OK`) for the same authoritative success state change while still remaining only a local QA/debug ingress
- it does not yet freeze any extra merchant-family `UPDATE_ITEM` / `UPDATE_PRICE` choreography

### Stale post-reclaim isolation

If a socket already lost live shared-world ownership because another session reclaimed the same selected character:
- packet `SHOP BUY` may still return the same self-local packet success burst (`ITEM_SET` refreshes + bare `GC::SHOP OK`) to that stale socket
- the local `/shop_buy <slot>` debug harness may still return that same self-local success burst (`ITEM_SET` refreshes + bare `GC::SHOP OK`) to that stale socket
- that stale buy mutation must not persist updated `gold` or `inventory`
- that stale buy mutation must not replace the replacement live owner's exact-name loopback inventory/currency snapshots
- if that stale socket later closes, a fresh reconnect/bootstrap must still reload the authoritative persisted `gold`/inventory state rather than the stale socket's local divergence
- no peer-facing packets are emitted from that stale socket for this bootstrap merchant-buy path

This keeps the first merchant transaction seam consistent with the current reconnect/reclaim ownership contract without widening it into final duplicate-session merchant semantics.

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
- packet `SHOP BUY` insufficient-gold failure now emits one bare self-only `GC::SHOP NOT_ENOUGH_MONEY`
- packet `SHOP BUY` no-valid-placement failure now emits one bare self-only `GC::SHOP INVENTORY_FULL`
- packet `SHOP BUY` unknown-slot failure now emits one bare self-only `GC::SHOP INVALID_POS`
- packet `SHOP BUY` against a still-open merchant window whose live actor/context or bound catalog snapshot has gone stale now emits one self-only `GC::SHOP END`, clears the active merchant context immediately, and still leaves gold/inventory unchanged
- a successful warp interaction or exact-position transfer trigger while that merchant window is still open now prepends one self-only `GC::SHOP END` before the self transfer rebootstrap burst and clears the active merchant context immediately, so later `SHOP BUY` requests on the destination side fail closed until the player opens a fresh merchant window again
- the local `/shop_buy <slot>` debug harness now reuses those same merchant-family insufficient-gold / no-valid-placement / unknown-slot visible failures (`GC::SHOP NOT_ENOUGH_MONEY` / `GC::SHOP INVENTORY_FULL` / `GC::SHOP INVALID_POS`) instead of keeping a second placeholder or silent unknown-slot surface

### Frozen packet-path merchant error seam

The narrowest honest merchant-window failure contract is now live too:
- packet `SHOP BUY` insufficient-gold failure answers with one bare `GC::SHOP NOT_ENOUGH_MONEY`
- packet `SHOP BUY` no-valid-placement failure answers with one bare `GC::SHOP INVENTORY_FULL`
- packet `SHOP BUY` unknown-slot failure answers with one bare `GC::SHOP INVALID_POS`
- packet `SHOP BUY` on a stale merchant window now answers with one bare `GC::SHOP END` instead of a merchant error subheader
- all three merchant error frames use only the common `SHOP (0x0810)` envelope plus the selected error subheader, with no extra payload bytes

This freeze is intentionally narrower than the whole failure surface:
- it applies only to packet `SHOP BUY` while an active merchant session still exists
- the stale-window `GC::SHOP END` path is a close-path companion, not an additional merchant error-subheader claim
- it does not yet freeze `SOLDOUT` or `NOT_ENOUGH_MONEY_EX`
- local `/shop_buy <slot>` now mirrors the same `GC::SHOP INVALID_POS` unknown-slot companion as the packet path for this first bootstrap merchant-buy surface

Compatibility-oriented server `SHOP` failure subheaders are still acknowledged as likely relevant, especially:
- `NOT_ENOUGH_MONEY`
- `INVENTORY_FULL`
- `SOLDOUT`
- `NOT_ENOUGH_MONEY_EX`

After the freeze above, the exact mapping between other server-side failure causes and final client-visible `GC::SHOP` responses still remains capture-gated.

The first repository-owned carried placement contract now lives beside this document in `item-stack-bootstrap.md`:
- validate merchant grants against template `stackable` / `max_count`
- prefer one deterministic full merge into an existing compatible carried stack
- otherwise allow deterministic full fan-out across several existing compatible carried stacks
- otherwise allow deterministic existing-stack fan-out plus one fresh carried slot
- otherwise claim one deterministic fresh carried slot
- otherwise fail closed

## Explicit unknowns before full GREEN ownership

The following are still intentionally unknown and must be captured or pinned by RED tests before broader implementation claims:
- the final semantic meaning of the first trailing byte in client `SHOP BUY`
- whether later compatibility work must switch from the currently planned `GC::SHOP START` path to `GC::SHOP START_EX`
- whether later compatibility work must widen the current owned packet-path success burst (`ITEM_SET` refreshes + bare `GC::SHOP OK`) with additional merchant-family `UPDATE_ITEM` / `UPDATE_PRICE` frames to keep the client UI fully stable
- whether explicit `GC::SHOP END` is mandatory on every close path while the socket remains alive in `GAME`
- whether multi-tab addressing changes the future meaning of `catalog_slot`

These unknowns are the implementation gate.
The repository should not pretend they are solved before tests or captures prove them.

## Explicit fail-closed sell ingress seam

The client-originated sell packet layouts are now owned only as ingress and dispatch seams:
- `SELL(slot)` is decoded in `GAME` and routed to a dedicated shop-sell handler when one is configured.
- `SELL2(slot,count)` is decoded in `GAME` and routed to a dedicated shop-sell2 handler when one is configured.
- The default handlers reject both requests silently with no response and no phase change.
- The current shipped runtime does not configure sell handlers, so live sell-back still fails closed.

This seam exists so a later sell-back slice can attach runtime semantics without reworking the packet dispatcher.
It does not define sale pricing or merchant-window response choreography yet.

The first sell-back implementation step is deliberately state-level only, behind `internal/player` tests rather than live game-socket routing:
- sell requests target carried inventory slots only
- a requested count of `0`, or a count larger than the current stack, means the full current stack
- accepted sells remove the whole stack or decrement the stack count and credit `unit_price * sold_count` to the live runtime gold total
- invalid slots, equipped items, zero unit price, and arithmetic overflow fail closed without mutating live or persisted state
- sell mutations remain live-runtime-only until the later packet/runtime slice wires persistence and visible response frames

The current bootstrap unit price is an explicit caller-supplied test seam. The legacy oracle computes sell value from item shop-buy price, stack count, flags such as count-per-gold, and tax before `POINT_GOLD`; this repo has not yet loaded that pricing data into `itemstore.Template`, so no 1:1 sell-price claim is made here.

## Explicit non-goals

This slice does **not** yet freeze:
- live `SELL` / `SELL2` game-socket success behavior
- compatibility-grade sell-price rules or vendor trash flow
- personal-shop (`MYSHOP`) behavior
- merchant stock depletion
- merchant refresh timers
- multi-tab cash/coin shops
- safebox, mall, or storage integration
- quest-driven merchant dialogs or special-case shop scripts

## Temporary local debug harness beside wire ownership

The bootstrap runtime now accepts the client `SHOP BUY` packet family as the primary ingress for the first buy-only merchant path:
- resolve merchant context through the already-owned `INTERACT` path against a visible structured `shop_preview`
- bind that catalog snapshot as the session's active merchant transaction gate
- interpret the later client `SHOP BUY` request against that active context by authored `catalog_slot`
- assert authoritative **state-level** outcomes first: gold debit, inventory grant, insufficient-gold rejection, and no-free-slot rejection

The local talking-chat command seam may still exist as a **temporary local-only debug harness**:
- `/shop_buy <catalog_slot>` may exercise the same state contract for QA and recovery
- it must remain bootstrap-scoped rather than becoming the primary client-facing merchant path

This does **not** replace the long-term ownership target:
- the compatibility-facing ingress is the client `SHOP` family
- the temporary slash harness must stay local/bootstrap-scoped
- later merchant-window slices may refine success/failure choreography without changing the state contract frozen above

## Success definition

After this slice, the repository should be able to say:
- the first real merchant transaction family is no longer undefined in project-owned docs
- the project now knows the buy-only gate sits on top of the existing structured `shop_preview` merchant surface
- the known `SHOP` family names and headers are recorded without overstating full UI ownership
- the minimum stable `BUY` addressing fact is frozen: the second trailing byte selects the authored catalog slot
- active merchant sessions can now route real client `SHOP BUY` requests through the same authoritative gold/inventory mutation contract
- the temporary `/shop_buy <catalog_slot>` harness remains available only as a local debug seam
- focused tests can target gold debit, inventory grant, insufficient-gold rejection, and no-free-slot rejection without pretending sell or full merchant-window choreography already exist
