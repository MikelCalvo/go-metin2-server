# Merchant SHOP SELL / SELL2 partial remainder presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after equip-side presence preserve
landed: partial-stack merchant `SHOP SELL` / `SHOP SELL2` /
`SellMerchantItem*` must keep the sold cell's **presence-aware**
sockets/attributes on the count-only remainder (including explicit zero;
omit→omit with template encode fallback), and the remainder must be an
**independent** clone so later writes cannot alias the pre-sell live
inventory pointer.

Today `SellMerchantItemForCredit` clones the inventory slice, then
overwrites the remainder cell with the pre-clone `item` value whose
sockets/attributes still point at the original live inventory. Spec/QA
still say partial sell preserves **template-authored** display metadata,
while encode already prefers `EffectiveSockets` / `EffectiveAttributes`.
Buy merge destination-wins is GREEN; sell remainder still lacks presence
proofs and still aliases.

## Why docs-first

This is priority-queue #1 (sell persistence / item-state consistency) on
the ordinary PvE merchant sell-back path. Opening RED without freezing:

- that partial sell is count-only identity-preserving on the same cell,
- that instance presence (incl. explicit zero) wins over template on the
  remainder `ITEM_UPDATE`,
- that omitted presence stays omitted (later encode may use template
  fallback),
- that the remainder must not share sockets/attributes pointers with the
  pre-sell live inventory,

would leave the stale template-only wording as the contract. Keep refine
catalysts, mall, and party ownership notices deferred. Do not reopen buy
merge / equip / safebox / exchange preserve contracts.

## Contract to freeze (before RED)

1. **Partial sell remainder**: when accepted `SellMerchantItem` /
   `SellMerchantItemWithTemplate` / `SellMerchantItemForCredit` (or packet
   `SHOP SELL` / `SHOP SELL2`) sells fewer than the live stack count, the
   remaining `ItemInstance` must:
   - keep the source item identity and slot;
   - decrement only `Count`;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the remainder
       (including explicit `{0,0,0}`) so later writes cannot alias the
       pre-sell live inventory pointer;
     - if the source `HasAttributes()`, clone those attributes onto the
       remainder (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the remainder likewise
       omits them (later encode keeps template fallback).
2. **Whole-stack sell**: when sold count equals the live stack count, the
   cell is removed (`ITEM_DEL`) as already owned; no remainder presence
   contract applies.
3. **Wire honesty**: partial-sell-burst `ITEM_UPDATE` must project
   presence-aware instance sockets/attributes via ordinary
   `EffectiveSockets` / `EffectiveAttributes` (instance presence including
   explicit zero wins over template; omitted keeps template fallback).
4. **Persistence**: the selected-character account snapshot after successful
   partial sell must round-trip independent presence-aware fields on the
   remainder (including explicit zero) without copying template display
   metadata onto instance presence.
5. **Non-goals**: inventing new sell reject/price policy, refine catalysts /
   mall / party ownership notices, buy merge reopen, or changing whole-stack
   `ITEM_DEL` / locked / anti-sell / over-count rejects already owned.

## Proof shape (RED → GREEN)

1. Unit: seed a multi-count carried stack with authoritative instance
   presence (active / explicit-zero / omitted) → partial
   `SellMerchantItem*` → remainder identity/slot preserved; presence is an
   independent clone; mutating the remainder leaves the pre-sell live
   inventory pointer unchanged; omitted stays omitted.
2. Session: open merchant → partial `SHOP SELL2` of a presence-bearing stack
   whose instance sockets/attrs differ from the loaded template → sell-burst
   `ITEM_UPDATE` + account snapshot carry instance presence (not template);
   omitted stays omitted / template-fallback encode.
3. Negatives: whole-stack remove, locked / anti-sell / over-count /
   gold-overflow rejects stay non-mutating as already owned.

## Likely files to change (GREEN)

- `internal/player/runtime.go` (`SellMerchantItemForCredit` partial branch)
- `internal/player/merchant_sell_preserve_instance_presence_test.go` (new)
- `internal/minimal/merchant_sell_preserve_instance_presence_test.go` (new)
- `spec/protocol/npc-shop-transaction-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'SellMerchant.*PreservesInstance' -count=1
go test ./internal/minimal -run 'ShopSell2.*PreservesInstance|MerchantSell.*PreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: partial-stack merchant sell remainder keeps
identity/slot and an independent presence-aware sockets/attributes clone
(including explicit zero; omit→omit) through `ITEM_UPDATE` + account snapshot
(`TestRuntimeSellMerchantItemPreservesInstancePresenceIndependently`,
`TestGameRuntimeShopSell2PartialPreservesInstanceSocketsAndAttributes`).
Whole-stack remove / reject paths stay already owned. Refine catalysts /
mall / party ownership remain deferred.
