# SAFEBOX_ITEM_MOVE partial-split remainder presence preserve — 2026-09-06

## Objective

Freeze the next Track C honesty seam after compatible partial
`SAFEBOX_ITEM_MOVE` merge remainder clone landed: counted empty-destination
partial `SAFEBOX_ITEM_MOVE` must keep the source remainder's
**presence-aware** sockets/attributes as an **independent** clone
(including explicit zero; omit→omit with template encode fallback) so
later writes cannot alias the pre-split open-presentation pointer.

The destination cell stays the already-owned **fresh identity + independent
clone**
(`docs/plans/2026-09-03-safebox-item-move-partial-split-independent-instance-sockets-attributes.md`).
Compatible partial-merge remainder clone stays already owned
(`docs/plans/2026-09-06-safebox-item-move-partial-merge-remainder-preserve-instance-sockets-attributes.md`).
This slice only freezes source-remainder independence on the still-occupied
source cell after an empty-destination partial split.

Today `HandleSafeboxItemMove` copies the live source cell by value
(`sourceRemainder = sourceItem`), decrements `Count`, and writes that same
pointer-bearing struct back into `activeSafeboxItems` on the empty-
destination branch. Encode already prefers `EffectiveSockets` /
`EffectiveAttributes`, destination split already clones onto a fresh
identity, and merge remainder already clones independently. Empty-destination
split source remainder still aliases.

The earlier split freeze explicitly kept remainder pointers as a non-goal
so only the new destination identity had to clone. That remainder-pointer
policy is now the honesty gap: a later write through the remainder cell
would still mutate the pre-split snapshot / destination clone seed if those
pointers stay shared. This freeze supersedes that remainder-pointer
non-goal without reopening destination-split clone, destination-wins merge,
or merge-remainder clone.

## Why docs-first

This is priority-queue #1 (item-state consistency) plus #4 (storage honesty)
on the ordinary warehouse drag-split path. Opening RED without freezing:

- that only empty-destination partial-split source remainder is in scope
  (not destination-split clone, not compatible merge remainder, not
  full-stack relocate),
- that the remainder keeps source identity/slot plus an independent clone
  of source presence (including explicit zero; omit→omit),
- that destination still gets a fresh identity plus its already-owned
  independent clone,

would invent policy mid-implementation. Keep refine catalysts, mall, and
party ownership notices deferred. Do not reopen destination-split clone,
merge-remainder clone, or destination-wins contracts.

## Contract to freeze (before RED)

1. **Partial split remainder**: when counted empty-destination
   `SAFEBOX_ITEM_MOVE` leaves `1..source_count-1` on the source cell, the
   source remainder `ItemInstance` must:
   - keep the source item identity and safebox slot;
   - change only `Count`;
   - keep an independent clone of the source's presence-aware
     sockets/attributes (including explicit `{0,0,0}` / all-zero
     attributes; omit→omit).
2. **Destination**: stays fresh identity + independent clone as already
   owned.
3. **Wire honesty**: source remainder `SAFEBOX_SET` encodes source presence
   via ordinary `EffectiveSockets` / `EffectiveAttributes`; destination
   `SAFEBOX_SET` encodes the cloned split presence.
4. **Persistence**: the durable same-account safebox FileStore cell after
   successful partial split must round-trip independent presence-aware
   fields on the remainder (including explicit zero) without sharing
   destination/template arrays. Same-session reopen `SAFEBOX_SET` follows
   the same remainder/destination split.
5. **Non-goals**: destination-split clone (already independent), compatible
   partial-merge remainder clone (already independent), full-stack relocate
   (already identity-preserving clone), destination-wins merge, refine
   catalysts / mall / party ownership notices, or changing locked /
   anti-stack / over-count / closed-presentation rejects already owned.
   Carried `ITEM_MOVE` empty-destination split remainder pointers stay on
   their existing split contract until a later freeze names that twin.

## Proof shape (RED → GREEN)

1. Helper/unit: seed a multi-count safebox source with authoritative
   instance presence (active / explicit-zero / omitted) →
   `safeboxPartialSplitRemainderItem` (or equivalent) → remainder is an
   independent clone of the pre-split source pointer; omitted stays
   omitted; mutating remainder sockets/attributes must leave the seed
   unchanged.
2. Session: packet `SAFEBOX_ITEM_MOVE` partial empty-destination split of
   a presence-bearing stack whose instance sockets/attrs differ from the
   loaded template → split-burst `SAFEBOX_SET` + reopen / FileStore
   rematerialize carry independent source remainder presence on the source
   cell (not destination/template) and cloned split presence on the
   destination; omitted stays omitted / template-fallback encode.
3. Negatives: destination-split clone, compatible merge remainder clone,
   full-stack relocate clone, closed / locked / over-count rejects stay
   already owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/minimal/factory.go` (`HandleSafeboxItemMove` empty-destination
  partial-split remainder branch)
- `internal/minimal/safebox_partial_split_instance_clone_test.go` (helper twin)
- `internal/minimal/item_storage_runtime_test.go` or a focused session twin
- `spec/protocol/item-storage-guard-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'SafeboxPartialSplitRemainder|SafeboxItemMovePartialSplitPreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

Frozen on `lane/items` (docs/spec only): counted empty-destination
`SAFEBOX_ITEM_MOVE` source remainder must keep an independent clone of
pre-split presence (including explicit zero; omit→omit) while the
destination stays a fresh identity plus its already-owned independent
clone. Production still aliases (`sourceRemainder = sourceItem`). Focused
remainder proofs stay the next GREEN twin. Do not claim the live split
remainder clone is owned until those proofs land. Carried `ITEM_MOVE`
split remainder pointers, refine catalysts / mall / party ownership remain
deferred.
