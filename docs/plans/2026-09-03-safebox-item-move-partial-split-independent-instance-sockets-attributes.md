# Safebox ITEM_MOVE partial-split independent instance sockets/attributes — 2026-09-03

## Objective

Freeze the next Track C fail-closed storage honesty seam after exchange finalize
and MYSHOP guest-buy whole-stack preserve landed: partial-count empty-destination
`SAFEBOX_ITEM_MOVE` must place an independent clone of the source stack's
presence-aware sockets/attributes onto the newly allocated destination cell,
matching the already-owned inventory `ITEM_MOVE` partial-split path
(`WithInventorySlot` → `CloneSockets` / `CloneAttributes`).

Today the safebox partial-split empty-destination branch copies the source
`ItemInstance` by value (`resultItem = sourceItem`) and only rewrites `ID` /
`Count` / `Slot`. Because sockets and attributes are pointer fields, the new
destination cell aliases the source remainder's presence pointers. A later
mutation of either cell's sockets/attributes would silently corrupt the other.

## Why docs-first

This is priority-queue #1 (item-state consistency) plus #4 (storage honesty).
Opening RED without freezing:

- that only the empty-destination partial-split path is in scope (not
  whole-stack relocate, not compatible merge),
- that destination must clone rather than share presence pointers,
- how omitted vs explicit-zero presence behaves after the split,

would invent policy mid-implementation. Keep stack-merge attribute/socket
policy, refine catalysts, and mall deferred.

## Contract to freeze (before RED)

1. **Partial empty-destination split**: when `SAFEBOX_ITEM_MOVE` splits
   `1..source_count-1` into an empty safebox cell, the destination
   `ItemInstance` must carry an independent clone of the source's
   presence-aware sockets and attributes:
   - if the source `HasSockets()`, clone those sockets onto the destination
     (including explicit `{0,0,0}`) so later source/destination socket writes
     cannot alias;
   - if the source `HasAttributes()`, clone those attributes onto the
     destination (including explicit all-zero / type-zero) so later
     source/destination attribute writes cannot alias;
   - if the source omits sockets and/or attributes, the destination likewise
     omits them (later encode keeps template fallback).
2. **Source of truth**: the live open-presentation safebox source cell already
   validated for the move; destination gets a fresh item identity as already
   owned (`nextSafeboxSplitItemID`).
3. **`SAFEBOX_SET` honesty**: both emitted `SAFEBOX_SET` frames (source
   remainder + destination split) must project presence-aware instance
   sockets/attributes via ordinary `EffectiveSockets` / `EffectiveAttributes`.
4. **Persistence**: durable same-account safebox FileStore cells after the
   successful move must round-trip independent presence-aware fields for both
   the remainder and the split (including explicit zero).
5. **Source remainder**: the source cell keeps its existing presence pointers
   (count-only mutation); only the new destination identity must clone.
6. **Non-goals**: whole-stack relocate (already identity-preserving),
   compatible partial/whole merge attribute/socket policy (stays count-only),
   carried inventory `ITEM_MOVE` (already clones via `WithInventorySlot`),
   refine catalysts / mall / inventing merge-on-stack rules.

## Proof shape (for later RED → GREEN)

1. Unit/session: seed a multi-count safebox cell with authoritative instance
   sockets and attributes (including one explicit-zero sockets case and one
   explicit all-zero / type-zero attributes case) → partial empty-destination
   `SAFEBOX_ITEM_MOVE` → destination cell + `SAFEBOX_SET` carry independent
   clones; mutating destination sockets/attributes must leave the source
   remainder unchanged (and vice versa); omitted-instance regression keeps
   template fallback / omitted presence.
2. Negatives: compatible merge path is not asserted here; closed presentation /
   oversize / locked rejects stay non-mutating.

## Likely files to change (later GREEN, not this freeze)

- `internal/minimal/factory.go` (`HandleSafeboxItemMove` empty-destination
  partial-split placement)
- `internal/minimal/item_storage_runtime_test.go` (focused independent-clone twin)
- `spec/protocol/item-storage-guard-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'TestGameRuntimeSafeboxItemMovePartialSplitClonesInstanceSocketsAndAttributesIndependently' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w internal/minimal/factory.go internal/minimal/item_storage_runtime_test.go
git diff --check
```

## Status

GREEN on `lane/items`: partial empty-destination `SAFEBOX_ITEM_MOVE` clones
presence-aware sockets/attributes (including explicit zero) onto the new
destination identity so later source/destination writes cannot alias.
Compatible merge stays count-only. Stack-merge attribute/socket policy, refine
catalysts, and mall remain deferred.
