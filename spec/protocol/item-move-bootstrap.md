# Item move bootstrap

This note freezes the current clean-room bootstrap contract for client-driven carried-inventory item movement on the game socket.

## Scope

Owned in this slice:

- `CG::ITEM_MOVE` is accepted only in `GAME` phase through the normal game flow dispatch.
- The request is limited to carried inventory slots in the bootstrap runtime.
- Full-stack moves, swaps, counted partial splits, and counted partial merges use the same runtime inventory mutation path.
- Successful mutations are self-only and refresh the touched inventory cells with existing item packets.

Not owned yet:

- Safebox, mall, belt, dragon-soul, equipment-window, or ground movement.
- Counted move semantics for incompatible occupied destinations beyond fail-closed rejection.
- Cross-window drag/drop behavior.
- Server-side pickup/drop packets or item ownership on the ground.
- Any claim that this layout has been verified against the legacy source; current reference-source searches did not find an item-move send/receive symbol, so this contract is project-owned until capture/source evidence is added.

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

`count = 0` means full-stack move/swap. Non-zero `count` means a counted move bounded by the source item's template `max_count` when a valid template exists.

## Runtime acceptance

The minimal runtime accepts an item move only when all of these are true:

1. the session is in `GAME` phase;
2. source and destination windows are both `WindowInventory`;
3. source and destination cells are carried-inventory cells;
4. the selected player is active and not at the bootstrap HP floor;
5. the source stack exists and is not locked;
6. the destination stack, when occupied, is not locked;
7. counted moves do not exceed the source stack count or the relevant template `max_count`;
8. counted moves into an occupied destination require the same `vnum` and must not overflow `max_count`.

Rejected requests fail closed and emit no frames.

## Successful replies

The bootstrap runtime emits only self-facing item refresh frames for the mutated cells:

- full-stack move into an empty slot: `GC_ITEM_DEL(source)` then `GC_ITEM_SET(destination)`;
- full-stack swap: `GC_ITEM_SET(source)` then `GC_ITEM_SET(destination)`;
- counted partial split into an empty slot: `GC_ITEM_SET(source remainder)` then `GC_ITEM_SET(destination split stack)`;
- counted partial merge into a compatible destination: `GC_ITEM_UPDATE(source remainder count)` then `GC_ITEM_UPDATE(destination merged count)`.

The same mutation is persisted to the account snapshot after the response frames are built. If frame generation fails, the selected player runtime rolls back to the previous persisted snapshot before reporting rejection.

## Tests

Current coverage:

- `internal/proto/item` freezes the `CG::ITEM_MOVE` wire layout and invalid-header/payload rejection.
- `internal/game` freezes GAME-phase dispatch to `HandleItemMove` and fail-closed denied behavior.
- `internal/player` freezes runtime full-stack, counted split/merge, max-count, incompatible-destination, and locked-stack behavior.
- `internal/minimal` freezes runtime persistence and self-frame behavior for slash-command full-stack movement and direct `CG::ITEM_MOVE` counted partial splits and compatible occupied-destination partial merges.
