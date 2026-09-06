# SAFEBOX_ITEM_MOVE partial-merge remainder presence preserve — 2026-09-06

## Objective

Freeze the next Track C honesty seam after carried `ITEM_MOVE` partial-merge
remainder clone landed: counted compatible partial `SAFEBOX_ITEM_MOVE` must
keep the source remainder's **presence-aware** sockets/attributes as an
**independent** clone (including explicit zero; omit→omit with template
encode fallback) so later writes cannot alias the pre-merge open-presentation
pointer.

The destination cell stays **destination-wins** as already owned
(`docs/plans/2026-09-03-compatible-stack-merge-destination-wins-instance-sockets-attributes.md`).
Empty-destination partial-split already clones the **new destination**
identity and keeps remainder pointers count-only
(`docs/plans/2026-09-03-safebox-item-move-partial-split-independent-instance-sockets-attributes.md`).
This slice only freezes source-remainder independence on the still-occupied
source cell after a compatible partial merge.

Today `HandleSafeboxItemMove` copies the live source cell by value
(`sourceRemainder := sourceItem`), decrements `Count`, and writes that same
pointer-bearing struct back into `activeSafeboxItems`. Encode already prefers
`EffectiveSockets` / `EffectiveAttributes`, destination-wins proofs already
own the merged cell, and empty-destination partial split already clones the
new destination identity. Partial-merge source remainder still aliases.

## Why docs-first

This is priority-queue #1 (item-state consistency) plus #4 (storage honesty)
on the ordinary warehouse drag-stack-onto-stack path. Opening RED without
freezing:

- that only partial compatible merge source remainder is in scope (not
  empty-destination split remainder pointers, not full-stack merge remove,
  not destination-wins),
- that the remainder keeps source identity/slot plus an independent clone
  of source presence (including explicit zero; omit→omit),
- that destination presence still wins on the merged cell,

would invent policy mid-implementation. Keep refine catalysts, mall, and
party ownership notices deferred. Do not reopen carried `ITEM_MOVE`
remainder clone, empty-destination split, or destination-wins contracts.

## Contract to freeze (before RED)

1. **Partial merge remainder**: when counted compatible `SAFEBOX_ITEM_MOVE`
   leaves `1..source_count-1` on the source cell because the destination
   only had partial room, the source remainder `ItemInstance` must:
   - keep the source item identity and safebox slot;
   - change only `Count`;
   - keep an independent clone of the source's presence-aware
     sockets/attributes (including explicit `{0,0,0}` / all-zero
     attributes; omit→omit).
2. **Destination**: stays destination-wins count-only as already owned.
3. **Wire honesty**: source remainder `SAFEBOX_SET` encodes source presence
   via ordinary `EffectiveSockets` / `EffectiveAttributes`; destination
   `SAFEBOX_SET` encodes destination presence.
4. **Persistence**: the durable same-account safebox FileStore cell after
   successful partial merge must round-trip independent presence-aware
   fields on the remainder (including explicit zero) without copying
   destination/template arrays onto source presence. Same-session reopen
   `SAFEBOX_SET` follows the same remainder/destination split.
5. **Non-goals**: empty-destination split (already cloned onto a fresh
   identity; remainder keeps existing pointers by that contract),
   full-stack merge source remove, destination-wins policy, refine
   catalysts / mall / party ownership notices, or changing locked /
   anti-stack / over-count / closed-presentation rejects already owned.

## Proof shape (RED → GREEN)

1. Helper/unit: seed a multi-count safebox source with authoritative
   instance presence (active / explicit-zero / omitted) →
   `safeboxPartialMergeRemainderItem` (or equivalent) → remainder is an
   independent clone of the pre-merge source pointer; omitted stays
   omitted; mutating remainder sockets/attributes must leave the seed
   unchanged.
2. Session: packet `SAFEBOX_ITEM_MOVE` partial merge of presence-bearing
   stacks whose instance sockets/attrs differ from the loaded template
   **and** from each other → merge-burst `SAFEBOX_SET` + reopen /
   FileStore rematerialize carry source remainder presence on the source
   cell (not destination/template) and destination presence on the target;
   omitted stays omitted / template-fallback encode.
3. Negatives: full-stack source remove, empty-destination split clone,
   closed / locked / over-count rejects stay already owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/minimal/factory.go` (`HandleSafeboxItemMove` compatible
  partial-merge remainder branch)
- `internal/minimal/safebox_partial_split_instance_clone_test.go` (helper twin)
- `internal/minimal/item_storage_runtime_test.go` or a focused session twin
- `spec/protocol/item-storage-guard-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal -run 'SafeboxPartialMergeRemainder|SafeboxItemMovePartialMergePreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: counted compatible partial `SAFEBOX_ITEM_MOVE`
keeps the source remainder as an independent clone of the pre-merge
open-presentation presence (including explicit zero; omit→omit with
template encode fallback) through remainder `SAFEBOX_SET`, durable
FileStore rematerialize, and same-session reopen, while the merged cell
stays destination-wins count-only
(`TestSafeboxPartialMergeRemainderItemClonesPresenceIndependently`,
`TestGameRuntimeSafeboxItemMovePartialMergePreservesInstanceSocketsAndAttributes`).
Empty-destination split remainder clone is now owned
(`docs/plans/2026-09-06-safebox-item-move-partial-split-remainder-preserve-instance-sockets-attributes.md`).
Full-stack source remove and destination-wins stay already owned. Carried
`ITEM_MOVE` split remainder pointers / refine catalysts / mall / party
ownership remain deferred.
