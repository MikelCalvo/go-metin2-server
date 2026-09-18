# Combat Target-Marker Bootstrap

This note freezes the first owned server target-marker packet shapes for `go-metin2-server` and the first deliberately narrow runtime emission policy: one self-only `TARGET_CREATE_NEW` presentation companion after an accepted non-zero client `TARGET`.

It sits next to:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `combat-fly-effect-bootstrap.md`
- `combat-pvp-duel-bootstrap.md`

## Scope

This slice owns the fixed server-to-client target-marker packet codecs plus the first combat-selection emission rule.

The packets are map/UI presentation helpers consumed by the game client while already in `GAME`. They stay separate from the current selected-combat-target acknowledgement `TARGET(0x0A10)`, which remains the HP/selection carrier used by accepted target selection and normal attacks.

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

The same client dispatch table keeps these packets separate from the selected-target HP packet `TARGET(0x0A10)`. That distinction stays important: combat target selection continues to return `TARGET(0x0A10)` as the HP acknowledgement, while this first GREEN only queues one self-only `TARGET_CREATE_NEW` presentation companion after that accepted non-zero selection.

## Current runtime rule

The shipped runtime now emits `TARGET_CREATE_NEW` only on this accepted non-zero `TARGET` seam:

- the session is already in `GAME` with a live selected character above the bootstrap `0`-HP floor
- the request is client `TARGET(0x0A01)` whose `target_vid` is non-zero and is accepted through the existing shared-world combat-target path
- the companion uses already-owned actor fields only: `id = int32(target_vid)`, `target_name` is the actor's current name, `vid = target_vid`, and `type = character`

On that accepted request the owner socket already receives:

1. `GC TARGET(target_vid, current_hp_percent)`

The same accepted selection then queues exactly one self-only `GC TARGET_CREATE_NEW` through the pending server-frame path. Visible peers receive no marker fanout in this first GREEN. Client `TARGET(0)` stays a silent clear with no HP echo and no marker companion. Hits, sit/stand, fly targeting, skill, death, restart, quest, and map paths still do not emit `TARGET_UPDATE` or `TARGET_DELETE`.

The companion is presentation only, not a second combat simulation:

- it does not replace `TARGET(0x0A10)` as the selected-target HP carrier
- it does not mutate selected-target HP, cadence, retaliation, death, respawn, restart, inventory, points, or persistence
- it does not invent PvP/duel, stun, quest/minimap lifecycle, or marker delete/update

Unsupported or rejected non-zero `TARGET` requests stay fail-closed with no HP ack and no marker companion.

## Relationship to current combat slices

Current accepted combat behavior stays on the already-owned surfaces:
- target selection and non-lethal hit refreshes continue to use `TARGET(target_vid, hp_percent)`,
- non-lethal hit effects continue to use `DAMAGE_INFO` where already owned,
- zero-HP edges continue to use `DEAD(vid)` plus `TARGET(0, 0)`,
- projectile, PvP, duel, stun, quest, and minimap marker lifecycle remain on their own slices.

This first GREEN only adds the accepted non-zero `TARGET` → self-only `TARGET_CREATE_NEW` presentation companion. Later quest/minimap, peer-fanout, or `TARGET_UPDATE`/`TARGET_DELETE` slices must freeze their own policy instead of widening this seam by implication.

## Non-goals

This slice does not freeze:
- quest target-marker authoring or lifecycle,
- map/minimap marker gameplay,
- marker fanout policy,
- client-originated target marker requests,
- replacing selected-combat-target `TARGET(0x0A10)` with marker packets,
- using marker packets as combat hit, death, or reward feedback,
- `TARGET_UPDATE` or `TARGET_DELETE` runtime emission.

## Success definition

After this slice:
- `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE` remain listed in the packet matrix as documented server target-marker packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- an accepted non-zero client `TARGET` still returns one self-only `GC TARGET(target_vid, hp_percent)` and then queues one self-only `GC TARGET_CREATE_NEW` using the already-owned actor name/VID and `type = character`,
- client `TARGET(0)`, rejected selection, ordinary hits, and peer sockets still omit `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE`.
