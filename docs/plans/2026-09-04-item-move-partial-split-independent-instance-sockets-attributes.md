# Carried ITEM_MOVE partial-split independent instance sockets/attributes — 2026-09-04

## Objective

Freeze the next Track C honesty seam after merchant buy destination-wins merge
landed: counted partial empty-destination `ITEM_MOVE` must place an independent
clone of the source stack's presence-aware sockets/attributes onto the newly
allocated destination cell, matching the already-owned safebox partial-split and
partial `ITEM_DROP2` clone contracts and the already-shipped
`WithInventorySlot` → `CloneSockets` / `CloneAttributes` path.

Today production already clones through `nextSplitItemID` + `WithInventorySlot`,
but protocol/QA language and focused presence proofs still name only quickslot /
count refresh behavior. Without an explicit freeze + proof, a later RED could
drop the clone back to value-copy pointer aliasing (the exact bug safebox had)
while docs still claim inventory split already owns independence.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary carried move
path. Opening RED without freezing:

- that only the empty-destination partial-split path is in scope (not
  whole-stack relocate, not compatible merge),
- that destination must clone rather than share presence pointers,
- how omitted vs explicit-zero presence behaves after the split,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred. Do not reopen destination-wins compatible-merge
contracts already owned across pickup / move / safebox / exchange / MYSHOP /
merchant buy.

## Contract to freeze (before RED)

1. **Partial empty-destination split**: when carried `ITEM_MOVE` /
   `MoveInventoryItemCount` splits `1..source_count-1` into an empty carried
   cell, the destination `ItemInstance` must carry an independent clone of the
   source's presence-aware sockets and attributes:
   - if the source `HasSockets()`, clone those sockets onto the destination
     (including explicit `{0,0,0}`) so later source/destination socket writes
     cannot alias;
   - if the source `HasAttributes()`, clone those attributes onto the
     destination (including explicit all-zero / type-zero) so later
     source/destination attribute writes cannot alias;
   - if the source omits sockets and/or attributes, the destination likewise
     omits them (later encode keeps template fallback).
2. **Source of truth**: the live carried source cell already validated for the
   move; destination gets a fresh item identity as already owned
   (`nextSplitItemID`).
3. **Wire honesty**: both emitted `ITEM_SET` frames (source remainder +
   destination split) must project presence-aware instance sockets/attributes
   via ordinary `EffectiveSockets` / `EffectiveAttributes`.
4. **Persistence**: the selected-character account snapshot after the successful
   move must round-trip independent presence-aware fields for both the remainder
   and the split (including explicit zero).
5. **Source remainder**: the source cell keeps its existing presence pointers
   (count-only mutation); only the new destination identity must clone.
6. **Non-goals**: whole-stack empty-destination relocate (already
   identity-preserving), compatible partial/whole merge (already destination-
   wins count-only), refine catalysts / mall / party ownership notices, or
   inventing merge-on-stack rules.

## Proof shape (RED → GREEN)

1. Unit: seed a multi-count carried stack with authoritative instance presence
   (active / explicit-zero / omitted) → `MoveInventoryItemCount` into an empty
   cell → destination identity is fresh; destination presence is an independent
   clone; mutating destination leaves the source remainder unchanged (and vice
   versa); omitted stays omitted.
2. Session: packet `ITEM_MOVE` partial split → dual `ITEM_SET` + account snapshot
   carry preserved/cloned presence; destination identity ≠ source remainder.
3. Negatives: compatible merge stays count-only destination-wins as already
   owned; whole-stack empty move stays identity-preserving; locked / oversize /
   incompatible partial rejects stay non-mutating.

## Likely files to change (GREEN)

- `internal/player/item_move_partial_split_instance_clone_test.go`
- `internal/minimal/item_move_partial_split_instance_clone_test.go`
- `spec/protocol/item-move-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'MoveInventoryItemCountPartialSplitClones' -count=1
go test ./internal/minimal -run 'ItemMoveCountedPartialSplitClonesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: counted partial empty-destination `ITEM_MOVE` /
`MoveInventoryItemCount` clones presence-aware sockets/attributes (including
explicit zero; omit→omit with template encode fallback) onto a fresh destination
identity so later source/destination writes cannot alias
(`TestRuntimeMoveInventoryItemCountPartialSplitClonesInstancePresenceIndependently`,
`TestGameRuntimeItemMoveCountedPartialSplitClonesInstanceSocketsAndAttributesIndependently`,
`TestGameRuntimeItemMoveCountedPartialSplitOmitsInstancePresenceIndependently`).
Compatible merge stays destination-wins count-only; whole-stack empty move stays
identity-preserving. Refine catalysts / mall / party ownership remain deferred.
