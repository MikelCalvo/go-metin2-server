# ITEM_USE_TO_ITEM merge presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after free-cell `ITEM_PICKUP`
presence preserve landed: accepted `ITEM_USE_TO_ITEM` / `UseItemOnItem`
compatible-stack consolidation must keep **presence-aware**
sockets/attributes on count-only `ITEM_UPDATE` refreshes.

The target cell stays **destination-wins** (including explicit zero;
omit→omit with template encode fallback). A partial-merge source remainder
keeps its own independent presence clone so later writes cannot alias the
pre-merge live inventory pointer. Source presence is discarded only onto
the merged destination; it is not copied onto that destination and is not
replaced by template display arrays.

Today `UseItemOnItem` already clones the live inventory before count-only
writes, encode already prefers `EffectiveSockets` / `EffectiveAttributes`,
and a helper-only full-merge destination-wins twin exists. Protocol/QA
still say drag-to-item `ITEM_UPDATE` preserves **template-authored**
display arrays, and the session proof still seeds omitted instance
presence then asserts template display as if that were the merge contract.
Without an explicit freeze + packet proof, a later RED could copy template
display onto merged cells or treat source remainder as destination-wins.

## Why docs-first

This is priority-queue #1 (use/equip/sell persistence + item-state
consistency) on the ordinary PvE drag-stack-onto-stack path. Opening RED
without freezing:

- that compatible `ITEM_USE_TO_ITEM` stays count-only identity-preserving
  on both still-occupied cells,
- that destination instance presence (incl. explicit zero) wins over
  template **and** over discarded source presence on the target
  `ITEM_UPDATE`,
- that omitted destination presence stays omitted (later encode may use
  template fallback),
- that a partial-merge source remainder keeps an independent clone of the
  source's presence rather than inheriting destination/template arrays,

would leave the stale template-only wording as the contract. Keep refine
catalysts, mall, and party ownership notices deferred. Do not reopen
pickup destination-wins merge, free-cell pickup preserve, or direct
`ITEM_USE` remainder contracts.

## Contract to freeze (before RED)

1. **Full merge**: when the source stack fits completely into the target,
   the source cell is removed (`ITEM_DEL`) as already owned. The target
   `ItemInstance` must:
   - keep the destination item identity and slot;
   - change only `Count`;
   - keep destination presence-aware sockets/attributes:
     - if the destination `HasSockets()`, those sockets remain
       (including explicit `{0,0,0}`);
     - if the destination `HasAttributes()`, those attributes remain
       (including explicit all-zero / type-zero);
     - if the destination omits sockets and/or attributes, the merged
       cell still omits them (later encode keeps template fallback).
   Source presence is discarded by the merge.
2. **Partial merge**: when the target has only partial room, both cells
   stay occupied and only `Count` changes on each. The source remainder
   must keep the source identity/slot plus an independent clone of the
   source's presence-aware sockets/attributes (including explicit zero;
   omit→omit). The destination follows the same destination-wins rule as
   full merge.
3. **Wire honesty**: merge-burst `ITEM_UPDATE` frames must project
   presence-aware instance sockets/attributes via ordinary
   `EffectiveSockets` / `EffectiveAttributes` (instance presence including
   explicit zero wins over template; omitted instance keeps template
   fallback). Partial-merge source `ITEM_UPDATE` encodes source presence;
   target `ITEM_UPDATE` encodes destination presence.
4. **Persistence**: the selected-character account snapshot after
   successful merge must round-trip independent presence-aware fields on
   remaining cells (including explicit zero) without copying template
   display metadata onto instance presence.
5. **Non-goals**: inventing new use-to-item reject/effect policy, reopen
   destination-wins pickup / `ITEM_MOVE` / merchant-buy merge, reopen
   direct `ITEM_USE` remainder, refine catalysts / mall / party ownership
   notices, or changing last-stack `ITEM_DEL` / locked / anti-stack /
   over-count rejects already owned.

## Proof shape (RED → GREEN)

1. Unit: seed two compatible carried stacks with authoritative instance
   presence (active / explicit-zero / omitted, source ≠ destination) →
   `UseItemOnItem` full and partial merge → destination presence wins on
   the target; partial source remainder is an independent clone; mutating
   the remainder leaves the pre-merge live inventory pointer unchanged;
   omitted stays omitted.
2. Session: packet `ITEM_USE_TO_ITEM` of presence-bearing stacks whose
   instance sockets/attrs differ from the loaded template **and** from
   each other → merge-burst `ITEM_UPDATE` + account snapshot carry
   instance presence (not template); omitted stays omitted /
   template-fallback encode.
3. Negatives: last-stack source remove, locked / anti-stack / already-full
   / over-count rejects stay non-mutating as already owned.

## Likely files to change (GREEN)

- `internal/player/use_to_item_preserve_instance_presence_test.go` (new)
- `internal/minimal/item_use_to_item_preserve_instance_presence_test.go` (new)
- `spec/protocol/item-use-bootstrap.md`
- `spec/protocol/packet-matrix.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'UseItemOnItemPreservesInstance' -count=1
go test ./internal/minimal -run 'ItemUseToItem.*(PreservesInstance|KeepsDestinationInstancePresence)' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: accepted `ITEM_USE_TO_ITEM` / `UseItemOnItem`
compatible-stack merges stay count-only with destination presence-aware
sockets/attributes authoritative on the target (including explicit zero;
omitted stays omitted / template-fallback encode) and an independent
source-remainder presence clone on partial merges through `ITEM_UPDATE` +
account snapshot
(`TestRuntimeUseItemOnItemPreservesInstancePresenceIndependently`,
`TestGameRuntimeItemUseToItemPartialPreservesInstanceSocketsAndAttributes`,
`TestGameRuntimeItemUseToItemFullMergeKeepsDestinationInstancePresence`).
Last-stack source remove / reject paths stay already owned. Refine
catalysts / mall / party ownership remain deferred.
