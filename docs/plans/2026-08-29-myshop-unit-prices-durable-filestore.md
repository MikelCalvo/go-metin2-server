# MYSHOP unit-price durable FileStore rematerialize — 2026-08-29

## Objective

Close the reconnect / process-restart gap after process-local silk-bag
`MyShopPriceList` rematerialize: persist remembered unit prices on the selected
character account FileStore snapshot so a later silk-bag `ITEM_USE` /
`/use_item` in a new session still emits the owned `MyShopPriceList` command
burst — without inventing GD/DB `MYSHOP_PRICELIST_*` packets, SQL columns,
quest-running open blocks, bag-missing INFO, or shopkeeper polymorph.

## Why this exists

Track C already owns process-local remembered unit prices from accepted
silk-path `CG::MYSHOP` open and first silk USE rematerialize
(`docs/plans/2026-08-28-myshop-bag-use-openprivateshop-pricelist.md`). That map
lives only in the same-socket session factory, so reconnect / daemon restart /
new session starts empty and first silk USE falls back to dummy
`MyShopPriceList 1 0`. The external oracle persists owner pricelists through DB;
bootstrap's durable stand-in is the already-owned account FileStore character
snapshot. Pure GD packet substrate is still deferred.

## Contract to freeze (before / with GREEN)

1. **Durable field**: optional `myshop_unit_prices` on `loginticket.Character`
   (account FileStore JSON). Each row is `{ "vnum": uint32, "unit_price": uint32 }`.
   - omitted / empty → no remembered prices (same as today)
   - at most `40` rows (`SHOP_PRICELIST_MAX_NUM` / `ShopHostItemMax`)
   - each `vnum` must be non-zero; duplicate `vnum` fails closed at account save/load
   - `unit_price` may be zero (dummy path remains `MyShopPriceList 1 0` only when
     the remembered map is empty — a stored `(1, 0)` row is still a real entry)
   - persisted JSON omits the field when empty so older character snapshots stay
     byte-compatible; when present, rows are written sorted by ascending `vnum`
2. **Write path**: on accepted **silk-path** `CG::MYSHOP` open success (the same
   branch that already calls `rememberMyShopUnitPricesFromStock`), replace the
   durable character `myshop_unit_prices` with
   `unitPrice = listed_price / listed_count` per distinct listed stock `vnum`
   (integer division), persist the account snapshot, and keep the process-local
   map + `myShopPriceListRematerialized = false` behavior already owned.
   Ordinary `50200` bag open does **not** update durable or process-local prices.
3. **Hydrate path**: when the session selects / rematerializes the character from
   account FileStore (EnterGame / select / account reload into the session
   ticket), copy durable `myshop_unit_prices` into the process-local remembered
   map and leave `myShopPriceListRematerialized = false` so the next silk USE in
   that session emits the owned price-list dump then `OpenPrivateShop`.
4. **Silk USE burst**: unchanged wire —
   - first silk USE after hydrate / after silk open → `MyShopPriceList` lines
     (or dummy `1 0` when empty) then `OpenPrivateShop`
   - later same-session silk USE → `OpenPrivateShop` only
5. **Fail-closed store**: account FileStore rejects malformed
   `myshop_unit_prices` (too many rows, zero `vnum`, duplicate `vnum`) before
   commit. Legacy snapshots without the field continue to load.
6. Spec/QA/roadmap name this beside owned MYSHOP bag USE once GREEN. Do **not**
   invent GD `MYSHOP_PRICELIST_REQ` / `UPDATE` / `RES`, SQL `myshop_pricelist`
   tables, quest-running, bag-missing INFO, zone gates, or polymorph.

## Locale / wording note

Reuse owned `MyShopPriceList %u %u` / `OpenPrivateShop` command payloads. No new
English INFO string.

## Explicit non-goals

- GD/DB `MYSHOP_PRICELIST_*` packets or `db/myshop_pricelist` SQL migration
- bag-missing INFO (oracle ordinary-bag miss stays silent)
- quest-running open block
- shopkeeper polymorph / mount teardown
- tax / empire guest-buy multipliers
- refine catalysts / `fail_result_vnum` SQL companion
- changing process-local same-session rematerialize already GREEN

## Proof shape

1. Catalog/store: account FileStore round-trips `myshop_unit_prices`; rejects
   >40 rows / zero vnum / duplicate vnum; omits empty field in deterministic JSON.
2. Session: silk-path `CG::MYSHOP` open → close shop → new session / process
   restart → silk bag USE emits remembered `MyShopPriceList` lines (sorted) then
   `OpenPrivateShop` without inventing GD packets.
3. Negatives: ordinary `50200` open does not persist prices; empty durable map
   still emits dummy `MyShopPriceList 1 0` on first silk USE; same-session later
   silk USE still emits only `OpenPrivateShop`.

## Status

GREEN for bootstrap scope on `lane/items`: silk-path `CG::MYSHOP` open persists
canonical `myshop_unit_prices` on the selected character account FileStore
snapshot; select / account rematerialize hydrates the process-local remembered
map; post-restart silk bag USE rematerializes owned `MyShopPriceList` lines then
`OpenPrivateShop` without GD/DB packets. Ordinary `50200` bag open still does
not write durable prices. GD/DB `MYSHOP_PRICELIST_*`, quest-running, bag-missing
INFO, and shopkeeper polymorph stay deferred.
