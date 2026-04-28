# Inventory-equipment bootstrap

This document freezes the first minimal owned inventory/equipment contract for `go-metin2-server`.

The goal of this slice is narrow:
- define the first self-only M3 item-state surface before runtime and persistence code starts inventing ad hoc semantics
- reserve a deterministic slot-addressed bootstrap shape for carried items and worn equipment
- keep the scope small enough that later slices can add value objects, persistence, codecs, and mutations without rewriting the contract

It does **not** yet define the full compatibility-grade item system.

## Scope

This contract currently applies only to:
- the selected character that has just entered `GAME`
- self-only bootstrap state owned by that character
- carried inventory slots
- equipped item slots

This slice does not yet freeze peer-visible item state, storage/safebox surfaces, or transactional gameplay.

## Working flow

The current bootstrap burst after `ENTERGAME` is still:

1. `PHASE(GAME)`
2. `CHARACTER_ADD`
3. `CHAR_ADDITIONAL_INFO`
4. `CHARACTER_UPDATE`
5. `PLAYER_POINT_CHANGE`
6. trailing peer/static-actor visibility frames when needed

The first inventory/equipment extension reserves the next self-only slot in that flow:

1. `PHASE(GAME)`
2. `CHARACTER_ADD`
3. `CHAR_ADDITIONAL_INFO`
4. `CHARACTER_UPDATE`
5. `PLAYER_POINT_CHANGE`
6. zero or more self-only inventory/equipment item bootstrap frames
7. trailing peer/static-actor visibility frames

This document intentionally freezes the ordering and scope first.
The exact packet headers and byte layouts for those item bootstrap frames remain deferred until the dedicated `internal/proto/item` golden-test slice lands.

## Logical item snapshot shape

The first owned inventory/equipment surface must stay slot-addressed and deterministic.

Each occupied carried or equipped slot is expected to map to one owned item snapshot with the following minimum semantics:
- `slot` — stable inventory slot index for carried items
- `vnum` — item template identifier
- `count` — stack count
- `id` — stable instance identity for persistence/runtime ownership, even if the first client-visible packet family does not expose it directly
- `equipped` — whether the item is currently worn
- `equipment_slot` — the named worn slot when `equipped = true`

Later slices may extend the runtime or packet shape, but this first contract does **not** yet require:
- sockets
- attributes / applies
- timers / expiry
- ownership seals
- refine level or transmutation metadata
- quickslots
- drag-to-ground state

## Inventory slot surface

The first bootstrap inventory surface is intentionally narrow:
- it covers only the selected character's normal carried inventory
- it is addressed by deterministic numeric slot indices
- empty slots do not need dedicated bootstrap frames in the first slice; only occupied slots need representation
- later move/swap slices must preserve stable slot identity instead of treating inventory as an unordered bag

This contract deliberately avoids freezing mall/storage/safebox windows or any secondary inventory pages beyond the project-owned carried-inventory surface required for the first M3 loop.

## Equipment slot surface

The first bootstrap equipment surface freezes a small named worn-slot set that is sufficient for early equip/unequip and visible-part follow-up work:
- `body`
- `weapon`
- `head`
- `hair`
- `shield`
- `wrist`
- `shoes`
- `neck`
- `ear`
- `unique1`
- `unique2`
- `arrow`

Rules for this first stage:
- each equipment slot may contain at most one item instance
- equipped items remain part of the same owned character item state as carried inventory
- peer-visible appearance still continues to ride the existing `CHARACTER_ADD` / `CHARACTER_UPDATE` bootstrap contract until a later slice explicitly refreshes those links

## Persisted snapshot boundary

Before any live M3 mutation is allowed, the file-backed `accountstore` / `loginticket` character snapshot is also expected to carry explicit owned item-state fields alongside the existing character bootstrap data:
- `gold` — first explicit currency field for owned character state
- `inventory` — carried item instances
- `equipment` — equipped item instances

Backwards-compatibility rules for this persistence boundary:
- older JSON snapshots that lack these fields must still load successfully
- missing `inventory` / `equipment` arrays normalize to empty slices rather than malformed or ambiguous state
- zero `gold` remains explicit state instead of being hidden behind undocumented point indices

## First packet-family boundary

The packet matrix now reserves the first project-owned family names for this surface:
- `ITEM_SET` — self-only occupied-slot bootstrap/update surface for carried or equipped items
- `ITEM_DEL` — future self-only clear/remove surface for slot eviction, unequip, consume, or move follow-up

These rows are intentionally marked `planned` with `Header = TBD` until:
- the exact TMP4-compatible wire shape is frozen in `internal/proto/item`
- project-owned golden packet tests exist for the final byte layout

This docs-first slice therefore freezes **names, scope, ordering, and non-goals** before freezing the exact bytes.

## Explicit non-goals

This slice does **not** yet freeze:
- safebox or item mall windows
- drag-to-ground / drop-on-map behavior
- trade or exchange windows
- crafting or refining flows
- real merchant buy/sell transactions
- sell-back
- currency packet families
- equipment bonus formulas or combat-side stat recomputation

## Success definition

After this slice, the repository should be able to say:
- inventory/equipment are no longer undefined territory in project docs
- the first self-only bootstrap ordering for item state is frozen relative to `ENTERGAME`
- the repo owns a stable vocabulary for carried inventory slots, equipment slots, and minimum item snapshot semantics
- the packet matrix reserves the first item-family names without pretending the byte-level codec is already finished
