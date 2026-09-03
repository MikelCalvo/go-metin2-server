# ITEM_DROP2 partial-drop fresh ground identity + independent instance clone — 2026-09-03

## Objective

Freeze the next Track C fail-closed drop/pickup honesty seam after safebox
partial-split independent clone landed: when counted `ITEM_DROP2` drops
`1..source_count-1` from a carried stack, the registered ground `ItemInstance`
must receive a **fresh item identity** and an **independent** presence-aware
sockets/attributes clone, matching the already-owned inventory `ITEM_MOVE`
partial-split path (`nextSplitItemID` + `WithInventorySlot` → `CloneSockets` /
`CloneAttributes`) and the just-landed safebox partial-split destination clone.

Today `droppedInventoryItem` copies the pre-drop live stack by value and only
rewrites `Count`, keeping the source `ID` and pointer fields. `RegisterGroundItem`
already clones sockets/attributes onto the ground handle, but the reused
identity still collides with the carried remainder: `PickupGroundItem` fail-closes
on `hasItemInstanceID(..., item.ID)` before merge/free-cell placement, so the
owner cannot pick up their own partial drop while the remainder still exists.

## Why docs-first

This is priority-queue #1 (reward/drop pickup safety + item-state consistency).
Opening RED without freezing:

- that only the partial-count path is in scope (not whole-stack drop),
- that the ground handle must mint a fresh ID rather than reuse the remainder,
- that the ground instance must carry an independent sockets/attributes clone
  (including explicit zero / omit→omit), even though register already clones,

would invent policy mid-implementation. Keep stack-merge attribute/socket
policy, refine catalysts, and mall deferred.

## Contract to freeze (before RED)

1. **Partial counted drop**: when accepted `ITEM_DROP2` drops `1..source_count-1`
   from a carried stack, the registered ground `ItemInstance` must:
   - receive a fresh item identity allocated through the owned split-ID
     allocator (`nextSplitItemID` or an equivalent live inventory/equipment
     max+1 helper), distinct from the carried remainder identity;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the ground
       instance (including explicit `{0,0,0}`) so later remainder/ground socket
       writes cannot alias before register;
     - if the source `HasAttributes()`, clone those attributes onto the ground
       instance (including explicit all-zero / type-zero) so later
       remainder/ground attribute writes cannot alias before register;
     - if the source omits sockets and/or attributes, the ground instance
       likewise omits them (later encode / rematerialize keeps template
       fallback).
2. **Source remainder**: the carried source cell keeps its existing identity and
   presence pointers (count-only mutation via the already-owned drop path);
   only the newly registered ground identity must mint + clone.
3. **Pickup honesty**: while the remainder still occupies inventory/equipment
   under the original identity, owner (and later public collector) pickup of the
   partial ground handle must succeed through ordinary `PickupGroundItem` /
   free-cell / compatible-merge placement without the duplicate-ID fail-closed
   gate rejecting the fresh ground identity.
4. **Persistence / rematerialize**: durable pending ground FileStore rows after
   the successful partial drop must round-trip the fresh identity plus
   independent presence-aware fields (including explicit zero), matching the
   already-owned ground sockets/attributes rematerialize seams.
5. **Whole-stack drop stays identity-preserving**: when the drop count equals
   the full carried stack, keep today's identity move onto the ground handle
   (source cell removed; no second identity).
6. **Non-goals**: inventing stack-merge attribute/socket policy on pickup merge,
   gold `DROP2`, mall, refine catalysts, peer ownership-notice policy changes,
   changing deterministic bootstrap ground `vid` derivation, or reopening
   exclusive-ownership / despawn timer contracts.

## Proof shape (for later RED → GREEN)

1. Unit/session: seed a multi-count carried stack with authoritative instance
   sockets and attributes (including one explicit-zero sockets case and one
   explicit all-zero / type-zero attributes case) → partial `ITEM_DROP2` →
   ground handle ID ≠ remainder ID; ground sockets/attributes are independent
   clones; mutating ground presence leaves the remainder unchanged (and vice
   versa); omitted-instance regression keeps template fallback / omitted
   presence.
2. Pickup twin: after the partial drop, owner pickup of the fresh ground
   identity succeeds (free-cell and/or compatible-merge as already owned) and
   preserves the cloned presence-aware fields onto the placed/merged carried
   cell through ordinary pickup frames / account snapshot.
3. Negatives: whole-stack drop keeps identity move; anti-drop / floor /
   duplicate-vid / locked rejects stay non-mutating; stack-merge attribute
   policy is not asserted here.

## Likely files to change (later GREEN, not this freeze)

- `internal/minimal/factory.go` (`droppedInventoryItem` / drop registration path)
- `internal/player/runtime.go` (optional split-ID helper exposure if drop needs
  live max+1 after the count-only remainder mutation)
- `internal/minimal/item_drop_runtime_test.go` (focused fresh-ID + clone +
  pickup twin)
- `spec/protocol/item-drop-pickup-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'TestGameRuntimeItemDrop2PartialFreshGroundInstanceIDAndClone|TestGameRuntimeItemDrop2PartialPickupPreservesClonedInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w internal/minimal/factory.go internal/player/runtime.go internal/minimal/item_drop_runtime_test.go
git diff --check
```

## Status

GREEN on `lane/items`: partial counted `ITEM_DROP2` mints a fresh ground
identity and independent presence-aware sockets/attributes clone so the owner
can pick up the split while the remainder still exists. Whole-stack drop stays
identity-preserving. Stack-merge attribute/socket policy, refine catalysts, and
mall remain deferred.
