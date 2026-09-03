# Compatible stack-merge instance sockets/attributes policy — 2026-09-03

## Objective

Freeze the next Track C honesty seam after partial `ITEM_DROP2` fresh ground
identity + clone landed: when a presence-aware source stack merges into an
already-occupied compatible unlocked destination stack, the merge stays
**count-only** and the **destination** presence-aware sockets/attributes remain
authoritative. Source instance presence is discarded by the merge; it is not
copied, OR-merged, overwritten onto the destination, or used to reject the
merge.

Today every owned compatible-merge path already behaves this way in code
(pickup merge, inventory `ITEM_MOVE` / `ITEM_USE_TO_ITEM` merge, safebox
checkout / partial merge, exchange finalize merge, MYSHOP guest-buy merge),
while whole-stack free-cell placements preserve source presence. The policy has
still been named only as "deferred" / "not asserted" in recent preserve slices.
Without an explicit freeze, a later RED could invent overwrite / reject / OR
rules mid-implementation and break the already-owned encode preference for
destination `EffectiveSockets` / `EffectiveAttributes` on count-only
`ITEM_UPDATE`.

## Why docs-first

This is priority-queue #1 (item-state consistency) across drop/pickup, trade,
shop, and storage. Opening RED without freezing:

- that destination presence wins on compatible merge,
- that source presence is discarded rather than merged/overwritten,
- that omitted destination presence stays omitted (later encode may fall back
  to template; that is already owned encode preference, not a merge mutation),
- which seams share this rule,

would invent policy mid-implementation. Keep refine catalysts and mall
deferred. Do not reopen whole-stack free-cell preserve contracts already owned
by exchange finalize, MYSHOP guest-buy, safebox checkout, and pickup free-cell
placement.

## Contract to freeze (before RED)

1. **Compatible unlocked same-`vnum` merge is count-only**: when a transfer /
   pickup / move / checkout path merges `count` into an already-occupied
   compatible unlocked destination stack under the resolved template
   `max_count`, only the destination `Count` changes. Destination `ID`,
   `Slot` / safebox cell, `Equipped` / `EquipSlot`, `Locked`, and presence
   pointers stay the destination's.
2. **Destination presence wins**:
   - if the destination `HasSockets()`, those sockets remain after the merge
     (including explicit `{0,0,0}`);
   - if the destination `HasAttributes()`, those attributes remain after the
     merge (including explicit all-zero / type-zero);
   - if the destination omits sockets and/or attributes, the merged cell still
     omits them (later `ITEM_UPDATE` / `SAFEBOX_SET` / browse encode may use
     template fallback through already-owned `EffectiveSockets` /
     `EffectiveAttributes`).
3. **Source presence is discarded on merge**: source instance sockets/attributes
   (including explicit zero) do not overwrite, OR-merge into, or reject against
   the destination. Whole-stack free-cell placements that preserve source
   presence stay out of this contract and remain as already owned.
4. **Shared seams**: the same destination-wins / count-only rule applies to:
   - ground `ITEM_PICKUP` compatible-stack merge (including after partial
     `ITEM_DROP2` fresh-identity ground handles);
   - carried `ITEM_MOVE` / `ITEM_USE_TO_ITEM` compatible merges;
   - open-presentation `SAFEBOX_CHECKOUT` compatible merge and
     `SAFEBOX_ITEM_MOVE` compatible partial/whole merge;
   - exchange mutual-accept finalize compatible-stack merge;
   - MYSHOP guest-buy compatible-stack merge.
5. **Wire / display**: count-only `ITEM_UPDATE` / safebox merge `SAFEBOX_SET`
   continue to encode destination `EffectiveSockets` / `EffectiveAttributes`
   (instance presence including explicit zero wins over template; omitted
   instance keeps template). Merge does not invent a new packet shape.
6. **Non-goals**: inventing source-overwrites-destination, attribute OR/AND
   merge, reject-on-presence-mismatch, refine catalysts, mall, changing
   whole-stack free-cell preserve paths, or changing exclusive-ownership /
   despawn / tip SQL companions.

## Proof shape (for later RED → GREEN)

1. Unit/session: seed a destination stack with authoritative instance presence
   (active + explicit-zero cases) and a same-`vnum` source/ground/trade/shop
   stack with different presence → compatible merge → destination count
   increases; destination presence unchanged and still independent; source
   presence discarded; omitted-destination regression keeps omit→template
   encode fallback.
2. Cross-seam twins: at least one pickup merge twin and one already-owned
   exchange finalize / MYSHOP / safebox merge twin asserting destination-wins
   rather than only "merge not asserted".
3. Negatives: whole-stack free-cell preserve stays source-preserving; locked
   compatible stacks stay skipped; over-`max_count` / anti-stack rejects stay
   non-mutating.

## Likely files to change (later GREEN, not this freeze)

- focused proofs under `internal/minimal` / `internal/player` (pickup merge +
  one trade/storage twin)
- `spec/protocol/item-drop-pickup-bootstrap.md`
- `spec/protocol/item-exchange-bootstrap.md` / storage / move / use notes as
  narrowly as needed
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/minimal ./internal/player -run 'StackMerge|CompatibleMerge.*Instance|Pickup.*Merge.*Instance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: compatible unlocked same-`vnum` stack-merge is proven
count-only with destination presence-aware sockets/attributes authoritative
(including explicit zero), source presence discarded, and omitted destination
presence kept omitted across pickup merge + safebox checkout / exchange finalize
helper twins. Whole-stack free-cell preserve stays source-preserving; locked /
over-`max_count` rejects stay non-mutating. Refine catalysts and mall remain
deferred.
