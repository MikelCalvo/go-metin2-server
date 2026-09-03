# Merchant SHOP BUY compatible-merge destination-wins instance sockets/attributes — 2026-09-04

## Objective

Close the remaining Track C honesty twin left after destination-wins stack-merge
and MYSHOP guest-buy merge proofs landed: NPC merchant `SHOP BUY` /
`BuyMerchantItem` / `GrantCarriedItem` already merge count-only into compatible
unlocked stacks, but that seam was never named beside the frozen shared policy
and still lacks focused presence proofs.

Without an explicit freeze + proof, a later RED could invent template-socket
overwrite on merchant grant merges, or claim merchant buy is outside the owned
destination-wins rule while MYSHOP guest-buy merge is inside it.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the PvE merchant buy path.
Opening RED without freezing:

- that merchant buy / carried grant reuse the same destination-wins /
  count-only rule already owned for pickup / move / safebox / exchange / MYSHOP,
- that newly minted free-cell grants stay omit→template encode (not inventing
  template-copy onto instance presence),
- that distributed multi-stack fan-out also keeps each destination's presence,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred.

## Contract to freeze (before RED)

1. **Merchant compatible merge is destination-wins / count-only**: when
   `BuyMerchantItem` / `GrantCarriedItem` merges purchased count into one or more
   already-occupied compatible unlocked destination stacks under template
   `max_count`, only destination `Count` changes. Destination `ID`, `Slot`,
   `Locked`, and presence pointers stay the destination's (including explicit
   zero sockets/attributes and omitted presence).
2. **No source presence**: merchant grants have no live source instance; they
   must not invent template sockets/attributes onto destination presence during
   merge, and must not clear destination presence.
3. **Free-cell remainder**: any leftover count placed into a fresh carried cell
   stays omit→template encode for sockets/attributes (already-owned grant
   minting). Do not copy template display metadata onto instance presence.
4. **Wire honesty**: merge `ITEM_UPDATE` continues to encode destination
   `EffectiveSockets` / `EffectiveAttributes`.
5. **Persistence**: the selected-character account snapshot after successful buy
   must round-trip destination presence on merged cells.
6. **Non-goals**: refine catalysts / mall / party ownership / inventing
   template-to-instance copy on free-cell grants / tax-empire multipliers /
   changing already-owned MYSHOP guest-buy merge proofs.

## Proof shape (RED → GREEN)

1. Unit: seed a destination stack with authoritative instance presence (active +
   explicit-zero + omitted) → `BuyMerchantItem` / `GrantCarriedItem` merge →
   destination count increases; destination presence unchanged.
2. Session: open merchant → `SHOP BUY` that merges into a carried destination
   with different presence → buy-burst `ITEM_UPDATE` + account snapshot keep
   destination presence; free-cell remainder (if any) stays omitted.
3. Negatives: locked / over-`max_count` / inventory-full rejects stay non-mutating
   as already owned.

## Likely files to change (GREEN)

- `internal/player/compatible_stack_merge_destination_wins_test.go`
- `internal/minimal/compatible_stack_merge_destination_wins_test.go`
- `docs/plans/2026-09-03-compatible-stack-merge-destination-wins-instance-sockets-attributes.md`
- `spec/protocol/npc-shop-transaction-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'BuyMerchantItemCompatibleMerge|GrantCarriedItemCompatibleMerge' -count=1
go test ./internal/minimal -run 'MerchantBuyCompatibleMergeKeepsDestination' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: NPC merchant `SHOP BUY` / `BuyMerchantItem` /
`GrantCarriedItem` compatible-stack merges are proven count-only with
destination presence-aware sockets/attributes authoritative (including
explicit zero; omitted stays omitted) through buy-burst `ITEM_UPDATE` and the
account snapshot
(`TestRuntimeBuyMerchantItemCompatibleMergeKeepsDestinationInstancePresence`,
`TestRuntimeGrantCarriedItemCompatibleMergeKeepsDestinationInstancePresence`,
`TestGameRuntimeMerchantBuyCompatibleMergeKeepsDestinationInstancePresence`).
Free-cell grant remainders stay omit→template encode. Refine catalysts / mall /
party ownership remain deferred.
