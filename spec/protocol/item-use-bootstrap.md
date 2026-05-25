# Item-use bootstrap

This document freezes the first owned item-use contract for `go-metin2-server`.

The goal is intentionally narrow:
- define exactly one consumable-only vertical before broader gameplay scripting exists
- freeze one tiny client-originated item-use ingress without pretending the full legacy item-use family is already frozen
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

The first owned item-use ingress remains bootstrap-scoped and deliberately narrow:
- the older bootstrap slash seam `/use_item <slot>` through the existing `CHAT(TALKING)` command path still remains valid
- the first owned client-originated item-use request packet is now `ITEM_USE` with framed header `0x0502`
- `ITEM_USE` currently carries only one packed `TItemPos` payload: `window_type:uint8`, `cell:uint16` (little-endian)
- only carried inventory slots are valid live runtime inputs
- equipped items remain out of scope even though equipment still uses the legacy combined inventory/equipment cell namespace elsewhere in the bootstrap item family

The client source also exposes a separate drag-to-item packet family, `ITEM_USE_TO_ITEM`:
- framed header `0x0506`
- payload is exactly two packed `TItemPos` values: `source_pos` then `target_pos`
- each packed `TItemPos` remains `window_type:uint8`, `cell:uint16` little-endian

The first owned live `ITEM_USE_TO_ITEM` use case is intentionally only stack-on-stack consolidation:
- source and target must both be carried inventory positions
- source and target must be different occupied slots with the same `vnum`
- the source template must resolve to a valid stackable item with non-zero `max_count`
- the live source stack must have non-zero count and must not already exceed the template-authored `max_count`
- the target stack must have free capacity under that `max_count`
- the runtime moves as many source items as fit into the target stack
- if the source stack fits completely, the response burst is `ITEM_DEL(source)`, `ITEM_SET(target)`, then zero or more `QUICKSLOT_DEL` frames for item quickslots referencing the removed source carried cell
- only item quickslots are removed; skill/command quickslots that happen to carry the same byte slot value stay unchanged
- if the target has only partial room, the source slot is refreshed with its remainder and the target slot is refreshed at its template `max_count`
- count-only partial refreshes use the existing `ITEM_UPDATE` packet shape for both changed carried cells and do not rewrite source item quickslots
- the normal `use_effect` path is not executed for this drag-to-item request, even when the source item also has a consumable template

Incompatible targets, empty slots, equipped cells, locked source or target items, non-stackable templates, missing templates, over-template-max source stacks, over-template-max target stacks, and already-full targets fail closed with no frames and no mutation.
The runtime also rejects non-stackable templates and over-template-max source or target stacks at the player mutation boundary itself, so these guards do not depend only on the minimal session handler pre-check.
When no runtime handler is installed, the default game-flow handler still rejects the packet silently/fail-closed.

For the first owned packet ingress, the runtime only accepts:
- `window_type = INVENTORY`
- `cell < 90`

Any other `TItemPos` currently fails closed.

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

When `/use_item <slot>` or `ITEM_USE(TItemPos{window = INVENTORY, cell = slot})` targets a carried inventory slot whose item resolves to a valid template-backed `use_effect` with `count >= 1`:

1. the runtime decrements the stack by exactly `1` while preserving the carried-stack bounds frozen in `item-stack-bootstrap.md`
2. the selected character's live `Points[point_index]` increases by exactly `point_delta`
3. the updated selected-character snapshot must be persisted before the new live state is committed
4. the server emits a deterministic self-only response burst in this order:
   1. `PLAYER_POINT_CHANGE`
   2. item refresh for that same carried slot
   3. zero or more `QUICKSLOT_DEL` frames for item quickslots that referenced the removed carried slot, only when the stack reaches zero
   4. one self-only `CHAT_TYPE_INFO` delivery acting as the temporary effect placeholder

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

If the consumed stack reaches zero, any selected-character item quickslots referencing that carried inventory cell are removed and refreshed with self-only `QUICKSLOT_DEL` frames. Skill and command quickslots that carry the same byte value are not affected.

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
- the request uses any `TItemPos` outside the current carried-inventory-only subset
- frame construction fails
- snapshot persistence fails

Failure behavior in this bootstrap slice:
- no partial live mutation may remain committed
- the selected runtime must roll back to the pre-use snapshot
- no peer-facing packets are emitted

## Persistence boundary

The first consumable path extends the existing M3 selected-character save-back boundary:
- `inventory` must reflect the decremented or removed stack
- `quickslots` must reflect any item quickslot deletion caused by a last-stack removal
- `points` must reflect the updated `Points[template.use_effect.point_index]` (currently `Points[1]` for the seeded bootstrap consumable)
- the save/commit path remains atomic from the perspective of the selected runtime

This slice still does **not** introduce separate buff-state stores, quest-state stores, or cooldown persistence.

## Stale post-reclaim isolation

If a socket already lost live shared-world ownership because another session reclaimed the same selected character:
- `/use_item <slot>` or `ITEM_USE` may still return the same self-local point/item/info burst to that stale socket
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
- runtime `ITEM_USE_TO_ITEM` effects beyond the first carried stack-on-stack consolidation case
- area effects or peer-visible FX
- general-purpose multi-effect template execution beyond the first point-change shape
- heal-over-time, poison, or buff stacking rules
- broader legacy item-use packet subfamilies beyond this first carried-slot request

## Success definition

With the first implementation slice landed, the repository can now say:
- the first owned item-use vertical is no longer undefined
- exactly one template-backed consumable shape is frozen and implemented before broader gameplay scripting begins
- `/use_item <slot>` and the first owned client-originated `ITEM_USE` packet now both mutate the first carried template-backed consumable in the bootstrap minimal runtime
- the self-only consumable outputs are explicit and exercised: `PLAYER_POINT_CHANGE`, `ITEM_SET`/`ITEM_DEL`, last-stack `QUICKSLOT_DEL`, and one `CHAT_TYPE_INFO` placeholder effect
- the first live `ITEM_USE_TO_ITEM` runtime case now reuses the carried-stack merge path for compatible same-`vnum` inventory stacks, persists the merged inventory, and deliberately avoids falling back to the normal consumable `use_effect`
- the selected-character writeback still preserves the existing atomic persistence and rollback boundary
