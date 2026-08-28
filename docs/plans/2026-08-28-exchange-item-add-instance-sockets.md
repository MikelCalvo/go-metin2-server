# EXCHANGE ITEM_ADD prefers instance EffectiveSockets — 2026-08-28

## Objective

Freeze then implement the next Track C display-honesty seam after silk/shop-bag
`ITEM_USE` → `OpenPrivateShop` / `MyShopPriceList`: active-shell
`CG::EXCHANGE ITEM_ADD` must prefer per-instance `EffectiveSockets` (including
explicit zero) over template sockets, matching the already-owned
`ITEM_SET` / `ITEM_UPDATE` instance-socket substrate from MYSHOP auto-potion
`socket0` deactivate.

## Why this exists

`player.ExchangeItemAddDisplay` still hardcodes `Sockets: template.Sockets`.
Any carried cell with instance sockets (for example deactivated auto-potion
`socket0 == 0`, or seeded `{7,8,9}` while the template is `{1,2,3}`) currently
lies in the trade window even though inventory refresh frames already tell the
truth. That is priority-queue #1 display consistency, not inventing deferred
`LESS_GOLD` auto-cancel, GD/DB `MYSHOP_PRICELIST`, quest-running, bag-missing
INFO, or refine keep-grade.

## Contract to freeze (before / with GREEN)

1. **Ingress**: unchanged active-shell `ITEM_ADD` display path through
   `ExchangeItemAddDisplay` → self + peer `GC::EXCHANGE ITEM_ADD`.
2. **Sockets**:
   - when the carried instance `HasSockets()` (`Sockets != nil`), emit
     `item.EffectiveSockets(template.Sockets)` — explicit `{0,0,0}` wins over
     non-zero template sockets;
   - when instance sockets are omitted, keep template sockets (regression).
3. **Attributes**: stay template-authored for this slice (no instance attribute
   substrate yet).
4. **Non-mutation**: display still does not move ownership, debit gold, or
   persist; live/persisted inventory stays unchanged on add.
5. **Out of scope**: `LESS_GOLD` auto-cancel, GD/DB pricelist, guest MYSHOP buy
   socket rematerialize beyond already-owned paths, refine keep-grade.

## Proof shape

1. Player unit: `ExchangeItemAddDisplay` prefers instance sockets including
   explicit zero; omitted sockets keep template; no live/persisted mutation.
2. Session: active-shell `ITEM_ADD` self + peer frames carry instance sockets
   when present; omitted-instance regression still emits template sockets.

## Status

GREEN on `lane/items`: active-shell `ITEM_ADD` prefers instance
`EffectiveSockets` (including explicit zero) over template sockets; omitted
instance sockets keep template sockets; attributes stay template-authored;
display remains non-mutating. `LESS_GOLD` auto-cancel / GD/DB `MYSHOP_PRICELIST`
/ quest-running / bag-missing INFO / refine keep-grade stay deferred.
