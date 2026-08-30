# Durable safebox cell instance attributes — 2026-08-30

## Objective

Freeze the next warehouse persistence-safety seam after durable ground-item
instance attributes: same-account safebox FileStore cells must round-trip the
same presence-aware instance attributes (including explicit all-zero /
type-zero) so check-in → `gamed` restart → reopen / checkout does not silently
fall back to template attributes.

## Why this exists

- Live in-memory check-in already stores `inventory.ItemInstance` (including
  `Attributes`) in the open-presentation map, and `SAFEBOX_SET` already prefers
  `EffectiveAttributes` over template attributes.
- Durable `safeboxstore.Cell` / `CharacterCells` / `ReplaceCharacterCells` still
  serialize only sockets beside `cell` / `id` / `vnum` / `count` / `locked`.
  Restore rebuilds `ItemInstance{ID,Vnum,Count,Slot,Locked,Sockets?}` with
  `Attributes: nil`.
- That drops authoritative instance attributes across restart even though
  carried FileStore, live encode preference, and ground-item FileStore already
  own the same shape.
- Priority-queue #1 (use/equip/sell/storage persistence) is the honest
  follow-on after ground-item attributes; tip-`0015` SQL additive stays deferred.

## Contract to freeze (before RED)

1. Extend `safeboxstore.Cell` with presence-aware optional attributes that mirror
   carried FileStore / ground-item FileStore:
   - omit / `has_attributes=false` → nil instance attributes (template fallback)
   - `has_attributes=true` including all-zero / type-zero → authoritative
     `Attributes`
2. `ReplaceCharacterCells` / `cellFromItemInstance` must persist instance
   attributes from the live presentation map.
3. `CharacterCells` / restore helpers must rebuild the same
   `ItemInstance.Attributes` pointer semantics before open-burst `SAFEBOX_SET` /
   checkout / item-move.
4. Quarantine / normalize reject non-zero attribute values when
   `has_attributes` is false.
5. Deterministic JSON omits `has_attributes` when false and omits empty
   attributes via `omitempty` (explicit-zero rows still write
   `"has_attributes": true` plus the zero attribute array).
6. Migration-shaped tip-`0015` safebox export identity stays tip `15` unless a
   later slice deliberately adds an additive SQL companion; this freeze is
   FileStore rematerialize first.
7. Do **not** invent mall, client `SAFEBOX_CHANGE_PASSWORD` packets, TMP4 CG
   `SAFEBOX_MONEY`, tip-`0010`/`0015` attribute SQL, or refine catalysts.

## Explicit non-goals

- tip-`0015` SQL additive companion / export row attributes
- mall / GD/DB myshop / quest-running MYSHOP open
- changing `GC::SAFEBOX_SET` wire layout (already carries attributes)
- attribute gameplay / apply formulas / combat recomputation

## Proof shape (after this freeze)

1. FileStore unit: durable snapshot round-trips presence-aware attributes
   including explicit all-zero / type-zero; non-zero attributes without
   `has_attributes` fail closed.
2. Runtime restart: check-in an instance-attributed item → restart `gamed` →
   `/open_safebox` rematerializes `SAFEBOX_SET` with the same attributes →
   checkout restores them into carried inventory (not template fallback).
3. Spec/QA/roadmap name the durable safebox cell attribute rematerialize;
   tip-`0015` SQL additive stays deferred.

## Proposed RED tests (do not open until this freeze lands)

- `TestFileStoreRoundTripPersistsInstanceAttributesIncludingExplicitZero`
- `TestFileStoreRejectsNonZeroAttributesWithoutHasAttributes`
- `TestGameRuntimeSafeboxCheckinInstanceAttributesRematerializeAcrossDaemonRestart`

## Status

Docs/spec freeze on `lane/items` after ground-item attribute rematerialize GREEN.
Production durable-cell fields / rematerialize wiring stay deferred until the
next GREEN run.
