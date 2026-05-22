# Item move bootstrap

This note freezes the current clean-room bootstrap contract for client-driven carried-inventory item movement on the game socket.

## Scope

Owned in this slice:

- `CG::ITEM_MOVE` is accepted only in `GAME` phase through the normal game flow dispatch.
- The request is limited to carried inventory slots in the bootstrap runtime.
- Same-position move requests fail closed before runtime mutation, matching the legacy dupe-guard behavior.
- Full-stack moves into empty carried slots, counted partial splits, counted partial merges, and exact counted full-stack merges into a compatible destination use the same runtime inventory mutation path.
- Successful mutations are self-only and refresh the touched inventory cells with existing item packets.

Not owned yet:

- Safebox, mall, belt, dragon-soul, equipment-window, or ground movement.
- Occupied-destination swap semantics for `count = 0` when the destination is incompatible; the current packet path fails closed there until the precise compatibility rule is frozen.
- Counted move semantics for incompatible occupied destinations beyond fail-closed rejection.
- Stack splitting/merging outside carried inventory.
- Cross-window drag/drop behavior.
- Server-side pickup/drop packets or item ownership on the ground.
- Any claim that the whole layout has been verified against the legacy source; current reference-source evidence has only been used to align the compatible occupied-destination stack merge rule where `count = 0` means use the whole source stack, capped by destination stack capacity.

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

The current carried-inventory window is `WindowInventory = 1`.

`count = 0` means a full-stack move. In the current owned packet path it succeeds when the destination carried slot is empty, or when the destination is a compatible stackable item that can accept at least one source item up to the source template's `max_count`. Non-zero `count` means a counted move bounded by the source item's template `max_count` when a valid template exists.

## Runtime acceptance

The minimal runtime accepts an item move only when all of these are true:

1. the session is in `GAME` phase;
2. source and destination windows are both `WindowInventory`;
3. source and destination cells are carried-inventory cells;
4. source and destination cells are different;
5. the selected player is active and not at the bootstrap HP floor;
6. the source stack exists and is not locked;
7. `count = 0` moves either target an empty destination slot or a compatible occupied destination stack;
8. the destination stack, when occupied for a merge, is not locked;
9. occupied-destination merges require the same `vnum` and must not overflow `max_count`;
10. zero-count occupied-destination merges move as much of the source stack as the destination can accept, capped by `max_count`, and leave a source remainder when the destination fills first;
11. counted moves do not exceed the source stack count or the relevant template `max_count`;
12. if a counted move equals the source stack count and the destination is compatible, it merges into the destination stack instead of swapping items.

Rejected requests fail closed and emit no frames.

## Successful replies

The bootstrap runtime emits only self-facing item refresh frames for the mutated cells:

- full-stack move into an empty slot: `GC_ITEM_DEL(source)` then `GC_ITEM_SET(destination)`;
- counted partial split into an empty slot: `GC_ITEM_SET(source remainder)` then `GC_ITEM_SET(destination split stack)`;
- counted or zero-count partial merge into a compatible destination: `GC_ITEM_UPDATE(source remainder count)` then `GC_ITEM_UPDATE(destination merged count)`;
- exact counted or zero-count full-stack merge into a compatible destination: `GC_ITEM_DEL(source)` then `GC_ITEM_UPDATE(destination merged count)`.

The same mutation is persisted to the account snapshot after the response frames are built. If frame generation fails, the selected player runtime rolls back to the previous persisted snapshot before reporting rejection.

## Tests

Current coverage:

- `internal/proto/item` freezes the `CG::ITEM_MOVE` wire layout and invalid-header/payload rejection.
- `internal/game` freezes GAME-phase dispatch to `HandleItemMove` and fail-closed denied behavior.
- `internal/player` freezes runtime same-slot rejection, full-stack empty-destination, counted split/merge, exact counted full-stack compatible merge, max-count, incompatible-destination, occupied-destination no-count rejection, and locked-stack behavior.
- `internal/minimal` freezes runtime persistence and self-frame behavior for slash-command full-stack movement and direct `CG::ITEM_MOVE` full-stack moves, occupied-destination no-count rejection, counted partial splits, compatible occupied-destination partial merges, and exact counted full-stack compatible merges.
