# Item-use bootstrap

This document freezes the first owned item-use contract for `go-metin2-server`.

The goal is intentionally narrow:
- define exactly one consumable-only vertical before broader gameplay scripting exists
- freeze one tiny client-originated item-use ingress without pretending the full legacy item-use family is already frozen
- make the first point-changing item path explicit enough that RED tests can pin it down and the minimal runtime can implement it without widening scope

It does **not** yet define the full legacy item-use surface.

The authored item-template snapshot boundary that feeds this runtime path is documented separately in `item-template-store-bootstrap.md`. In particular, malformed snapshots and snapshots with unknown JSON fields fail closed rather than booting while silently ignoring unowned item metadata.

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
- the first owned client-originated item-use request packet is now `ITEM_USE` with framed header `0x0501`
- `ITEM_USE` currently carries only one packed `TItemPos` payload: `window_type:uint8`, `cell:uint16` (little-endian)
- only carried inventory slots are valid live runtime inputs (`cell < 90`)
- the player mutation boundary also rejects slots outside that carried-inventory range before template effects are applied, so equipment/extended cells cannot mutate points or stacks even if a stale or direct runtime caller has an item snapshot there
- equipped items remain out of scope even though equipment still uses the legacy combined inventory/equipment cell namespace elsewhere in the bootstrap item family

The client source also exposes a separate drag-to-item packet family, `ITEM_USE_TO_ITEM`:
- framed header `0x0506`
- payload is exactly two packed `TItemPos` values: `source_pos` then `target_pos`
- each packed `TItemPos` remains `window_type:uint8`, `cell:uint16` little-endian

The first owned live `ITEM_USE_TO_ITEM` use case is intentionally only stack-on-stack consolidation:
- source and target must both be carried inventory positions (`cell < 90`)
- source and target must be different occupied slots with the same `vnum` and different item instance IDs
- empty source, empty target, and same-cell source/target requests fail closed before any ordinary use-effect fallback
- the minimal session/runtime packet path freezes same-cell `ITEM_USE_TO_ITEM` as a no-frame/no-mutation rejection, including unchanged persisted inventory and quickslots
- the source template must resolve to a valid stackable carried-item template with non-zero `max_count`
- the minimal session/runtime packet path now freezes missing source-template metadata as a no-frame/no-mutation rejection for drag-to-item stack consolidation, leaving persisted inventory and quickslots unchanged even when the live source/target stacks otherwise match
- the minimal session/runtime packet path now freezes equippable templates with an authored `equip_slot` as no-frame/no-mutation rejections for drag-to-item stack consolidation, matching the existing player mutation boundary guard
- the resolved source template `vnum` must match the live source stack `vnum`; a mismatched template is treated like unresolved/malformed metadata and fails closed
- the template-authored `max_count` must fit the currently owned one-byte item refresh count range (`<= 255`) because `ITEM_SET` / `ITEM_UPDATE` expose count as `uint8` in this bootstrap packet family; the minimal session/runtime packet path now freezes an over-`uint8` runtime template max as a no-frame/no-mutation rejection even if such a malformed template is injected after store validation
- templates with authored `anti_stack = true`, `anti_drop = true`, `anti_give = true`, or `anti_sell = true` are rejected for drag-to-item stack consolidation even when the live stacks otherwise match
- templates with authored job/sex restrictions for the selected character (`anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_male`, or `anti_female`) are rejected for drag-to-item stack consolidation even when the live stacks otherwise match
- templates with authored `min_level` above the selected character's current persisted `level` are rejected for drag-to-item stack consolidation even when the live stacks otherwise match
- the live source stack must have non-zero count, must not already exceed the template-authored `max_count`, and must validate as a well-formed carried inventory item even when the merge will remove it entirely
- the target stack must have non-zero count and free capacity under that `max_count`; already-full targets fail closed without source, target, quickslot, or persisted-state mutation
- the runtime moves as many source items as fit into the target stack
- if the source stack fits completely, the response burst is `ITEM_DEL(source)`, `ITEM_SET(target)`, then zero or more `QUICKSLOT_DEL` frames for item quickslots referencing the removed source carried cell
- the minimal session/runtime packet path now freezes that full-merge burst and the persisted account snapshot: source stack removed, target stack refreshed with the merged count, item quickslots for the removed source cell deleted, non-item quickslots with the same byte slot preserved, and target item quickslots left stable
- selected-character job/sex anti-flag and min-level templates are frozen through the normal minimal session/runtime packet path as no-frame/no-mutation rejections, so these template-authored restrictions are enforced before any stack consolidation or normal `use_effect` fallback
- the min-level runtime coverage explicitly preserves both carried stacks and item quickslots unchanged when the source/target stack metadata would otherwise be merge-compatible
- only item quickslots are removed and persisted; skill/command quickslots that happen to carry the same byte slot value stay unchanged
- if the target has only partial room, the source slot is refreshed with its remainder and the target slot is refreshed at its template `max_count`
- the minimal session/runtime packet path now freezes that partial-merge burst and persisted account snapshot too: source and target counts are updated, both cells emit `ITEM_UPDATE`, and the source item quickslot is preserved because the source stack remains occupied
- count-only partial refreshes use the existing `ITEM_UPDATE` packet shape for both changed carried cells and do not rewrite source item quickslots
- the normal `use_effect` path is not executed for this drag-to-item request, even when the source item also has a consumable template

Incompatible targets, empty source/target slots, out-of-carried-range source or target cells, same-cell source/target requests, duplicate source/target item instance IDs, duplicate live occupancy of the source or target carried cell, equipped cells, locked source or target items, zero-count source or target stacks, non-stackable templates, equippable templates with an authored `equip_slot`, anti-stack/anti-drop/anti-give/anti-sell templates, selected-character job/sex/min-level restricted templates, missing templates, template/source-`vnum` mismatches, over-template-max source stacks, over-template-max target stacks, already-full targets, and templates whose `max_count` exceeds the current one-byte item refresh count range fail closed with no frames and no mutation.
The runtime also rejects empty source/target slots, source/target cells outside the carried inventory range, same-cell source/target requests, duplicate source/target item instance IDs, duplicate live occupancy of the source or target carried cell, zero-count source or target stacks, template/source-`vnum` mismatches, non-stackable templates, equippable templates with an authored `equip_slot`, authored `anti_stack = true`, `anti_drop = true`, `anti_give = true`, or `anti_sell = true` templates, authored job/sex/min-level restrictions for the selected character, over-template-max source or target stacks, and template `max_count` values above `255` at the player mutation boundary itself, so these guards do not depend only on the minimal session handler pre-check. The player mutation boundary applies drag-to-item consolidation against a cloned carried-inventory snapshot and only swaps it into live state after both the source-side and target-side updates validate, so late validation failures cannot leave a partially removed source or partially incremented target stack behind.
The minimal session/runtime harness also freezes those guards through the normal `ITEM_USE_TO_ITEM` packet path: duplicate source/target item instance IDs, duplicate live occupancy of the source or target carried cell, locked source or target stacks, missing and invalid source templates, non-stackable templates, equippable templates with an authored `equip_slot`, anti-stack/anti-drop/anti-give/anti-sell templates, and selected-character job/sex/min-level restricted templates leave inventory and quickslot snapshots unchanged even when source and target live stacks otherwise share the same `vnum`. Already-full targets, over-template-max source stacks, over-template-max target stacks, and over-`uint8` template `max_count` values are likewise frozen through both the player mutation boundary and minimal session packet path as no-frame/no-mutation rejections.
When no runtime handler is installed, the default game-flow handler still rejects the packet silently/fail-closed.

For the first owned packet ingress, the runtime only accepts:
- `window_type = INVENTORY`
- `cell < 90`

Any other `TItemPos` currently fails closed.

## First consumable prototype

Exactly one consumable shape is frozen here:
- supported source surface: carried inventory only
- the runtime resolves the consumed slot through file-backed item-template metadata keyed by that item's `vnum`
- only non-equippable templates with a valid `use_effect` payload are currently eligible for `/use_item <slot>` or `ITEM_USE`
- templates with an authored `equip_slot` are rejected for direct consumable use even if they also carry a valid `use_effect`, so equipment metadata cannot accidentally execute the consumable point-effect path
- templates with authored job/sex restrictions for the selected character (`anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_male`, or `anti_female`) are rejected before point effects or stack mutations are applied
- templates with authored `min_level` above the selected character's current persisted `level` are rejected before point effects or stack mutations are applied
- templates with authored transfer/trade guards (`anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`) are rejected for this direct consumable path too, so bound/untransferable bootstrap templates cannot be consumed through `ITEM_USE` while other item mutation paths treat them as restricted
- locked carried stacks are rejected before point effects or stack mutations are applied; the minimal session/runtime packet path now freezes locked `ITEM_USE` as no-frame/no-mutation behavior with inventory, quickslots, and point values unchanged
- duplicate live occupancy of the requested carried cell is rejected before point effects or stack mutations are applied; the player mutation boundary and minimal session/runtime packet path now freeze duplicate-slot `ITEM_USE` as no-frame/no-mutation behavior with inventory, quickslots, and point values unchanged
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

The minimal session/runtime packet path now freezes last-stack `ITEM_USE` with template-authored output: `PLAYER_POINT_CHANGE`, `ITEM_DEL`, item-only `QUICKSLOT_DEL`, then the template-authored `CHAT_TYPE_INFO` placeholder. The persisted account snapshot removes the consumed stack, persists the updated point value, and deletes only item quickslots that referenced the removed carried cell.

If the consumed stack reaches zero, any selected-character item quickslots referencing that carried inventory cell are removed and refreshed with self-only `QUICKSLOT_DEL` frames immediately after `ITEM_DEL` and before the placeholder info chat. Skill and command quickslots that carry the same byte value are not affected. This ordering is frozen for both the older `/use_item <slot>` chat seam and the owned `ITEM_USE` packet ingress.

The temporary self-facing effect placeholder is intentionally text-backed in this slice:
- one self-only `CHAT_TYPE_INFO`
- `vid = 0`
- deterministic message text from `template.use_effect.message`

For the current seeded bootstrap consumable template this still means `consume:27001:+50`.

This effect placeholder exists only because there is not yet an owned visual-effect packet family wired through the runtime.

## Failure rules

The first consumable path must fail closed when any of these are true:
- the slot is empty
- the slot's `vnum` does not resolve to a valid non-equippable item template with a valid `use_effect`
- the resolved template carries an authored `equip_slot`
- the carried live item snapshot is malformed under the bootstrap item-instance validation rules
- the carried live item is locked
- the requested carried cell has duplicate live item occupancy
- the carried live item stack count already exceeds the resolved template-authored `max_count`; the minimal session/runtime packet path freezes this as no-frame/no-mutation behavior with inventory, quickslots, and point values unchanged
- applying the template-authored `use_effect.point_delta` would overflow the current signed 32-bit point-value range exposed by the bootstrap `PLAYER_POINT_CHANGE` path; the minimal session/runtime packet path freezes this as no-frame/no-mutation behavior with inventory, quickslots, and point values unchanged
- the resolved template carries an authored job/sex anti flag for the selected character
- the resolved template carries an authored `min_level` above the selected character's current persisted `level`
- the resolved template carries an authored `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell` guard
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
- `ITEM_USE_TO_ITEM` may still return self-local item/quickslot refresh frames to that stale socket for a compatible stack consolidation request
- stale item-use mutations must not persist updated `points`, `inventory`, or `quickslots`
- stale drag-to-item consolidation must not replace the replacement live owner's exact-name loopback inventory snapshot
- the minimal session/runtime harness now freezes stale full-stack `ITEM_USE_TO_ITEM` after ownership reclaim as self-local only: the stale socket can still receive its own `ITEM_DEL`, `ITEM_SET`, and source item `QUICKSLOT_DEL` frames, while persisted inventory/quickslots and the replacement live owner's loopback inventory snapshot remain unchanged
- if that stale socket later closes, a fresh reconnect/bootstrap must still reload the authoritative persisted `points`/`inventory`/`quickslots` state rather than the stale socket's local divergence
- no peer-facing packets are emitted from that stale socket for these bootstrap item-use paths

This keeps the first item-use seams consistent with the current reconnect/reclaim ownership contract without widening them into final duplicate-session gameplay semantics.

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
