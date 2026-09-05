# ITEM_PICKUP free-cell presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after partial `ITEM_USE` remainder
presence preserve landed: accepted free-cell `ITEM_PICKUP` (including
owner restore of a self-dropped whole stack into the original carried
slot, or the lowest free cell when that slot is occupied) must project
**presence-aware** sockets/attributes (`EffectiveSockets` /
`EffectiveAttributes`) on the placement `ITEM_SET` (including explicit
zero; omit→omit with template encode fallback), and the placed
`ItemInstance` must be an **independent** clone of the pending ground
snapshot so later inventory writes cannot alias that ground handle.

Today `PickupGroundItem` already places through `WithInventorySlot`
(`CloneSockets` / `CloneAttributes`), encode already prefers instance
presence, and a helper-only free-cell clone proof exists for the active
case. Protocol/QA still describe free-cell pickup as a bare `ITEM_SET`
and still talk about template-authored display on the split-fill path.
Without an explicit freeze + session proof, a later RED could copy
template display onto the restored cell or reopen destination-wins merge
as if free-cell pickup were still identity-only.

## Why docs-first

This is priority-queue #1 (reward/drop pickup safety + item-state
consistency) on the ordinary PvE drop → pickup restore path. Opening RED
without freezing:

- that only free-cell / original-slot restore and split-remainder
  placement are in scope (not compatible-stack merge, already
  destination-wins),
- that instance presence (incl. explicit zero) wins over template on the
  placement `ITEM_SET`,
- that omitted presence stays omitted (later encode may use template
  fallback),
- that the placed cell must not share sockets/attributes pointers with
  the pending ground snapshot,

would leave the stale template-only wording as the contract. Keep refine
catalysts, mall, and party ownership notices deferred. Do not reopen
whole-stack ground preserve, partial-drop remainder, use remainder, or
pickup destination-wins merge contracts.

## Contract to freeze (before RED)

1. **Free-cell placement**: when accepted `ITEM_PICKUP` /
   `PickupGroundItem` cannot merge the whole ground count into a
   compatible unlocked stack and instead occupies the original carried
   slot (when free) or the lowest free carried cell, the placed
   `ItemInstance` must:
   - keep the ground item identity and remaining count;
   - occupy that free cell;
   - carry an independent clone of the ground snapshot's presence-aware
     sockets and attributes:
     - if the ground item `HasSockets()`, clone those sockets onto the
       placed cell (including explicit `{0,0,0}`) so later inventory
       writes cannot alias the pending ground snapshot;
     - if the ground item `HasAttributes()`, clone those attributes onto
       the placed cell (including explicit all-zero / type-zero);
     - if the ground item omits sockets and/or attributes, the placed
       cell likewise omits them (later encode keeps template fallback).
2. **Split remainder**: when stackable pickup fills compatible partial
   stacks first and then places leftover count into a free cell, that
   remainder placement follows the same free-cell preserve contract. The
   filled stacks stay count-only destination-wins as already owned.
3. **Wire honesty**: placement `ITEM_SET` must project presence-aware
   instance sockets/attributes via ordinary `EffectiveSockets` /
   `EffectiveAttributes` (instance presence including explicit zero wins
   over template; omitted keeps template fallback).
4. **Persistence**: the selected-character account snapshot after
   successful free-cell pickup must round-trip independent presence-aware
   fields on the placed cell (including explicit zero) without copying
   template display metadata onto instance presence.
5. **Non-goals**: inventing new pickup reject/ownership policy, reopen
   destination-wins compatible-stack merge, reopen whole-stack ground
   preserve, gold pickup, refine catalysts / mall / party ownership
   notices, or changing inventory-full / anti-get / range rejects already
   owned.

## Proof shape (RED → GREEN)

1. Unit: seed a ground snapshot with authoritative instance presence
   (active / explicit-zero / omitted) → free-cell `PickupGroundItem` →
   placed identity/slot preserved; presence is an independent clone;
   mutating the placed cell leaves the ground snapshot pointer unchanged;
   omitted stays omitted. Split remainder placement uses the same clone
   contract while filled stacks stay destination-wins.
2. Session: whole-stack drop of a presence-bearing stack whose instance
   sockets/attrs differ from the loaded template → owner `ITEM_PICKUP`
   restore into the original free slot → placement `ITEM_SET` + account
   snapshot carry instance presence (not template); omitted stays omitted
   / template-fallback encode.
3. Negatives: compatible merge stays destination-wins as already owned;
   inventory-full / anti-get / dead-collector / range rejects stay
   non-mutating as already owned.

## Likely files to change (GREEN)

- `internal/player/compatible_stack_merge_destination_wins_test.go`
  (expand the helper-only free-cell proof)
- `internal/minimal/item_pickup_preserve_instance_presence_test.go` (new)
- `spec/protocol/item-drop-pickup-bootstrap.md`
- `spec/protocol/packet-matrix.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'PickupGroundItemFreeCellPreserves|PickupGroundItemSplitRemainderPreserves' -count=1
go test ./internal/minimal -run 'ItemPickupFreeCellPreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: accepted free-cell `ITEM_PICKUP` /
`PickupGroundItem` (including restore-self-drop to the original slot and
split-remainder free-cell placement) keeps ground identity and an
independent presence-aware sockets/attributes clone (including explicit
zero; omit→omit) through placement `ITEM_SET` + account snapshot
(`TestRuntimePickupGroundItemFreeCellPreservesSourceInstancePresence`,
`TestRuntimePickupGroundItemSplitRemainderPreservesSourceInstancePresence`,
`TestGameRuntimeItemPickupFreeCellPreservesInstanceSocketsAndAttributes`).
Compatible merge stays destination-wins count-only. Refine catalysts /
mall / party ownership remain deferred.
