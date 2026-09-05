# ITEM_MOVE partial-merge remainder presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after `ITEM_USE_TO_ITEM` merge
presence preserve landed: counted partial compatible `ITEM_MOVE` /
`MoveInventoryItemCount` must keep the source remainder's
**presence-aware** sockets/attributes as an **independent** clone
(including explicit zero; omit→omit with template encode fallback) so
later writes cannot alias the pre-merge live inventory pointer.

The destination cell stays **destination-wins** as already owned. This
slice only freezes source-remainder independence on the still-occupied
source cell after a partial merge.

Today `MoveInventoryItemCountBounded` copies the live source cell by
value (`sourceRemainder := sourceItem`), decrements `Count`, and writes
that same pointer-bearing struct back into `liveInventory`. Encode
already prefers `EffectiveSockets` / `EffectiveAttributes`, destination-
wins proofs already own the merged cell, and empty-destination partial
split already clones the new destination identity. Partial-merge source
remainder still aliases.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary
carried drag-stack-onto-stack `ITEM_MOVE` path. Opening RED without
freezing:

- that only partial compatible merge source remainder is in scope (not
  empty-destination split, not full-stack merge remove, not destination-
  wins),
- that the remainder keeps source identity/slot plus an independent
  clone of source presence (including explicit zero; omit→omit),
- that destination presence still wins on the merged cell,

would invent policy mid-implementation. Keep refine catalysts, mall, and
party ownership notices deferred. Do not reopen `ITEM_USE_TO_ITEM`
merge, empty-destination split, or destination-wins contracts.

## Contract to freeze (before RED)

1. **Partial merge remainder**: when counted compatible `ITEM_MOVE`
   leaves `1..source_count-1` on the source cell because the destination
   only had partial room, the source remainder `ItemInstance` must:
   - keep the source item identity and slot;
   - change only `Count`;
   - keep an independent clone of the source's presence-aware
     sockets/attributes (including explicit `{0,0,0}` / all-zero
     attributes; omit→omit).
2. **Destination**: stays destination-wins count-only as already owned
   (`docs/plans/2026-09-03-compatible-stack-merge-destination-wins-instance-sockets-attributes.md`).
3. **Wire honesty**: source remainder `ITEM_UPDATE` encodes source
   presence via ordinary `EffectiveSockets` / `EffectiveAttributes`;
   destination `ITEM_UPDATE` encodes destination presence.
4. **Persistence**: the selected-character account snapshot after
   successful partial merge must round-trip independent presence-aware
   fields on the remainder (including explicit zero) without copying
   destination/template arrays onto source presence.
5. **Non-goals**: empty-destination split (already cloned onto a fresh
   identity), full-stack merge source remove, destination-wins policy,
   refine catalysts / mall / party ownership notices, or changing
   locked / anti-stack / over-count rejects already owned.

## Proof shape (RED → GREEN)

1. Unit: seed two compatible carried stacks with authoritative instance
   presence (active / explicit-zero / omitted) → counted partial
   `MoveInventoryItemCount` merge → source remainder is an independent
   clone of the pre-merge live source pointer; destination stays
   destination-wins; omitted stays omitted.
2. Session: packet `ITEM_MOVE` partial merge of presence-bearing stacks
   whose instance sockets/attrs differ from the loaded template **and**
   from each other → merge-burst `ITEM_UPDATE` + account snapshot carry
   source remainder presence on the source cell (not destination/template)
   and destination presence on the target; omitted stays omitted /
   template-fallback encode.
3. Negatives: full-stack source remove, empty-destination split clone,
   locked / anti-stack / over-count rejects stay already owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/player/runtime.go` (`MoveInventoryItemCountBounded` partial
  merge remainder branch)
- `internal/player/item_move_partial_merge_preserve_instance_presence_test.go` (new)
- `internal/minimal/item_move_partial_merge_preserve_instance_presence_test.go` (new)
- `spec/protocol/item-move-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/player -run 'MoveInventoryItemPartialMergePreservesInstance' -count=1
go test ./internal/minimal -run 'ItemMovePartialMergePreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: counted and zero-count compatible partial
`ITEM_MOVE` / `MoveInventoryItemCountBounded` / `MoveInventoryItemBounded`
keep the source remainder as an independent clone of the pre-merge live
inventory presence (including explicit zero; omit→omit with template
encode fallback) through remainder `ITEM_UPDATE` + account snapshot, while
the merged cell stays destination-wins count-only
(`TestRuntimeMoveInventoryItemPartialMergePreservesInstancePresenceIndependently`,
`TestGameRuntimeItemMovePartialMergePreservesInstanceSocketsAndAttributes`).
Empty-destination split remainder pointers, full-stack source remove, and
destination-wins stay already owned. Refine catalysts / mall / party
ownership remain deferred.
