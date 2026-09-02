# MYSHOP guest-buy whole-stack instance socket/attribute preserve — 2026-09-03

## Objective

Close the remaining item-state honesty gap left after exchange finalize
whole-stack preserve landed: guest private-shop `SHOP BUY` already freezes
"preserve item identity / sockets / attributes / count" in
`docs/plans/2026-08-24-myshop-guest-buy-mutation-contract-freeze.md`, but
`applyMyShopGuestBuy` still strips the live host item down to a bare
`exchangeDisplayedItem` and places `ItemInstance{ID,Vnum,Count}`.

## Why RED is honest now

No new protocol seam is required. The 2026-08-24 guest-buy freeze already
names presence-aware preserve for the transferred host stack. Exchange finalize
now owns the free-cell preserve helper; MYSHOP only needs to pass the live host
`plan.Item` into that same placement path.

## Contract (already frozen; restate for this slice)

1. **Whole-stack free-cell guest placement**: when guest buy places the sold
   host stack into a fresh carried cell, the placed `ItemInstance` must carry
   the host live-source presence-aware sockets/attributes:
   - `HasSockets()` / explicit `{0,0,0}` clone onto the guest cell;
   - `HasAttributes()` / explicit all-zero / type-zero clone onto the guest cell;
   - omitted host presence stays omitted (later encode keeps template fallback).
2. **Source of truth**: the matched live host inventory item already captured on
   `myShopGuestBuyPlan.Item` at resolve time.
3. **Guest `ITEM_SET` honesty**: buy-burst `ITEM_SET` for the newly placed cell
   must project preserved instance sockets/attributes via ordinary
   `EffectiveSockets` / `EffectiveAttributes`.
4. **Persistence**: guest account snapshot after successful buy must round-trip
   those presence-aware fields.
5. **Stack-merge policy stays deferred / count-only**: if guest buy merges into
   an already-occupied compatible unlocked stack, keep today's count-only merge.
6. **Non-goals**: tax/empire multipliers / guest sell-into-PC-shop / mall /
   refine catalysts / inventing attribute merge-on-stack rules.

## Proof shape (RED → GREEN)

1. Session: host lists a whole-stack transferable carried item with
   authoritative instance sockets and attributes (including one explicit-zero
   sockets case) → guest browses empty inventory → guest `SHOP BUY` → guest
   persisted inventory + buy-burst `ITEM_SET` carry those instance fields;
   omitted-instance regression keeps template fallback / omitted presence.
2. Negatives: stack-merge path is not asserted here; sold-out / inventory-full /
   insufficient-gold rejects stay non-mutating.

## Likely files to change (later GREEN)

- `internal/minimal/factory.go` (`applyMyShopGuestBuy` free-cell placement call)
- `internal/minimal/item_myshop_runtime_test.go` (focused preserve twin)
- `spec/protocol/npc-shop-transaction-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'TestGameRuntimeMyShopGuestBuyPreservesInstanceSocketsAndAttributes' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w internal/minimal/factory.go internal/minimal/item_myshop_runtime_test.go
git diff --check
```

## Status

GREEN on `lane/items`: guest private-shop whole-stack free-cell placement
preserves host live-source presence-aware sockets/attributes through buy-burst
`ITEM_SET` and the guest account snapshot. Stack-merge attribute/socket policy,
refine catalysts, and mall remain deferred.
