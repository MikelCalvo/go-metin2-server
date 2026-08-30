# Durable safebox cell instance sockets — 2026-08-30

## Objective

Freeze the next warehouse persistence-safety seam after durable ground-item
instance sockets: same-account safebox FileStore cells must round-trip the same
presence-aware instance sockets (including explicit zero) so check-in →
`gamed` restart → reopen / checkout does not silently fall back to template
sockets.

## Why this exists

- Live in-memory check-in already stores `inventory.ItemInstance` (including
  `Sockets`) in the open-presentation map, and `SAFEBOX_SET` already prefers
  `EffectiveSockets` over template sockets.
- Durable `safeboxstore.Cell` / `CharacterCells` / `ReplaceCharacterCells` still
  serialize only `cell` / `id` / `vnum` / `count` / `locked`. Restore rebuilds
  `ItemInstance{ID,Vnum,Count,Slot,Locked}` with `Sockets: nil`.
- That drops deactivated auto-potion `socket0 = 0` (and any other presence-aware
  instance sockets) across restart, so post-restart reopen `SAFEBOX_SET` and
  checkout can rematerialize the wrong effective sockets even though carried
  FileStore, tip-`0003`+`0024` SQL, and ground-item FileStore already own the
  same shape.
- Priority-queue #1 (use/equip/sell/storage persistence) is the honest
  follow-on after ground-item sockets; tip-`0015` SQL additive stays deferred.

## Contract to freeze (before RED)

1. Extend `safeboxstore.Cell` with presence-aware optional sockets that mirror
   carried FileStore / tip-`0003`+`0024` / ground-item FileStore:
   - omit / `has_sockets=false` → nil instance sockets (template fallback)
   - `has_sockets=true` including all-zero → authoritative `Sockets`
2. `ReplaceCharacterCells` must persist instance sockets from the live
   presentation map.
3. `CharacterCells` must rebuild the same `ItemInstance.Sockets` pointer
   semantics before open-burst `SAFEBOX_SET` / checkout / item-move.
4. Quarantine / normalize reject non-zero sockets when `has_sockets` is false.
5. Deterministic JSON omits `has_sockets` when false and omits zero socket
   fields via `omitempty` (explicit-zero rows still write `"has_sockets": true`).
6. Migration-shaped tip-`0015` safebox export identity stays tip `15` unless a
   later slice deliberately adds an additive SQL companion; this freeze is
   FileStore rematerialize first.
7. Do **not** invent mall, client `SAFEBOX_CHANGE_PASSWORD` packets, TMP4 CG
   `SAFEBOX_MONEY`, tip-`0010` SQL, attributes-on-instance, or refine catalysts.

## Explicit non-goals

- tip-`0015` SQL additive companion / export row sockets
- mall / GD/DB myshop / quest-running MYSHOP open
- changing `GC::SAFEBOX_SET` wire layout (already carries sockets)

## Proof shape (after this freeze)

1. FileStore unit: durable snapshot round-trips presence-aware sockets including
   explicit zero; non-zero sockets without `has_sockets` fail closed.
2. Runtime restart: check-in an instance-socketed item → restart `gamed` →
   `/open_safebox` rematerializes `SAFEBOX_SET` with the same sockets → checkout
   restores them into carried inventory (not template fallback).
3. Spec/QA/roadmap name the durable safebox cell socket rematerialize;
   tip-`0015` SQL additive stays deferred.

## Proposed RED tests

- `TestFileStoreRoundTripPersistsInstanceSocketsIncludingExplicitZero`
- `TestFileStoreRejectsNonZeroSocketsWithoutHasSockets`
- `TestGameRuntimeSafeboxCheckinInstanceSocketsRematerializeAcrossDaemonRestart`

## Status

Docs/spec freeze + GREEN on `lane/items`: durable safebox FileStore cells
round-trip presence-aware instance sockets (`has_sockets` + `socket0/1/2`,
including explicit zero) through check-in → persist → restart → reopen /
checkout. Tip-`0015` SQL additive stays deferred.
