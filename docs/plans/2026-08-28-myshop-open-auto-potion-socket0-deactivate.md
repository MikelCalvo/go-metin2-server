# MYSHOP open auto-potion socket0 deactivate (+ instance sockets) — 2026-08-28

## Objective

Freeze the next evidence-backed host-only `CG::MYSHOP` open success companion
after cancel-active-exchange: when a listed stock cell is an oracle auto-potion
vnum (`72723..72730`) whose live `socket0 == 1`, accepted open must clear that
socket to `0`, persist the instance, and emit self-only `ITEM_UPDATE` before the
owned live `SHOP_SIGN` burst — matching the external `OpenMyShop` MR-3
deactivate step without inventing auto-potion USE/affect activation,
`MYSHOP_PRICELIST`, quest-running open blocks, bag-missing INFO, or
shopkeeper polymorph / horse / mount teardown.

## Why this exists

Oracle `OpenMyShop` walks listed stock and, for auto HP/SP recovery vnums with
`GetSocket(0) == 1`, calls `SetSocket(0, 0)` before bag/silk success and
`SHOP_SIGN`. Bootstrap currently encodes every inventory refresh from
**template** sockets (`bootstrapItemSockets`) because `inventory.ItemInstance`
has no per-instance sockets, so that deactivate is impossible without inventing
a substrate. Track C prefers this client-visible open-success companion over
DB pricelist rematerialize, quest `IsRunning`, inventing bag-missing INFO
(oracle bag-miss is silent), or appearance/polymorph seams that the Go runtime
does not own yet.

## Contract to freeze (before RED)

### A. Minimal per-instance sockets substrate

1. **Model**: `inventory.ItemInstance` gains optional per-instance sockets
   sufficient to carry the owned wire width (`itemproto.ItemSocketCount == 3`).
   Prefer a presence-aware form (pointer / `HasSockets` / equivalent) so that
   persisted `socket0 = 0` after deactivate is distinguishable from “no instance
   sockets yet → fall back to template”.
2. **Encode preference**: inventory `ITEM_SET` / `ITEM_UPDATE` (and any other
   carried refresh that today calls `bootstrapItemSockets`) must prefer
   instance sockets when present; otherwise keep the owned template fallback.
3. **Persistence**: FileStore / account character inventory JSON round-trips the
   optional instance sockets deterministically. SQL `0003` socket columns stay
   deferred (export/import follow-on); missing/omitted sockets remain valid.
4. **Non-goals for the substrate**: no auto-potion USE path, no
   `AFFECT_AUTO_*`, no full metin/bonus socket gameplay, no attribute-instance
   substrate beyond what encode already pulls from templates.

### B. MYSHOP open deactivate

1. **Vnum set** (oracle `ITEM_AUTO_HP_RECOVERY_{S,M,L,X}` /
   `ITEM_AUTO_SP_RECOVERY_{S,M,L,X}`):
   `72723`, `72724`, `72725`, `72726`, `72727`, `72728`, `72729`, `72730`.
2. **When**: only on accepted open **success** paths (silk `71049` bag-less and
   ordinary `50200` consume), after all reject gates and after any ordinary bag
   debit/persist, and **before**:
   - exchange cancel `GC::EXCHANGE END` (when an active shell exists), and
   - live `GC::SHOP_SIGN` (+ peer around-broadcast as already owned).
3. **Mutation**: for each listed stock cell whose live carried item `vnum` is in
   the set above and whose effective `socket0 == 1`:
   - ensure instance sockets are present (seed from the resolved template when
     the instance had no sockets yet);
   - set `socket0 = 0` (leave socket1/socket2 unchanged);
   - persist the host account inventory together with the open (same Save
     boundary already used by bag consume / open);
   - emit one self-only `ITEM_UPDATE` for that cell reflecting the cleared
     sockets.
4. **Ordering on success**:
   - ordinary bag: bag refresh → auto-potion `ITEM_UPDATE`(s) → exchange `END`
     (if any) → live `SHOP_SIGN`
   - silk: auto-potion `ITEM_UPDATE`(s) → exchange `END` (if any) → live
     `SHOP_SIGN`
5. **Non-mutation**:
   - listed auto-potions with `socket0 != 1` stay unchanged;
   - non-auto / non-listed cells stay unchanged;
   - reject / busy / missing-bag / already-open close paths never mutate
     sockets and never emit these deactivate updates;
   - do **not** invent guest-browse stock socket rematerialize beyond the
     already-owned browse path reading live cells after open (once instance
     sockets exist, browse naturally sees `0`).
6. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
7. Do **not** invent quest-running open block, `MYSHOP_PRICELIST`, Canada
   banword bypass, bag-missing INFO, shopkeeper `SetPolymorph(30000)` / horse /
   mount teardown, guest-buy tax/empire `*3`, or `LESS_GOLD` auto-cancel.

## Locale / wording note

No new INFO chat. Success reuses owned `ITEM_UPDATE` + `SHOP_SIGN` (+ exchange
`END` when cancelling). Do not copy oracle source comments or Korean keys into
runtime code.

## Explicit non-goals

- auto-potion USE / affect activation / recovery ticks
- shopkeeper polymorph / horse / mount teardown or close restore
- `MYSHOP_PRICELIST` / GD price-list packets / silk DB rematerialize
- quest-running open block (`PC::IsRunning`)
- bag-missing INFO chat (oracle ordinary-bag miss stays silent)
- Canada-locale banword bypass / DB banword reload
- guest-buy tax / empire `*3`
- SQL `0003` socket columns / import backfill
- refine keep-grade / catalysts; OR-materials; binary cube headers
- `LESS_GOLD` exchange auto-cancel

## Proof shape (for the later GREEN / RED)

1. Catalog/runtime: instance sockets round-trip through account FileStore;
   encode prefers instance when present; omitted sockets keep template
   fallback.
2. Runtime/session: seed listed `72723` with instance `socket0=1` + ordinary
   bag → open emits bag refresh → `ITEM_UPDATE(socket0=0)` → live `SHOP_SIGN`;
   account persists `0`.
3. Runtime/session: same deactivate on silk path (no bag debit); one SP vnum
   twin.
4. Negatives: `socket0=0` / non-auto listed vnum / armor|gold|banword|cash|
   equipped|locked|missing-bag rejects → no socket mutation; exchange
   cancel-on-open and guest browse regressions stay GREEN.

## Status

GREEN on `lane/items`: minimal per-instance inventory sockets substrate plus
accepted MYSHOP open auto-potion `socket0` deactivate (bag + silk success paths)
with focused store/encode/runtime coverage. `MYSHOP_PRICELIST` / quest-running /
bag-missing INFO / shopkeeper polymorph / refine keep-grade stay deferred;
`LESS_GOLD` auto-cancel stays out.
