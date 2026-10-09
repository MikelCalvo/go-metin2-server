# Combat Target-Marker Bootstrap

This note freezes the owned server target-marker packet shapes for `go-metin2-server` and the narrow runtime emission policy: self-only `TARGET_CREATE_NEW` after accepted non-zero client `TARGET`, self-only `TARGET_UPDATE` on an already-selected actor's same-map chase step, and self-only `TARGET_DELETE` when that owner explicitly clears the selected marker with `TARGET(0)`.

It sits next to:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `combat-fly-effect-bootstrap.md`
- `combat-pvp-duel-bootstrap.md`

## Scope

This slice owns the fixed server-to-client target-marker packet codecs plus combat-selection create, selected-target chase location update, and explicit owner clear emission rules.

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
- status: documented, codec-owned in `internal/proto/combat`, and emitted once as a self-only companion on one successful same-map selected-target chase step

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
- status: documented and codec-owned in `internal/proto/combat`; emitted on accepted explicit owner `TARGET(0)` only for a marker successfully queued for the current selected target

Payload layout:
1. `int32 id` (little-endian)

## Clean-room evidence summary

Client-source inspection of the TMP4-compatible client shows game-phase handlers for the target-marker family:
- `TARGET_CREATE_NEW` creates a minimap/target marker by id, name, optional actor `VID`, and type.
- `TARGET_UPDATE` updates a marker location by id and coordinates.
- `TARGET_DELETE` removes the marker by id.

The same client dispatch table keeps these packets separate from the selected-target HP packet `TARGET(0x0A10)`. Combat target selection continues to return `TARGET(0x0A10)` as the HP acknowledgement; create/update/delete companions use the pending self-only presentation path and do not replace the combat HP carrier.

## Current runtime rule

The shipped runtime now emits `TARGET_CREATE_NEW` only on this accepted non-zero `TARGET` seam:

- the session is already in `GAME` with a live selected character above the bootstrap `0`-HP floor
- the request is client `TARGET(0x0A01)` whose `target_vid` is non-zero and is accepted through the existing shared-world combat-target path
- the companion uses already-owned actor fields only: `id = int32(target_vid)`, `target_name` is the actor's current name, `vid = target_vid`, and `type = character`

On that accepted request the owner socket already receives:

1. `GC TARGET(target_vid, current_hp_percent)`

The same accepted selection queues one self-only `GC TARGET_CREATE_NEW` through the pending server-frame path when the actor name fits the fixed marker field. A valid combat actor with a name of 33 or more bytes still receives the selected-target HP ack, but its marker encoder rejects the name and no create is queued. Visible peers receive no marker fanout. An accepted explicit `TARGET(0)` clears the selection without an HP echo and queues exactly one self-only `GC TARGET_DELETE(id = int32(previous_target_vid))` **only if that selection successfully queued a create**. A failed create, including a long-name selection that replaces a previously marked target, cannot invent a delete for the current selection. Clearing again without a selected marker queues nothing. A rejected clear (including a dead or non-live owner) must neither clear nor enqueue a delete. No peer receives the delete. A pending create, if not yet drained, precedes its delete. Implicit target invalidation, death, restart, quest and map paths do not create this explicit-clear delete.

One successful same-map pending-frame chase step that already queues retained-viewer `MOVE` for an actor the living owner still has selected now also queues exactly one self-only `GC TARGET_UPDATE` to that owner. The update uses the already-created marker id (`id = int32(target_vid)`) and the stepped coordinates (`x`, `y`). It is delivered on that same pending-frame flush after the chase `MOVE` and after any same-flush delayed-retaliation frames, using the owned codec. It does not replace `TARGET(0x0A10)` as the HP carrier, does not emit `TARGET_DELETE`, and does not fan the marker out to peers who can see the chase `MOVE` but do not hold that selection. A chase step with no living selected owner, a stationary complete plan, homeward, return-step, and operator/runtime position `MOVE` still omit `TARGET_UPDATE`. Owners already at the bootstrap `0`-HP floor stay skipped.

The companion is presentation only, not a second combat simulation:

- it does not replace `TARGET(0x0A10)` as the selected-target HP carrier
- it does not mutate selected-target HP, cadence, retaliation, death, respawn, restart, inventory, points, or persistence
- it does not invent PvP/duel, stun, quest/minimap lifecycle, or implicit-clear marker delete
- the chase companion only relocates the marker already created for that selected actor; it does not invent a second chase scheduler, pathfinding, pack AI, or cross-map `MOVE` / `GC WARP`

Unsupported or rejected non-zero `TARGET` requests stay fail-closed with no HP ack and no marker companion.

## Relationship to current combat slices

Current accepted combat behavior stays on the already-owned surfaces:
- target selection and non-lethal hit refreshes continue to use `TARGET(target_vid, hp_percent)`,
- non-lethal hit effects continue to use `DAMAGE_INFO` where already owned,
- zero-HP edges continue to use `DEAD(vid)` plus `TARGET(0, 0)`,
- projectile, PvP, duel, stun, quest, and minimap marker lifecycle remain on their own slices.

This slice adds accepted non-zero `TARGET` → self-only `TARGET_CREATE_NEW`, already-selected same-map chase step → self-only `TARGET_UPDATE`, and accepted explicit `TARGET(0)` → self-only `TARGET_DELETE` for that selected marker. Other clear causes, quest/minimap lifecycle and peer fanout remain deferred.

## Non-goals

This slice does not freeze:
- quest target-marker authoring or lifecycle,
- map/minimap marker gameplay,
- marker fanout policy,
- client-originated target marker requests,
- replacing selected-combat-target `TARGET(0x0A10)` with marker packets,
- using marker packets as combat hit, death, or reward feedback,
- `TARGET_DELETE` on implicit target clears, death or other lifecycle events,
- emitting `TARGET_UPDATE` on hits, player `MOVE` / `SYNC_POSITION`, homeward, return-step, or operator position changes.

## Success definition

After this slice:
- `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE` remain listed in the packet matrix as documented server target-marker packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- an accepted non-zero client `TARGET` still returns one self-only `GC TARGET(target_vid, hp_percent)` and queues one self-only `GC TARGET_CREATE_NEW` using the already-owned actor name/VID and `type = character` when the name is encodable (up to 32 bytes),
- one successful same-map chase step for that still-selected living owner queues exactly one self-only `GC TARGET_UPDATE(id = int32(target_vid), x, y)` at the stepped coordinates without replacing the HP carrier,
- accepted explicit owner `TARGET(0)` after a successfully queued selected non-zero marker queues exactly one self-only `TARGET_DELETE` for that marker id, without an HP echo; unencodable actor names still get HP acks but do not produce a marker or a delete, and repeated or rejected clears do not queue a delete,
- rejected selection, ordinary hits, homeward/return-step movement, implicit clears, and peer sockets still omit marker deletes.
