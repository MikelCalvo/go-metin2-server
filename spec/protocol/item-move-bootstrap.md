# Item move bootstrap

This note freezes the current clean-room bootstrap contract for client-driven carried-inventory item movement on the game socket.

## Scope

Owned in this slice:

- `CG::ITEM_MOVE` is accepted only in `GAME` phase through the normal game flow dispatch.
- The request is limited to carried-inventory sources in the bootstrap runtime.
- Same-position carried-inventory move requests fail closed before runtime mutation, matching the legacy dupe-guard behavior.
- Full-stack moves into empty carried slots, full-stack swaps with incompatible occupied carried slots, counted partial splits, counted partial merges, and exact counted full-stack merges into a compatible destination use the same runtime inventory mutation path.
- A carried-inventory source may also be moved to an owned equipment-cell destination (`InventoryMaxCell + wear index`) to equip an item through the real `CG::ITEM_MOVE` packet path.
- Loaded item-template metadata is authoritative for stack compatibility: templates marked `anti_stack` or non-stackable are not merged by `CG::ITEM_MOVE`, even when source and destination share the same `vnum`; if an explicitly authored item-template snapshot is loaded, same-`vnum` occupied-destination merge attempts with missing source-template metadata fail closed instead of falling back to an unbounded merge.
- An owned equipment-cell source may be moved back to an empty carried-inventory destination to unequip through the real `CG::ITEM_MOVE` packet path.
- Successful mutations are self-only and refresh the touched inventory/equipment cells with existing item packets. When the source carried item lands in the destination cell — including exact full-stack compatible merges and incompatible occupied-slot swaps — the carried-to-carried runtime also appends the existing `GC::QUICKSLOT_ADD` refresh to retarget matching source item quickslots to the destination cell. If a stale item quickslot already points at the destination cell, the runtime first appends `GC::QUICKSLOT_DEL` for that destination binding so the moved source binding remains the only owned item quickslot for that cell. Partial stack moves/merges that leave the source item occupied keep source item quickslots unchanged. Equipping from a carried cell removes any item quickslot that referenced the now-equipped source cell.

Not owned yet:

- Safebox, mall, belt, dragon-soul, or ground movement.
- Counted partial move/swap semantics for incompatible occupied destinations beyond fail-closed rejection.
- Stack splitting/merging outside carried inventory.
- Cross-window drag/drop behavior.
- Server-side pickup/drop packets or item ownership on the ground.
- Any claim that the whole layout has been verified against the legacy source; current reference-source evidence has only been used to align the compatible occupied-destination stack merge rule where `count = 0` means use the whole source stack, capped by destination stack capacity, and the incompatible occupied-destination full-stack swap rule.

## Client request

`CG::ITEM_MOVE` uses header `0x0504`.

Payload size is 7 bytes:

| Offset | Size | Field | Encoding |
| --- | ---: | --- | --- |
| 0 | 1 | source window | `uint8` |
| 1 | 2 | source cell | little-endian `uint16` |
| 3 | 1 | destination window | `uint8` |
| 4 | 2 | destination cell | little-endian `uint16` |
| 6 | 1 | count | `uint8` |

The current carried-inventory window is `WindowInventory = 1`. Equipment cells are addressed in the same bootstrap window as `InventoryMaxCell + wear_index`; the currently owned wear-index mapping is the same mapping already used for equipment bootstrap: body `0`, head `1`, shoes `2`, wrist `3`, weapon `4`, neck `5`, ear `6`, unique1 `7`, unique2 `8`, arrow `9`, shield `10`, and costume hair `20`.

`count = 0` means a full-stack move. In the current owned packet path it succeeds when the destination carried slot is empty, swaps with an incompatible occupied carried slot, merges into a compatible stackable item that can accept at least one source item up to the source template's `max_count`, equips the source item into an empty owned equipment cell, or unequips an owned equipment-cell source into an empty carried slot. Non-zero `count` means a counted move bounded by the source item's template `max_count` when a valid template exists; when the count covers the whole source stack and the carried destination is incompatible, the runtime treats the request as the same full-stack carried swap already owned for `count = 0`. Counted equipment moves are not owned and fail closed without mutation.

Reference-oracle evidence: the legacy item-move entrypoint delegates to the character item-move routine, whose carried-item path has an explicit same-position guard, a compatible-stack merge branch, and then a zero-count/full-stack occupied-cell movement path that swaps ordinary carried items instead of treating every incompatible occupied destination as a rejection. This repository owns only the project-written carried-inventory subset of that behavior.

## Runtime acceptance

The minimal runtime accepts an item move only when all of these are true:

1. the session is in `GAME` phase;
2. source and destination windows are both `WindowInventory`;
3. source and destination cells are carried-inventory cells;
4. source and destination cells are different;
5. the selected player is active and not at the bootstrap HP floor;
6. the source stack exists and is not locked;
7. `count = 0` moves target an empty destination slot, a compatible occupied destination stack, or an incompatible occupied destination that can be swapped;
8. the destination stack, when occupied for a merge, is not locked;
9. occupied-destination merges require the same `vnum`, a valid loaded template that is stackable and not `anti_stack`, and must not overflow `max_count`; when an explicitly authored item-template snapshot is loaded, a missing source template for the same-`vnum` occupied destination is treated like missing stack metadata and the merge fails closed;
10. zero-count occupied-destination merges move as much of the source stack as the destination can accept, capped by the valid loaded template's `max_count`, and leave a source remainder when the destination fills first;
11. counted moves do not exceed the source stack count or the relevant template `max_count`;
12. if a counted move equals the source stack count and the destination is compatible, it merges into the destination stack instead of swapping items;
13. incompatible occupied-destination swaps are owned for `count = 0` full-stack moves and for non-zero counts that are greater than or equal to the source stack count;
14. partial incompatible occupied-destination moves still fail closed;
15. equipment destinations and sources are owned only for `WindowInventory` cells at `InventoryMaxCell + owned wear index`;
16. packet equip succeeds only when the destination equipment slot is empty, the source stack exists and is not locked, a valid loaded item template exists for the source `vnum`, that template authors the same `equip_slot` as the destination equipment cell, the selected character is allowed by that template's job/sex anti flags, and the runtime can encode the destination equipment cell;
17. when an item-template snapshot is explicitly authored/loaded, missing or invalid template metadata for the source `vnum` is authoritative and fails closed before equipment mutation;
18. the template-authored `equip_slot` and anti-flag guards are fail-closed for both the equipment mutation and any template-backed point side effect, so missing metadata, a mismatched slot, or a restricted selected character emits no frames and leaves live/persisted item state unchanged;
19. applying a template-backed equip point side effect must also fit the bootstrap signed 32-bit `PLAYER_POINT_CHANGE` value range; overflow fails closed before the point effect is committed;
20. packet unequip succeeds only when the equipment source exists and is not locked, the destination carried slot is empty, and the runtime can encode both touched cells;
21. equipment-cell equip or unequip requests require `count = 0`; non-zero counted equipment requests fail closed before mutation.

Rejected requests fail closed and emit no frames.

## Successful replies

The bootstrap runtime emits only self-facing item refresh frames for the mutated cells:

- full-stack move into an empty slot: `GC_ITEM_DEL(source)` then `GC_ITEM_SET(destination)`;
- full-stack swap with an incompatible occupied slot, including a non-zero count that covers the whole source stack: `GC_ITEM_SET(source with former destination item)` then `GC_ITEM_SET(destination with former source item)`, followed by `GC_QUICKSLOT_DEL` for any stale destination item quickslot and one `GC_QUICKSLOT_ADD` per matching item quickslot retargeted from source cell to destination cell;
- counted partial split into an empty slot: `GC_ITEM_SET(source remainder)` then `GC_ITEM_SET(destination split stack)`;
- counted or zero-count partial merge into a compatible destination: `GC_ITEM_UPDATE(source remainder count)` then `GC_ITEM_UPDATE(destination merged count)`;
- exact counted or zero-count full-stack merge into a compatible destination: `GC_ITEM_DEL(source)` then `GC_ITEM_UPDATE(destination merged count)`, followed by `GC_QUICKSLOT_DEL` for any stale destination item quickslot and one `GC_QUICKSLOT_ADD` per matching item quickslot retargeted from source cell to destination cell;
- packet equip from carried inventory to an owned equipment cell: `GC_ITEM_DEL(source)` then `GC_ITEM_SET(equipment position)`, followed by any template-backed `GC_POINT_CHANGE`, then self `GC_CHARACTER_UPDATE` for the projected appearance refresh, and then `GC_QUICKSLOT_DEL` for any item quickslot that referenced the carried source cell;
- packet unequip from an owned equipment cell to an empty carried-inventory slot: `GC_ITEM_DEL(equipment position)` then `GC_ITEM_SET(destination carried slot)`, followed by any template-backed `GC_POINT_CHANGE`, then self `GC_CHARACTER_UPDATE` for the projected appearance refresh.

The same mutation is persisted to the account snapshot after the response frames are built. If frame generation fails, the selected player runtime rolls back to the previous persisted snapshot before reporting rejection.

## Tests

Current coverage:

- `internal/proto/item` freezes the `CG::ITEM_MOVE` wire layout and invalid-header/payload rejection.
- `internal/game` freezes GAME-phase dispatch to `HandleItemMove` and fail-closed denied behavior.
- `internal/player` freezes runtime same-slot rejection, full-stack empty-destination, full-stack incompatible occupied-destination swaps, counted split/merge, exact counted full-stack compatible merge, max-count, counted incompatible-destination rejection, locked-stack behavior, and template-authored equip-slot and anti-flag guards.
- `internal/minimal` freezes runtime persistence and self-frame behavior for slash-command full-stack movement and direct `CG::ITEM_MOVE` full-stack moves, direct incompatible full-stack swaps, exact-count incompatible full-stack swaps, locked-destination rejection, counted partial incompatible-destination rejection, counted partial splits, compatible occupied-destination partial merges, exact counted full-stack compatible merges, fail-closed rejection when loaded item template metadata marks a same-`vnum` pair as `anti_stack`, fail-closed rejection when an explicitly authored item-template snapshot is missing the source `vnum` for a same-`vnum` occupied-destination merge, quickslot synchronization frames generated by carried moves where the source item lands in the destination cell, quickslot deletion when direct `CG::ITEM_MOVE` equip removes a carried item, direct `CG::ITEM_MOVE` carried-to-equipment equip, direct `CG::ITEM_MOVE` equipment-to-carried unequip, counted equipment equip/unequip fail-closed behavior, fail-closed rejection when an explicitly authored item-template snapshot is missing the source `vnum` for equipment, fail-closed rejection when loaded item template metadata authors a different `equip_slot` than the requested equipment destination, and fail-closed rejection when loaded item template metadata restricts the selected character by job/sex anti flags.
