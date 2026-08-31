# Durable ground-item instance attributes — 2026-08-30

## Objective

Freeze then GREEN the next pickup/persistence-safety seam after account FileStore
+ live encode preference owns presence-aware instance attributes: pending ground
FileStore rematerialize must round-trip the same authoritative instance
attributes (including explicit all-zero / type-zero) so drop → `gamed` restart →
pickup does not silently fall back to template attributes.

## Why this exists

- Live in-memory drop → pickup already clones `inventory.ItemInstance.Attributes`.
- Account FileStore + `ITEM_SET` / `ITEM_UPDATE` / exchange `ITEM_ADD` already prefer
  `EffectiveAttributes`.
- Durable `DurableGroundItemRecord` / ground-item FileStore previously serialized only
  sockets beside `item_id` / `vnum` / `count` (+ ownership/timers). Restore built
  `ItemInstance{ID,Vnum,Count,Sockets?}` with `Attributes: nil`.
- That dropped authoritative instance attributes across restart even though live
  and carried FileStore already owned the shape.

## Contract owned by this slice

1. Extend `DurableGroundItemRecord` with presence-aware optional attributes that
   mirror the carried FileStore / sockets rule:
   - omit / `has_attributes=false` → nil instance attributes (template fallback)
   - `has_attributes=true` including all-zero / type-zero → authoritative
     `Attributes`
2. Drop / reward registration that already carries `ItemInstance.Attributes`
   persists those attributes into the durable snapshot.
3. Rematerialize / restore rebuilds the same `ItemInstance.Attributes`
   pointer semantics before the handle is visible for pickup.
4. Gold markers stay attribute-less (no invented gold attributes).
5. Quarantine / normalize reject non-zero attribute values when
   `has_attributes` is false.
6. Migration-shaped tip-`0010` export identity stays tip `10`; this slice is
   FileStore rematerialize only (tip-`0010` projection still omits attributes).
7. Do **not** invent ground-add packet attribute fields, party delivery, or mall.

## Explicit non-goals

- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- additive `0010` SQL companion (defer)
- safebox cell attributes (runner-up; needs Cell/FileStore freeze of its own)
- attribute gameplay / refine catalysts / mall / GD/DB myshop

## Proof shape

1. FileStore / shared-world unit: durable snapshot round-trips presence-aware
   attributes including explicit all-zero / type-zero.
2. Runtime restart: drop an instance-attributed item → restart `gamed`
   rematerialize → pickup restores the same attributes into carried inventory
   (not template fallback).
3. Spec/QA/roadmap name the durable attribute rematerialize; tip-`0010` SQL
   additive stays deferred.

## Tests

- `TestGroundItemFileStoreRoundTripPersistsInstanceAttributesIncludingExplicitZero`
- `TestGroundItemFileStoreRejectsNonZeroAttributesWithoutHasAttributesAndGoldAttributes`
- `TestSharedWorldDurableGroundItemSnapshotRoundTripsInstanceAttributesIncludingExplicitZero`
- `TestGameRuntimePendingGroundItemInstanceAttributesRematerializeAcrossDaemonRestart`

## Status

GREEN on `lane/items`: durable pending ground FileStore rematerialize
round-trips presence-aware instance attributes (`has_attributes` + `attributes`,
including explicit all-zero / type-zero) through register → persist → restart →
pickup. Gold markers stay attribute-less. tip-`0010` SQL attribute companion
(`0029`) is GREEN via
[bootstrap ground-item instance-attributes SQL
additive](2026-08-31-bootstrap-ground-item-instance-attributes-sql-additive.md);
seeded hermetic tip-`0010`+`0029` import-export drill is owned by
[seeded ground-item instance-attributes tip
sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md).
Safebox cell attributes FileStore + tip-`0015`+`0028` SQL + seeded tip sync are
owned separately.
