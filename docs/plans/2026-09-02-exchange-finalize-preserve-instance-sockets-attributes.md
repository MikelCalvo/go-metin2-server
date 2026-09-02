# Exchange finalize preserves instance sockets/attributes — 2026-09-02

## Objective

Freeze the next Track C fail-closed trade honesty seam after the post-floor
refine restart matrix closed: mutual-accept `EXCHANGE` finalize must preserve
presence-aware per-instance sockets and attributes from the giver's live carried
source onto the receiver's newly placed whole-stack cell, matching the already
owned drop→pickup / safebox check-in/out / display `ITEM_ADD` substrate.

Today `exchangeDisplayedItem` remembers only `{ItemID, Vnum, Count, Slot}`, and
`exchangePlaceIncomingDisplayedItemReason` constructs a bare
`ItemInstance{ID, Vnum, Count}` for free-cell placement. That silently drops
authoritative instance sockets/attributes even though display `ITEM_ADD` already
projects them and FileStore rematerialize already round-trips them.

## Why docs-first

This is priority-queue #2 (fail-closed finalization preconditions / mutation
honesty) plus #1 (item-state consistency). Opening RED without freezing:

- which placement path must preserve (whole-stack free-cell vs stack-merge),
- whether display-map rows or live-source lookup is the source of truth,
- how omitted vs explicit-zero presence behaves after transfer,

would invent policy mid-implementation. Keep refine catalysts / mall / party
ownership notices deferred.

## Contract to freeze (before RED)

1. **Whole-stack free-cell placement**: when mutual-accept finalize places an
   incoming displayed item into a fresh carried inventory cell (the owned path
   used when no compatible unlocked stack absorbs the count), the placed
   `ItemInstance` must carry the giver's live source presence-aware sockets and
   attributes:
   - if the live source `HasSockets()`, clone those sockets onto the placed cell
     (including explicit `{0,0,0}`);
   - if the live source `HasAttributes()`, clone those attributes onto the placed
     cell (including explicit all-zero / type-zero);
   - if the live source omits sockets and/or attributes, the placed cell likewise
     omits them (later encode keeps template fallback).
2. **Source of truth**: prefer the giver's live carried item identity at
   finalize time (already revalidated by `exchangeDisplayedItemsStillLive`), not
   inventing a second copy of sockets/attributes on `exchangeDisplayedItem`
   unless a later slice proves display-map retention is required for a
   post-display live mutation that still keeps the shell open (those mutations
   already fail closed today).
3. **Receiver `ITEM_SET` honesty**: finalize burst `ITEM_SET` for the newly
   placed cell must project the preserved instance sockets/attributes the same
   way ordinary inventory encode already prefers `EffectiveSockets` /
   `EffectiveAttributes`.
4. **Persistence**: both selected-character account snapshots after successful
   finalize must round-trip the preserved presence-aware fields.
5. **Stack-merge policy stays deferred**: when finalize merges count into an
   already-occupied compatible unlocked receiver stack, keep today's count-only
   merge. Do not invent attribute/socket merge, overwrite, or reject rules in
   this slice.
6. **Non-goals**: refine catalysts / mall / party ownership notices /
   `LESS_GOLD` behavior changes / GD-DB `MYSHOP_PRICELIST` / changing display
   `ITEM_ADD` (already GREEN for sockets + attributes).

## Proof shape (for later RED → GREEN)

1. Session: seed whole-stack transferable carried items with authoritative
   instance sockets and attributes (including one explicit-zero sockets case and
   one explicit all-zero / type-zero attributes case) → mutual-accept finalize →
   receiver persisted inventory + finalize `ITEM_SET` carry those instance
   fields; omitted-instance regression keeps template fallback / omitted
   presence.
2. Negatives: stack-merge path is not asserted here; busy-window / Check / Space
   reject paths stay non-mutating; display `ITEM_ADD` preference stays owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/minimal/shared_world.go` (`exchangePlaceIncomingDisplayedItemReason`
  and/or finalize apply path that can see the live source item)
- `internal/minimal/item_exchange_runtime_test.go` (focused mutual-accept
  preserve twin)
- `spec/protocol/item-exchange-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'TestGameRuntimeItemExchangeMutualAcceptFinalizesPreservesInstanceSocketsAndAttributes' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w internal/minimal/shared_world.go internal/minimal/item_exchange_runtime_test.go
git diff --check
```

## Status

Docs/spec freeze landed on `lane/items`. Intentional RED session proof
`TestGameRuntimeItemExchangeMutualAcceptFinalizesPreservesInstanceSocketsAndAttributes`
is prepared in the working tree and left uncommitted until GREEN.
