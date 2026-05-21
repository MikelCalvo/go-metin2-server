# Inventory-equipment bootstrap

This document freezes the first minimal owned inventory/equipment contract for `go-metin2-server`.

The goal of the current contract is still narrow:
- define the first self-only M3 item-state surface for carried inventory and worn equipment
- reserve a deterministic slot-addressed bootstrap shape for owned items
- freeze the first self-only mutation refresh semantics for inventory slot move/swap without pretending the final client drag/drop packet family already exists
- keep the scope small enough that later slices can add richer runtime, equip/use semantics, and final compatibility packet ingress without rewriting the contract

It does **not** yet define the full compatibility-grade item system.

## Scope

This contract currently applies only to:
- the selected character that has just entered `GAME`
- self-only bootstrap state owned by that character
- carried inventory slots
- equipped item slots

This slice does not yet freeze peer-visible item state, storage/safebox surfaces, or transactional gameplay.

## Working flow

The first owned inventory/equipment extension now occupies the next self-only slot in the live bootstrap burst after `ENTERGAME`:

1. `PHASE(GAME)`
2. `CHARACTER_ADD`
3. `CHAR_ADDITIONAL_INFO`
4. `CHARACTER_UPDATE`
5. `PLAYER_POINT_CHANGE`
6. zero or more self-only inventory/equipment item bootstrap frames
7. trailing peer/static-actor visibility frames

This document now also freezes the first byte-level owned item bootstrap contract.
The exact packet headers and layouts are owned by `internal/proto/item` and verified by project-owned golden tests:
- `ITEM_SET = 0x0511`
- `ITEM_DEL = 0x0510`

## Logical item snapshot shape

The first owned inventory/equipment surface must stay slot-addressed and deterministic.

Each occupied carried or equipped slot is expected to map to one owned item snapshot with the following minimum semantics:
- `slot` — stable inventory slot index for carried items
- `vnum` — item template identifier referencing the deterministic file-backed template catalog seam under `internal/itemstore`
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
- peer-visible appearance for equipped `body`, `weapon`, and `head` items is now frozen separately in `spec/protocol/equipment-appearance-bootstrap.md` for bootstrap/peer-visibility packet builders
- live equip/unequip appearance fanout still remains out of scope here

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

The packet matrix now documents the first project-owned family names for this surface:
- `ITEM_SET` — self-only occupied-slot bootstrap/update surface for carried or equipped items (`0x0511`)
- `ITEM_DEL` — self-only clear/remove surface for slot eviction, unequip, consume, or move follow-up (`0x0510`)

The exact wire layout is now frozen by `internal/proto/item` golden tests.

## First live mutation refresh boundary

After the bootstrap burst, the owned mutation surface remains intentionally bootstrap-scoped:
- ingress now includes the first carried-slot client-originated `ITEM_MOVE` packet for inventory move/swap; the older `/inventory_move <from> <to>` slash-command seam remains as operator/test bootstrap compatibility
- the first carried-slot client-originated `ITEM_USE` ingress lives separately in `item-use-bootstrap.md`
- the current supported seams are:
  - `ITEM_MOVE` (`0x0504`) for carried-slot move/swap
  - `/inventory_move <from> <to>` for carried-slot move/swap compatibility
  - `/equip_item <from> <equip_slot>` for carried -> worn transitions
  - `/unequip_item <equip_slot> <to>` for worn -> carried transitions
- carried inventory keeps using `window_type = INVENTORY (1)` with `0 <= cell < 90`
- worn equipment still refreshes through the legacy combined inventory namespace `window_type = INVENTORY (1), cell = 90 + wear_index`
- successful mutations by the authoritative selected-character session must persist the updated selected-character inventory/equipment snapshot before the runtime commits the new live state
- failed persistence must fail closed and leave the selected runtime on the pre-mutation snapshot

Refresh rules for a successful self-only mutation:
- move into an empty slot emits `ITEM_DEL(from)` then `ITEM_SET(to)`
- swap with an occupied slot emits `ITEM_SET(from)` for the item that moved back into the source slot, then `ITEM_SET(to)` for the item that moved into the destination slot
- equip emits `ITEM_DEL(inventory_from)` then `ITEM_SET(equipment_cell)` then, when the equipped template carries the current narrow `equip_effect` metadata on that same authored `equip_slot`, one self-only `PLAYER_POINT_CHANGE`, then one self-only `CHARACTER_UPDATE`
- unequip emits `ITEM_DEL(equipment_cell)` then `ITEM_SET(inventory_to)` then, when the unequipped template carries the current narrow `equip_effect` metadata on that same authored `equip_slot`, one self-only `PLAYER_POINT_CHANGE`, then one self-only `CHARACTER_UPDATE`
- the current self-only equip/unequip `CHARACTER_UPDATE` reuses the appearance projection frozen in `spec/protocol/equipment-appearance-bootstrap.md`
- the first equip-driven point refresh is still intentionally narrow: it is self-only, template-authored, and limited to the selected session's runtime/persisted point snapshot; peer-visible point fanout and bootstrap recomputation from already-worn bonus items remain out of scope
- the direct item-slot response stays self-only; when the mutating character is already registered in shared-world visibility, already-visible stable peers now also receive one queued `CHARACTER_UPDATE` reusing the same projected appearance
- if a stale old socket has already lost live shared-world ownership because another session reclaimed that character, later `/equip_item` / `/unequip_item` may still return those self-local frames but must not persist carried/equipped state, must not queue peer-visible appearance refreshes, and must not overwrite the replacement live owner's exact-name loopback inventory/equipment snapshots

## Frozen wire position addressing

The first item family uses a packed legacy-compatible `TItemPos` equivalent:
- `window_type` — `uint8`
- `cell` — little-endian `uint16`

The first client-originated carried-slot drag/drop ingress is now frozen as `ITEM_MOVE`:
- header `0x0504`
- total frame length `11`
- payload order:
  1. source packed `TItemPos`
  2. destination packed `TItemPos`
  3. `count uint8`
- for the current bootstrap runtime, both source and destination must be normal carried inventory positions (`window_type = INVENTORY`, `0 <= cell < 90`)
- the accepted runtime path reuses the same selected-character full-stack move/swap semantics and `ITEM_DEL` / `ITEM_SET` refresh frames already owned by `/inventory_move`
- `count` is now honored as an exact full-stack guard for packet ingress: the packet is accepted only when `count` matches the authoritative stack count currently occupying the source slot
- partial stack splitting remains future work and fails closed instead of silently moving the entire source stack

For the current owned bootstrap surface:
- carried inventory uses `window_type = INVENTORY (1)` with `0 <= cell < 90`
- equipped items also travel with `window_type = INVENTORY (1)` and the legacy combined item cell namespace `cell = 90 + wear_index`
- the wire does **not** expose the project's named equipment-slot enum directly; it reuses the legacy wear indices in the combined inventory/equipment cell space
- examples: `body -> cell 90`, `weapon -> cell 94`, `shield -> cell 100`, `hair -> cell 110` (`WEAR_COSTUME_HAIR`)

## Frozen frame shapes

`ITEM_DEL` frame:
- header `0x0510`
- total frame length `7`
- payload: packed `TItemPos`

`ITEM_SET` frame:
- header `0x0511`
- total frame length `54`
- payload order:
  1. packed `TItemPos`
  2. `vnum uint32`
  3. `count uint8`
  4. `flags uint32`
  5. `anti_flags uint32`
  6. `highlight uint8`
  7. `sockets [3]int32`
  8. `attributes [7]` of `{type uint8, value int16}`

Although the first logical item snapshot contract still does not assign gameplay meaning to sockets, attributes, or most legacy flag bits yet, the first codec carries them as opaque compatibility fields so later slices do not need to redraw the frame boundary.

This docs-first contract therefore now freezes **names, ordering, scope, byte layout, and non-goals** for the first owned item bootstrap family.

## Explicit non-goals

This slice does **not** yet freeze:
- safebox or item mall windows
- drag-to-ground / drop-on-map behavior
- trade or exchange windows
- crafting or refining flows
- merchant sell-back or richer shop-window acknowledgement choreography
- currency packet families
- broader equipment bonus formulas or combat-side stat recomputation beyond the first narrow template-backed equip/unequip point delta

## Success definition

After this slice, the repository should be able to say:
- inventory/equipment are no longer undefined territory in project docs
- the first self-only bootstrap ordering for item state is frozen relative to `ENTERGAME`
- the loading-to-game burst now emits owned `ITEM_SET` frames for occupied carried/equipped slots immediately after `PLAYER_POINT_CHANGE`
- the first carried/worn mutation loops now persist selected-character move/swap/equip/unequip changes; carried-slot move/swap is accepted through both `ITEM_MOVE` and the older `/inventory_move` bootstrap seam, and refreshes the client with deterministic self-only `ITEM_DEL` / `ITEM_SET` frames, plus one self-only template-backed `PLAYER_POINT_CHANGE` when matched equip templates carry the current narrow `equip_effect`, one self-only `CHARACTER_UPDATE` after successful equip/unequip appearance changes, and one queued peer-visible `CHARACTER_UPDATE` for already-visible stable watchers in shared world
- the repo owns a stable vocabulary for carried inventory slots, equipment slots, minimum item snapshot semantics, and the first self-only mutation refresh rules
- the packet matrix and `internal/proto/item` codec now agree on the first byte-level item bootstrap family
