# Combat Target-Marker Bootstrap

This note freezes the first owned server target-marker packet shapes for `go-metin2-server` without yet adding quest, minimap, or combat runtime emission.

It sits next to:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `combat-fly-effect-bootstrap.md`
- `combat-pvp-duel-bootstrap.md`

## Scope

This slice owns only the fixed server-to-client target-marker packet codecs and the current non-emission rule.

The packets are map/UI presentation helpers consumed by the game client while already in `GAME`. They are separate from the current selected-combat-target acknowledgement `TARGET(0x0A10)`, which remains the only runtime-visible combat target carrier used by accepted target selection and normal attacks.

## Packet shapes

### Server `TARGET_CREATE_NEW` (`0x0A13`)

- direction: server -> client
- phase: `GAME`
- header: `0x0A13`
- payload length: `42`
- total frame length: `46`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `int32 id` (little-endian)
2. `target_name[33]` fixed-width NUL-padded string
3. `uint32 vid` (little-endian)
4. `uint8 type`

The currently owned type labels are:
- `0` = none / unspecified
- `1` = location target
- `2` = character target

Encoding requires the name to fit the fixed 33-byte field with a terminating NUL; a 32-byte name is accepted and a 33-byte name is rejected.

### Server `TARGET_UPDATE` (`0x0A11`)

- direction: server -> client
- phase: `GAME`
- header: `0x0A11`
- payload length: `12`
- total frame length: `16`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `int32 id` (little-endian)
2. `int32 x` (little-endian)
3. `int32 y` (little-endian)

### Server `TARGET_DELETE` (`0x0A12`)

- direction: server -> client
- phase: `GAME`
- header: `0x0A12`
- payload length: `4`
- total frame length: `8`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `int32 id` (little-endian)

## Clean-room evidence summary

Client-source inspection of the TMP4-compatible client shows game-phase handlers for the target-marker family:
- `TARGET_CREATE_NEW` creates a minimap/target marker by id, name, optional actor `VID`, and type.
- `TARGET_UPDATE` updates a marker location by id and coordinates.
- `TARGET_DELETE` removes the marker by id.

The same client dispatch table keeps these packets separate from the selected-target HP packet `TARGET(0x0A10)`. That distinction is important for this repo: combat target selection continues to use `TARGET(0x0A10)`, while marker packets remain presentation-only codecs until a later quest/minimap/target-marker runtime slice owns emission.

## Relationship to current combat slices

Current accepted combat behavior is unchanged:
- target selection and non-lethal hit refreshes continue to use `TARGET(target_vid, hp_percent)`,
- non-lethal hit effects continue to use `DAMAGE_INFO` where already owned,
- zero-HP edges continue to use `DEAD(vid)` plus `TARGET(0, 0)`,
- projectile, PvP, duel, quest, and marker presentation remain non-emitting unless a later slice adds transcript-level runtime tests.

The shipped runtime does not currently emit `TARGET_CREATE_NEW`, `TARGET_UPDATE`, or `TARGET_DELETE` from combat, quest, NPC, map, or slash-command paths.

## Non-goals

This slice does not freeze:
- quest target-marker authoring or lifecycle,
- map/minimap marker gameplay,
- marker fanout policy,
- client-originated target marker requests,
- replacing selected-combat-target `TARGET(0x0A10)` with marker packets,
- using marker packets as combat hit, death, or reward feedback.

## Success definition

After this slice:
- `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE` are listed in the packet matrix as documented server target-marker packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- later quest/minimap/target-marker runtime slices can start from tested packet shapes without changing current bootstrap combat emission.
