# NPC Shop Transaction Bootstrap

This document freezes the first real merchant-transaction gate for `go-metin2-server`.

The goal is intentionally narrow:
- move from read-only structured `shop_preview` catalogs toward real bootstrap merchant transactions
- record the buy request contract inside the now-frozen minimal merchant packet family without pretending the project already owns the full final merchant-window choreography
- record the first live sell-back seam only after it has its own focused packet/runtime coverage
- keep personal shops, storage, and richer merchant UI semantics out of scope, while routing owned `BUY`, `SELL`, and `SELL2` ingress through dedicated game-flow seams

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
- one merchant buy path that debits gold and grants exactly one authored catalog entry per request
- one merchant sell-back path that targets carried inventory slots while an active merchant window exists
- self-only state mutation for currency and carried inventory
- deterministic validation against the already-owned item-template catalog

This slice does **not** yet apply to:
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

This document freezes the first packet/runtime `BUY` path, the later focused bootstrap `SELL` / `SELL2` sell-back seam, and the reusable `GC::SHOP UPDATE_ITEM` codec shape used by legacy shop-slot refreshes.
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

The first merchant buy request freezes only the minimum the repository can state honestly today:
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
- one-page bootstrap catalog size capped at the currently owned `SHOP_HOST_ITEM_MAX = 40` normal shop entries
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

The first buy contract intentionally remains single-entry and immediate:
- one request buys one catalog entry
- no basket
- no multi-buy quantity chooser
- no sell-side inventory input on `BUY` itself; sell-back is frozen separately below through `SELL` / `SELL2`
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
- that packet-path success does **not** append an extra bare self-only `GC::SHOP OK`; the changed carried-slot refreshes are the complete visible success companion for this packet path
- the packet-path success no longer ends on the older placeholder `CHAT_TYPE_INFO("Merchant purchase complete.")`

That owned seam remains intentionally small:
- it applies to successful packet `SHOP BUY` while the merchant session is still active
- the temporary local `/shop_buy <slot>` debug harness remains a local QA/debug ingress and may still append the older bare `GC::SHOP OK` after its item refreshes until a later slice removes or replaces that debug surface
- it does not yet emit any extra merchant-family `UPDATE_ITEM` / `UPDATE_PRICE` choreography

### Stale post-reclaim isolation

If a socket already lost live shared-world ownership because another session reclaimed the same selected character:
- packet `SHOP BUY` may still return the same self-local packet success burst (`ITEM_SET` refreshes only, with no extra `GC::SHOP OK`) to that stale socket
- the local `/shop_buy <slot>` debug harness may still return its local debug success burst (`ITEM_SET` refreshes plus the older bare `GC::SHOP OK`) to that stale socket until that debug surface is tightened separately
- that stale buy mutation must not persist updated `gold` or `inventory`
- that stale buy mutation must not replace the replacement live owner's exact-name loopback inventory/currency snapshots
- if that stale socket later closes, a fresh reconnect/bootstrap must still reload the authoritative persisted `gold`/inventory state rather than the stale socket's local divergence
- no peer-facing packets are emitted from that stale socket for this bootstrap merchant-buy path

This keeps the first merchant transaction seam consistent with the current reconnect/reclaim ownership contract without widening it into final duplicate-session merchant semantics.

### Failure path

The first buy path must fail closed when any of these are true:
- no active merchant transaction gate exists
- the requested slot is unknown or stale
- the catalog/template resolution fails
- the player has insufficient gold
- no valid carried inventory placement exists
- persistence/writeback fails

The first sell/sell2 path must fail closed when any of these are true:
- no active merchant transaction gate exists
- the requested carried slot is unknown or empty
- the requested count is zero or larger than the carried stack
- the item is currently equipped or otherwise not in a plain carried state
- the carried item is marked runtime-locked
- the template is marked `anti_sell`
- the template has no sell price
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
- local `/shop_buy <slot>` now mirrors the same `GC::SHOP INVALID_POS` unknown-slot companion as the packet path for this first bootstrap merchant-buy surface

### Frozen bare server shop result codecs

The repository also owns the exact bare-frame codec shape for the remaining compatibility-oriented no-payload shop result subheaders in the current `ShopSub::GC` ordering:
- `SOLDOUT = 6`
- `SOLD_OUT = 9`
- `NOT_ENOUGH_MONEY_EX = 11`

Each currently freezes only the common server `SHOP` envelope (`0x0810`) plus the one-byte subheader payload, with no trailing fields.

This is a codec-only ownership step for later stock, extended-shop, and player-shop slices:
- the bootstrap NPC `BUY`, `SELL`, and `SELL2` runtime paths still do not emit `SOLDOUT`, `SOLD_OUT`, or `NOT_ENOUGH_MONEY_EX`
- the exact mapping between future merchant failure causes and these result subheaders remains capture-/slice-gated

### Frozen `GC::SHOP START_EX` codec seam

The legacy-compatible extended shop open packet is now frozen at the codec level:
- server family: `SHOP`, header `0x0810`
- subheader: `START_EX = 10`
- fixed fields after the subheader: `owner_vid uint32 LE`, `shop_tab_count uint8`
- the fixed fields are followed by exactly `shop_tab_count` tab records
- each tab record is `name[32]`, `coin_type uint8`, and `40` normal shop item entries
- each item entry uses the same layout as the existing `START` and `UPDATE_ITEM` item entries

This is still runtime-gated:
- the bootstrap NPC `BUY`, `SELL`, and `SELL2` runtime paths do not emit `START_EX`
- multi-tab and secondary-coin shop behavior remains a later dedicated slice
- this slice only gives later extended-shop work an exact encoded/decoded packet shape to build on

### Frozen `GC::SHOP UPDATE_ITEM` codec seam

The legacy client handles `GC::SHOP UPDATE_ITEM` as a merchant-slot refresh: it reads one `pos` byte plus one `packet_shop_item` payload and refreshes the shop window for that slot.
The repository now owns that packet shape at the codec level:
- server family: `SHOP`, header `0x0810`
- subheader: `UPDATE_ITEM = 2`
- payload after the subheader: `pos uint8` + one normal shop catalog item entry
- item entry layout matches the currently frozen `START` catalog entry layout: `vnum uint32 LE`, `price uint32 LE`, `count uint8`, `display_pos uint8`, three little-endian `int32` sockets, and seven `(type uint8, value int16 LE)` attributes

This is a codec-only compatibility seam for later stock/sold-out/player-shop refresh work.
The current bootstrap NPC `BUY`, `SELL`, and `SELL2` runtime paths still use the already-owned selected-character inventory refreshes plus their separately frozen merchant companions: packet `SHOP BUY` success is item-refresh-only, sell success still appends bare `GC::SHOP OK`, and error paths use the owned bare merchant error frames.
They do not emit `UPDATE_ITEM` yet.

### Frozen `GC::ITEM_UPDATE` codec seam

The legacy client handles `GC::ITEM_UPDATE` as a count/socket/attribute refresh for an already-known item cell.
The repository now owns that packet shape at the codec level:
- server family: item update, header `0x0514`
- payload: `TItemPos` (`window_type uint8`, `cell uint16 LE`) + `count uint8` + three little-endian `int32` sockets + seven `(type uint8, value int16 LE)` attributes
- unlike `GC::ITEM_SET`, this packet does not carry `vnum`, flags, anti-flags, or highlight

This packet shape is now reused by the first lighter-weight runtime refresh slice.
The current bootstrap merchant buy path still emits `ITEM_SET` for changed non-empty stacks, and whole-stack merchant sells still emit `ITEM_DEL` for removed stacks.
Partial-stack `SHOP SELL2` success now emits `ITEM_UPDATE` for the already-known carried cell instead of replaying a full `ITEM_SET` with `vnum`, flags, anti-flags, or highlight.

### Runtime-locked item sell guard

The bootstrap item instance model now carries a runtime `locked` flag so merchant sell validation can preserve a narrow legacy-style guard for items that should remain visible in carried inventory but temporarily cannot be sold.
For this slice:
- account and login-ticket stores round-trip the `locked` bit as part of each carried/equipped item instance
- carried-item slot movement preserves the lock bit while clearing equipment state as before
- packet `SHOP SELL` / `SHOP SELL2` reject locked carried items before mutation
- rejection emits the same self-only `GC::SHOP INVALID_POS` frame used by the current anti-sell / equipped-item sell guards
- live inventory, live gold, persisted inventory, and persisted gold remain unchanged on the locked-item rejection path

This is still a bootstrap runtime guard, not a full legacy item-lock system:
- the current runtime does not yet own lock acquisition/release packets or timers
- equipment, trade, storage, drop, and personal-shop lock semantics remain later slices
- no new client-visible lock state packet is introduced in this slice

### Frozen `GC::SHOP UPDATE_PRICE` codec seam

The legacy client packet headers define `GC::SHOP UPDATE_PRICE` as a merchant-window price refresh with one signed Elk amount:
- server family: `SHOP`, header `0x0810`
- subheader: `UPDATE_PRICE = 3`
- payload after the subheader: `iElkAmount int32 LE`

The repository now owns that packet shape at the codec level only.
The current bootstrap NPC `BUY`, `SELL`, and `SELL2` runtime paths still do not emit `UPDATE_PRICE`; runtime use remains capture-/slice-gated.

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
- whether later compatibility work must widen the current owned packet-path success burst (`ITEM_SET` refreshes only, with no extra bare `GC::SHOP OK`) by emitting the now-owned `UPDATE_ITEM` codec, `UPDATE_PRICE`, or both to keep the client UI fully stable
- whether explicit `GC::SHOP END` is mandatory on every close path while the socket remains alive in `GAME`
- whether multi-tab addressing changes the future meaning of `catalog_slot`

These unknowns are the implementation gate.
The repository should not pretend they are solved before tests or captures prove them.

## Bootstrap sell-back packet/runtime seam

The client-originated sell packet layouts are now owned as packet ingress plus the first live merchant-window runtime path:
- `SELL(slot)` is decoded in `GAME` and routed to the shop-sell handler when one is configured.
- `SELL2(slot,count)` is decoded in `GAME` and routed to the shop-sell2 handler when one is configured.
- The generic game-flow default handlers still reject both requests silently with no response and no phase change.
- The shipped bootstrap `gamed` runtime now configures both handlers while a structured merchant `shop_preview` window is active.

The first live sell-back contract remains intentionally narrow:
- sell requests target carried inventory slots only
- `SELL(slot)` uses count `0`, meaning the full current stack
- `SELL2(slot,count)` sells the requested count, with count `0` or a count larger than the current stack meaning the full current stack
- accepted sells remove the whole stack or decrement the stack count and credit the selected character's live gold total with the owned template-derived sell credit
- ordinary sell credit derives from loaded item-template `shop_buy_price` as `floor((shop_buy_price * sold_count) / 5)` minus `floor(3% tax)`
- templates flagged `sell_count_per_gold` follow the legacy count-per-gold branch first: use `floor(sold_count / shop_buy_price)` when `shop_buy_price > 0`, or `sold_count` when it is zero, then apply the same `/5` and 3% tax floor; if the resulting credit is zero, the bootstrap runtime fails closed
- templates flagged `anti_sell` fail closed before credit calculation, return bare self-only `GC::SHOP INVALID_POS` on the packet sell path while a merchant window is active, and leave live plus persisted inventory/currency unchanged
- the updated selected-character snapshot is persisted before the live shared-world registration is refreshed
- if persistence/writeback fails, the runtime rolls the selected character's live gold and carried inventory back to the pre-sell snapshot, emits no success frames, and leaves the persisted account snapshot unchanged
- whole-stack success emits self-only `ITEM_DEL(slot)`, then self-only `PLAYER_POINT_CHANGE(type = POINT_GOLD, amount = credited_elk, value = new_gold)`, then bare self-only `GC::SHOP OK`
- partial-stack success emits self-only `ITEM_UPDATE(slot, remaining_count)`, then self-only `PLAYER_POINT_CHANGE(type = POINT_GOLD, amount = credited_elk, value = new_gold)`, then bare self-only `GC::SHOP OK`
- invalid slots, equipped items, zero unit price, and arithmetic overflow fail closed without mutating live or persisted state
- an invalid packet/runtime sell while an active merchant window exists returns bare self-only `GC::SHOP INVALID_POS`
- stale active merchant context still returns `GC::SHOP END`, clears the active context, and leaves inventory/currency unchanged
- if a socket already lost live shared-world ownership because another session reclaimed the same selected character, packet `SHOP SELL` / `SHOP SELL2` may still return the same self-local sell success burst (`ITEM_DEL` or `ITEM_UPDATE` plus bare `GC::SHOP OK`) to that stale socket
- that stale sell mutation must not persist updated `gold` or `inventory`
- that stale sell mutation must not replace the replacement live owner's exact-name loopback inventory/currency snapshots
- no peer-facing packets are emitted from that stale socket for this bootstrap merchant-sell path

The packet/runtime path now loads the item shop-buy price, count-per-gold flag, and first anti-sell policy through `itemstore.Template.shop_buy_price`, `itemstore.Template.sell_count_per_gold`, and `itemstore.Template.anti_sell`, then applies the legacy-compatible count/price branch before the shared `/5` and 3% tax floors. This still is not a full 1:1 pricing claim: locked/bound instance policy, locale-specific tax variants, and final UI/result choreography remain later slices.

This slice owns the state mutation and smallest visible merchant-window companion only. It still does not freeze richer sell-result packets, merchant stock updates, tax formulas, or final client UI refresh choreography.

## Explicit non-goals

This slice does **not** yet freeze:
- full compatibility-grade sell-price rules including locked/bound item-instance policy and locale-specific tax variants
- final client-visible sell-result choreography beyond `ITEM_DEL` / `ITEM_UPDATE` + `GC::SHOP OK`
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
