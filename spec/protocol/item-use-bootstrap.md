# Item-use bootstrap

This document freezes the first owned item-use contract for `go-metin2-server`.

The goal is intentionally narrow:
- define exactly one consumable-only vertical before broader gameplay scripting exists
- keep ingress self-owned by the server runtime instead of pretending the final legacy item-use packet family is already frozen
- make the first point-changing item path explicit enough that RED tests can pin it down before implementation lands

It does **not** yet define the full legacy item-use surface.

## Scope

This first contract applies only to:
- the currently selected character already in `GAME`
- one carried-inventory consumable path
- self-only point/state change
- self-only item refresh
- self-only feedback/effect delivery

It does **not** yet apply to peers, world drops, merchants, or quest-scripted items.

## First owned ingress seam

The first owned item-use ingress remains bootstrap-scoped and deliberately server-owned:
- the client-facing bootstrap seam is `/use_item <slot>` through the existing `CHAT(TALKING)` command path
- there is no owned legacy `ITEM_USE` request packet yet in this slice
- only carried inventory slots are valid inputs
- equipped items are out of scope

This keeps the first vertical small while still freezing deterministic behavior.

## First consumable prototype

Exactly one consumable path is frozen here:
- supported `vnum`: `27001`
- supported source surface: carried inventory only
- fixed point target: `Points[1]`
- fixed point-change packet `type`: `1`
- fixed point delta per consume: `+50`

This is intentionally a prototype consumable, not the final item-template or quest-scripted system.

## Success path

When `/use_item <slot>` targets a carried inventory slot that holds `vnum = 27001` with `count >= 1`:

1. the runtime decrements the stack by exactly `1`
2. the selected character's live `Points[1]` increases by exactly `50`
3. the updated selected-character snapshot must be persisted before the new live state is committed
4. the server emits a deterministic self-only response burst in this order:
   1. `PLAYER_POINT_CHANGE`
   2. item refresh for that same carried slot
   3. one self-only `CHAT_TYPE_INFO` delivery acting as the temporary effect placeholder

## Frozen self-only refresh semantics

`PLAYER_POINT_CHANGE` for the first consumable path must use:
- `vid = selected character vid`
- `type = 1`
- `amount = 50`
- `value = updated Points[1]`

The item refresh for the consumed slot must use the existing owned item family:
- if the stack remains non-zero after consume, emit `ITEM_SET(slot)` with the decremented `count`
- if the consumed stack reaches zero, emit `ITEM_DEL(slot)`

The temporary self-facing effect placeholder is intentionally text-backed in this slice:
- one self-only `CHAT_TYPE_INFO`
- `vid = 0`
- deterministic message text: `consume:27001:+50`

This effect placeholder exists only because there is not yet an owned visual-effect packet family or item-template/name seam.

## Failure rules

The first consumable path must fail closed when any of these are true:
- the slot is empty
- the slot does not hold `vnum = 27001`
- the item is not in carried inventory
- frame construction fails
- snapshot persistence fails

Failure behavior in this bootstrap slice:
- no partial live mutation may remain committed
- the selected runtime must roll back to the pre-use snapshot
- no peer-facing packets are emitted

## Persistence boundary

The first consumable path extends the existing M3 selected-character save-back boundary:
- `inventory` must reflect the decremented or removed stack
- `points` must reflect the updated `Points[1]`
- the save/commit path remains atomic from the perspective of the selected runtime

This slice still does **not** introduce separate buff-state stores, quest-state stores, or cooldown persistence.

## Explicit non-goals

This first item-use bootstrap contract does **not** yet freeze:
- quest item scripting
- timed buffs
- equipment enchanting
- drag-to-world use semantics
- drag-and-drop target semantics
- area effects or peer-visible FX
- consumable name lookup from templates
- heal-over-time, poison, or buff stacking rules
- a final legacy client-originated item-use packet family

## Success definition

After this docs slice, the repository should be able to say:
- the first owned item-use vertical is no longer undefined
- exactly one consumable path is frozen before broader template/catalog work begins
- RED tests can target `/use_item <slot>` with `vnum = 27001`
- the expected self-only outputs are explicit: `PLAYER_POINT_CHANGE`, `ITEM_SET`/`ITEM_DEL`, and one `CHAT_TYPE_INFO` placeholder effect
- the atomic persistence and rollback expectations are explicit before implementation starts
