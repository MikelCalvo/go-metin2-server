# ITEM_DROP / ITEM_DROP2 whole-stack ground presence preserve — 2026-09-04

## Objective

Freeze the next Track C honesty seam after safebox checkout free-cell
preserve landed: whole-stack `ITEM_DROP` / `ITEM_DROP2` (including zero /
oversized DROP2 normalized to the full stack) must move the existing source
item identity onto the registered ground handle while carrying an
**independent** presence-aware sockets/attributes clone (including explicit
zero; omit→omit), matching the already-owned partial `ITEM_DROP2` clone
helper, pickup free-cell preserve, and `RegisterGroundItem` clone path.

Today production already clones through `RegisterGroundItem`, and the
partial-drop plan already names whole-stack as identity-preserving, but
focused proofs still only cover identity/count
(`TestGameRuntimeItemDrop2WholeStackKeepsSourceIdentity`). Without an
explicit freeze + proof, a later RED could drop whole-stack registration
back to pointer-sharing / bare `{ID,Vnum,Count}` while docs still claim
presence round-trips through durable ground rematerialize.

## Why docs-first

This is priority-queue #1 (reward/drop pickup safety + item-state
consistency) on the ordinary drop path. Opening RED without freezing:

- that only whole-stack / normalized-whole-stack drops are in scope (not
  partial fresh-ID),
- that destination ground presence must clone rather than share the
  pre-drop source pointers,
- how omitted vs explicit-zero presence behaves after registration,

would invent policy mid-implementation. Keep refine catalysts, mall, and
party ownership notices deferred. Do not reopen partial `ITEM_DROP2`
fresh-ID policy or destination-wins pickup merge.

## Contract to freeze (before RED)

1. **Whole-stack identity move + independent presence clone**: when accepted
   `ITEM_DROP` or whole-stack / normalized-whole-stack `ITEM_DROP2` removes a
   carried stack, the registered ground `ItemInstance` must:
   - keep the source item identity and dropped count;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the ground
       instance (including explicit `{0,0,0}`) so later ground writes cannot
       alias the pre-drop snapshot / helper input;
     - if the source `HasAttributes()`, clone those attributes onto the
       ground instance (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the ground instance
       likewise omits them (later encode / rematerialize keeps template
       fallback).
2. **Source of truth**: the pre-drop carried stack already validated for
   drop; inventory removal stays as already owned.
3. **Wire / durable honesty**: pending ground FileStore rows after the
   successful whole-stack drop must round-trip the preserved identity plus
   independent presence-aware fields (including explicit zero).
4. **Shared paths**: `ITEM_DROP` and whole-stack / zero-count / oversized
   `ITEM_DROP2` share the same identity-move + clone contract.
5. **Non-goals**: partial `ITEM_DROP2` fresh-ID policy (already owned),
   destination-wins pickup merge (already owned), gold drops, refine
   catalysts / mall / party ownership notices, inventing merge-on-stack
   rules, or changing exclusive-ownership / despawn timers.

## Proof shape (RED → GREEN)

1. Helper/unit: `droppedInventoryItem` (or equivalent whole-stack helper)
   clones presence independently for active / explicit-zero / omitted cases
   and keeps source identity + count; mutating the clone leaves the seed
   pointer unchanged.
2. Session: seed a presence-bearing whole stack → `ITEM_DROP` and
   whole-stack `ITEM_DROP2` → pending ground handle keeps source identity;
   ground presence is an independent clone; durable ground snapshot
   round-trips presence; omitted stays omitted.
3. Negatives: partial path stays fresh-ID + clone; gold drop stays
   socket-/attribute-less; anti-drop / floor rejects stay non-mutating.

## Likely files to change (GREEN)

- `internal/minimal/factory.go` (`droppedInventoryItem`)
- `internal/minimal/item_drop2_partial_fresh_ground_instance_test.go`
  (whole-stack presence twins beside the identity-only proof)
- `spec/protocol/item-drop-pickup-bootstrap.md`
- `docs/qa/manual-client-checklist.md` (narrow honesty note)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/minimal -run 'DroppedInventoryItem|ItemDrop.*WholeStackPreserves|ItemDrop2WholeStackPreserves|ItemDrop2WholeStackKeepsSourceIdentity' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: whole-stack / zero-count / oversized-normalized
`ITEM_DROP` / `ITEM_DROP2` keep the source item identity and prove independent
presence-aware sockets/attributes clones (including explicit zero; omit→omit)
through the pending ground handle and durable ground snapshot
(`TestDroppedInventoryItemClonesPresenceIndependently`,
`TestGameRuntimeItemDropWholeStackPreservesInstanceSocketsAndAttributes`,
`TestGameRuntimeItemDrop2WholeStackPreservesInstanceSocketsAndAttributes`).
Partial fresh-ID, pickup destination-wins merge, refine catalysts / mall /
party ownership remain deferred.
