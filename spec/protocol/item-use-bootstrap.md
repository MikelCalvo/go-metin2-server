# Item-use bootstrap

This document freezes the first owned item-use contract for `go-metin2-server`.

The goal is intentionally narrow:
- define exactly one consumable-only vertical before broader gameplay scripting exists
- keep ingress self-owned by the server runtime instead of pretending the final legacy item-use packet family is already frozen
- make the first point-changing item path explicit enough that RED tests can pin it down and the minimal runtime can implement it without widening scope

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

Exactly one consumable shape is frozen here:
- supported source surface: carried inventory only
- the runtime resolves the consumed slot through file-backed item-template metadata keyed by that item's `vnum`
- only templates with a valid `use_effect` payload are currently eligible for `/use_item <slot>`
- that `use_effect` payload currently owns:
  - the point target index (`point_index`)
  - the point-change packet `type` (`point_type`)
  - the per-consume delta (`point_delta`)
  - the temporary self-only placeholder text (`message`)
- the current seeded bootstrap consumable template still uses `vnum = 27001`, `point_index = 1`, `point_type = 1`, `point_delta = 50`, and `message = consume:27001:+50`

This is intentionally the first template-backed consumable prototype, not the final quest-scripted or compatibility-grade item-use system.

The general carried-stack contract that this consumable now depends on lives in `item-stack-bootstrap.md`.
That companion document owns merge/new-slot/fail-closed placement semantics for template-driven carried items.

## Success path

When `/use_item <slot>` targets a carried inventory slot whose item resolves to a valid template-backed `use_effect` with `count >= 1`:

1. the runtime decrements the stack by exactly `1` while preserving the carried-stack bounds frozen in `item-stack-bootstrap.md`
2. the selected character's live `Points[point_index]` increases by exactly `point_delta`
3. the updated selected-character snapshot must be persisted before the new live state is committed
4. the server emits a deterministic self-only response burst in this order:
   1. `PLAYER_POINT_CHANGE`
   2. item refresh for that same carried slot
   3. one self-only `CHAT_TYPE_INFO` delivery acting as the temporary effect placeholder

## Frozen self-only refresh semantics

`PLAYER_POINT_CHANGE` for the first consumable path must use:
- `vid = selected character vid`
- `type = template.use_effect.point_type`
- `amount = template.use_effect.point_delta`
- `value = updated Points[template.use_effect.point_index]`

For the current seeded bootstrap consumable template this still means:
- `type = 1`
- `amount = 50`
- `value = updated Points[1]`

The item refresh for the consumed slot must use the existing owned item family:
- if the stack remains non-zero after consume, emit `ITEM_SET(slot)` with the decremented `count`
- if the consumed stack reaches zero, emit `ITEM_DEL(slot)`

The temporary self-facing effect placeholder is intentionally text-backed in this slice:
- one self-only `CHAT_TYPE_INFO`
- `vid = 0`
- deterministic message text from `template.use_effect.message`

For the current seeded bootstrap consumable template this still means `consume:27001:+50`.

This effect placeholder exists only because there is not yet an owned visual-effect packet family wired through the runtime.

## Failure rules

The first consumable path must fail closed when any of these are true:
- the slot is empty
- the slot's `vnum` does not resolve to a valid item template with a valid `use_effect`
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
- `points` must reflect the updated `Points[template.use_effect.point_index]` (currently `Points[1]` for the seeded bootstrap consumable)
- the save/commit path remains atomic from the perspective of the selected runtime

This slice still does **not** introduce separate buff-state stores, quest-state stores, or cooldown persistence.

## Stale post-reclaim isolation

If a socket already lost live shared-world ownership because another session reclaimed the same selected character:
- `/use_item <slot>` may still return the same self-local point/item/info burst to that stale socket
- that stale mutation must not persist updated `points` or `inventory`
- that stale mutation must not replace the replacement live owner's exact-name loopback inventory snapshot
- if that stale socket later closes, a fresh reconnect/bootstrap must still reload the authoritative persisted `points`/`inventory` state rather than the stale socket's local divergence
- no peer-facing packets are emitted from that stale socket for this bootstrap consumable path

This keeps the first item-use seam consistent with the current reconnect/reclaim ownership contract without widening it into final duplicate-session gameplay semantics.

## Explicit non-goals

This first item-use bootstrap contract does **not** yet freeze:
- quest item scripting
- timed buffs
- equipment enchanting
- drag-to-world use semantics
- drag-and-drop target semantics
- area effects or peer-visible FX
- general-purpose multi-effect template execution beyond the first point-change shape
- heal-over-time, poison, or buff stacking rules
- a final legacy client-originated item-use packet family

## Success definition

With the first implementation slice landed, the repository can now say:
- the first owned item-use vertical is no longer undefined
- exactly one template-backed consumable shape is frozen and implemented before broader gameplay scripting begins
- `/use_item <slot>` now mutates the first carried template-backed consumable in the bootstrap minimal runtime
- the self-only outputs are explicit and exercised: `PLAYER_POINT_CHANGE`, `ITEM_SET`/`ITEM_DEL`, and one `CHAT_TYPE_INFO` placeholder effect
- the selected-character writeback still preserves the existing atomic persistence and rollback boundary
