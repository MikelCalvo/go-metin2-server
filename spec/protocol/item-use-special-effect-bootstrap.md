# Item-use special-effect bootstrap

This note freezes the first visual-effect companion for the existing template-backed `ITEM_USE` consumable path.

## Scope

The slice is intentionally narrow:

- accepted direct carried consumable use may emit one self-only server `SPECIAL_EFFECT` packet when the consumed item template authors `use_effect.special_effect_type`
- the effect is driven only by file-backed item-template metadata, not by a runtime `vnum` switch
- the effect targets the selected character's current `VID`
- the existing point, stack, quickslot, persistence, stale-session, and rejection rules from `item-use-bootstrap.md` remain unchanged

This does not add timed buffs, cooldowns, peer-visible area effects, equipment-use effects, quest/applicable item behavior, or a general multi-effect scripting system.

## Packet contract

`SPECIAL_EFFECT` is server -> client:

- header: `0x0A30`
- payload length: `5` bytes
- payload layout:
  - `type uint8`
  - `vid uint32` little-endian

The first item-use runtime slice owns only encoding this server packet and decoding it in tests.
The broader `SPECIAL_EFFECT` enum names are recorded as constants for readability, but runtime emission is currently limited to values explicitly authored in a loaded item template.

## Template metadata

`use_effect.special_effect_type` is optional:

- omitted or `0`: no visual-effect packet is emitted, preserving the older item-use burst
- `1..25`: accepted by the item-template store and emitted as the `SPECIAL_EFFECT.type`
- values above `25`: rejected at item-template validation/load time

The current cap is conservative. It keeps the first authored visual-effect field inside the currently owned TMP4-era client enum range while leaving future extended effect policy for a separate slice.

## Success burst

When packet-originated `ITEM_USE` consumes a carried item whose template authors non-zero `use_effect.special_effect_type`, the self-only burst is:

1. `ITEM_USE` echo (`0x0512`)
2. `PLAYER_POINT_CHANGE`
3. item refresh for the consumed cell (`ITEM_UPDATE` or `ITEM_DEL`)
4. zero or more `QUICKSLOT_DEL` frames if the stack was removed
5. `SPECIAL_EFFECT(type = template.use_effect.special_effect_type, vid = selected character VID)`
6. `CHAT_TYPE_INFO` placeholder text

The slash `/use_item <slot>` harness keeps the same sequence without the packet-only `ITEM_USE` echo.
If the template omits `special_effect_type` or sets it to `0`, the burst remains the older sequence documented in `item-use-bootstrap.md`.

## Failure and persistence boundaries

The visual effect is emitted only after the existing item-use mutation has succeeded and frame construction has reached the response-burst stage.
All existing item-use fail-closed cases still emit no special-effect packet unless the failure path is an authored rejection chat for guarded direct use.

The special-effect packet itself is not persisted. Persistence remains limited to the mutated selected-character points, inventory, and quickslots already owned by the item-use path.

## Tests

Current coverage:

- `internal/proto/effect` freezes the `SPECIAL_EFFECT` wire layout and decode guards
- `internal/itemstore` round-trips `use_effect.special_effect_type` deterministically and rejects out-of-range values
- `internal/minimal` proves packet-originated `ITEM_USE` emits the template-authored self-only `SPECIAL_EFFECT` after the point/item refresh and before the placeholder info chat
