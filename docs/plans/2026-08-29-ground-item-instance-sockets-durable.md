# Durable ground-item instance sockets — 2026-08-29

## Objective

Freeze the next pickup/persistence-safety seam after tip-`0003` SQL import
owned presence-aware inventory/equipment sockets (`0024`): pending ground
FileStore rematerialize must round-trip the same authoritative instance
sockets (including explicit zero) so drop → `gamed` restart → pickup does not
silently fall back to template sockets.

## Why this exists

- Live in-memory drop → pickup already carries `inventory.ItemInstance.Sockets`
  on the temporary shared-world ground handle.
- Durable `DurableGroundItemRecord` / ground-item FileStore still serialize only
  `item_id` / `vnum` / `count` (+ ownership/timers). Restore builds
  `ItemInstance{ID,Vnum,Count}` with `Sockets: nil`.
- That drops deactivated auto-potion `socket0 = 0` (and any other presence-aware
  instance sockets) across restart, so post-restart pickup can rematerialize the
  wrong effective sockets even though FileStore inventory and tip-`0003`+`0024`
  SQL already own the carried shape.
- Priority-queue #1 (reward/drop pickup safety + persistence) is the honest
  follow-on; no new `ITEM_GROUND_ADD` wire fields are invented.

## Contract to freeze (before RED)

1. Extend `DurableGroundItemRecord` with presence-aware optional sockets that
   mirror the carried FileStore / tip-`0003`+`0024` rule:
   - omit / `has_sockets=false` → nil instance sockets (template fallback)
   - `has_sockets=true` including all-zero → authoritative `Sockets`
2. Drop / reward registration that already carries `ItemInstance.Sockets` must
   persist those sockets into the durable snapshot.
3. Rematerialize / restore must rebuild the same `ItemInstance.Sockets` pointer
   semantics before the handle is visible for pickup.
4. Gold markers stay socket-less (no invented gold sockets).
5. Quarantine / normalize reject non-zero sockets when `has_sockets` is false.
6. Migration-shaped tip-`0010` bootstrap ground-item export identity stays tip
   `10` unless a later slice deliberately adds an additive SQL companion; this
   freeze is FileStore rematerialize first.
7. Do **not** invent ground-add packet socket fields, party delivery, or mall.

## Explicit non-goals

- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- additive `0010` SQL companion (defer unless FileStore GREEN proves the need)
- safebox cell sockets (runner-up; needs Cell/FileStore freeze of its own)
- refine catalysts / mall / GD/DB myshop / quest-running MYSHOP open

## Proof shape (after this freeze)

1. FileStore / shared-world unit: durable snapshot round-trips presence-aware
   sockets including explicit zero.
2. Runtime restart: drop an instance-socketed item (e.g. deactivated auto-potion
   `socket0=0`) → restart `gamed` rematerialize → pickup restores the same
   sockets into carried inventory (not template fallback).
3. Spec/QA/roadmap name the durable socket rematerialize; tip-`0010` SQL
   additive stays deferred unless needed.

## Proposed RED tests (do not open until this freeze lands)

- `TestGameRuntimePendingGroundItemInstanceSocketsRematerializeAcrossDaemonRestart`
- optional helper:
  `TestSharedWorldDurableGroundItemSnapshotRoundTripsInstanceSocketsIncludingExplicitZero`

## Status

GREEN on `lane/items`: durable pending ground FileStore rematerialize
round-trips presence-aware instance sockets (`has_sockets` + `socket0/1/2`,
including explicit zero) through register → persist → restart → pickup.
Gold markers stay socket-less; tip-`0010` SQL additive and safebox cell sockets
stay deferred.
