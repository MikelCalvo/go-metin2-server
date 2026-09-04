# Safebox ITEM_MOVE whole-stack empty-destination presence preserve — 2026-09-04

## Objective

Freeze the next Track C honesty seam after `SAFEBOX_CHECKIN` independent
presence clone landed: open-presentation whole-stack `SAFEBOX_ITEM_MOVE` into
an empty safebox cell must relocate the existing item identity while carrying
an **independent** presence-aware sockets/attributes clone (including explicit
zero; omit→omit), matching the already-owned check-in clone, checkout free-cell
preserve, partial-split destination clone, and `WithInventorySlot` helpers.

Today production relocates with `resultItem = sourceItem; resultItem.Slot = dest`
and deletes the source map entry, so two live cells do not alias after success,
but focused proofs still only cover identity/count
(`TestGameRuntimeSafeboxItemMoveWhileOpenRelocatesWholeStack`). Without an
explicit freeze + clone + proof, a later RED could keep pointer-sharing relative
to the pre-move snapshot / helper input while docs claim presence round-trips
honestly through relocate `SAFEBOX_SET` and reopen/durable rematerialize.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary warehouse
relocate path — the last open safebox presence twin after check-in / checkout /
partial-split / destination-wins merge. Opening RED without freezing:

- that only whole-stack empty-destination relocate is in scope (not compatible
  merge, already destination-wins; not partial-split, already independent),
- that destination must clone rather than share the pre-move source pointers,
- how omitted vs explicit-zero presence behaves after relocate,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred. Do not reopen destination-wins merge or partial-split
contracts.

## Contract to freeze (before RED)

1. **Whole-stack empty-destination relocate**: when accepted `SAFEBOX_ITEM_MOVE`
   with `count = 0` or `count == source_count` places a remembered safebox stack
   into an empty in-range safebox cell, the destination `ItemInstance` must:
   - keep the source item identity and count;
   - update only the destination slot metadata;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the destination
       (including explicit `{0,0,0}`) so later writes cannot alias the pre-move
       source snapshot / helper input;
     - if the source `HasAttributes()`, clone those attributes onto the
       destination (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the destination likewise
       omits them (later encode / rematerialize keeps template fallback).
2. **Source of truth**: the live open-presentation safebox source cell already
   validated for the move; source map entry is deleted as already owned.
3. **Wire honesty**: relocate-burst `SAFEBOX_SET` must project presence-aware
   instance sockets/attributes via ordinary `EffectiveSockets` /
   `EffectiveAttributes`.
4. **Persistence**: the durable same-account safebox FileStore cell (when
   durable sync is enabled) and same-session reopen `SAFEBOX_SET` must
   round-trip independent presence-aware fields (including explicit zero).
5. **Non-goals**: compatible merge (already destination-wins count-only),
   partial-split (already independent clone), refine catalysts / mall / party
   ownership notices, inventing merge-on-stack rules, or changing exclusive
   ownership / despawn timers.

## Proof shape (RED → GREEN)

1. Helper/unit: `safeboxWholeStackRelocateItem` (or equivalent) clones presence
   independently for active / explicit-zero / omitted cases and keeps source
   identity + count with the new slot; mutating the clone leaves the seed
   pointer unchanged.
2. Session: seed a presence-bearing stack → `/open_safebox` → `SAFEBOX_CHECKIN`
   → whole-stack empty-destination `SAFEBOX_ITEM_MOVE` → relocate `SAFEBOX_SET`
   + reopen `SAFEBOX_SET` carry preserved presence; omitted stays omitted /
   template-fallback encode.
3. Negatives: compatible merge stays destination-wins as already owned;
   closed / occupied / out-of-range rejects stay non-mutating.

## Likely files to change (GREEN)

- `internal/minimal/factory.go` (whole-stack empty-destination relocate branch)
- `internal/minimal/safebox_partial_split_instance_clone_test.go` or focused
  whole-stack relocate helper twin
- `internal/minimal/item_storage_runtime_test.go` (session twin)
- `spec/protocol/item-storage-guard-bootstrap.md`
- `docs/qa/manual-client-checklist.md` (narrow honesty note)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/minimal -run 'SafeboxWholeStackRelocate|SafeboxItemMoveWholeStack.*Preserves|SafeboxItemMoveWhileOpenRelocatesWholeStack' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: whole-stack empty-destination `SAFEBOX_ITEM_MOVE`
relocates with source identity preserved and an independent presence-aware
sockets/attributes clone (including explicit zero; omit→omit) through relocate
`SAFEBOX_SET` and same-session reopen
(`TestSafeboxWholeStackRelocateItemClonesPresenceIndependently`,
`TestGameRuntimeSafeboxItemMoveWholeStackEmptyDestinationPreservesInstanceSocketsAndAttributes`).
Compatible merge stays destination-wins count-only. Partial-split independent
clone remains owned. Refine catalysts / mall / party ownership remain deferred.
