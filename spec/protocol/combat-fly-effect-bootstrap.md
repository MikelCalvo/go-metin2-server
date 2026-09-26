# Combat Fly-Effect Bootstrap

This note freezes the first owned server fly-effect packet shapes for `go-metin2-server` and the first deliberately narrow runtime emission policy: one self-only `CREATE_FLY` presentation companion after an accepted client `FLY_TARGETING`, the bootstrap presentation `USE_SKILL`, or the bootstrap presentation `SHOOT` against the currently selected visible combat target. An accepted client `FLY_TARGETING` also queues that same `CREATE_FLY` to currently visible live peers, and the owner socket receives one self-only server `FLY_TARGETING` echo beside that projectile.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `combat-damage-info-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`

## Scope

This slice owns three fixed server-to-client packet codecs plus the first `FLY_TARGETING`, bootstrap presentation `USE_SKILL`, and bootstrap presentation `SHOOT` emission rules.

The packets are:

### `FLY_TARGETING`

- direction: server -> client
- phase: `GAME`
- header: `0x0411`
- payload length: `16`
- status: documented, codec-owned in `internal/proto/combat`, and emitted as one self-only echo

Payload layout:
1. `uint32 shooter_vid` (little-endian)
2. `uint32 target_vid` (little-endian)
3. `int32 x` (little-endian)
4. `int32 y` (little-endian)

An accepted client `FLY_TARGETING` echoes this packet once to the owner socket beside the already-owned `CREATE_FLY`. `shooter_vid` is the selected owner's visible VID, `target_vid` is the currently selected combat target, and `x` / `y` are the client request coordinates copied through unchanged. The echo is self-only: currently visible peers still receive only the already-owned `CREATE_FLY`. Bootstrap presentation `USE_SKILL`, `SHOOT`, ordinary `ATTACK`, and unsupported `FLY_TARGETING` do not emit it.

### `ADD_FLY_TARGETING`

- direction: server -> client
- phase: `GAME`
- header: `0x0412`
- payload length: `16`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout matches `FLY_TARGETING`:
1. `uint32 shooter_vid` (little-endian)
2. `uint32 target_vid` (little-endian)
3. `int32 x` (little-endian)
4. `int32 y` (little-endian)

Client `ADD_FLY_TARGETING` and this server packet stay decode-and-fail-closed / codec-only. Multi-target and chained projectile presentation remain later policy.

### `CREATE_FLY`

- direction: server -> client
- phase: `GAME`
- header: `0x0413`
- payload length: `9`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `uint8 type`
2. `uint32 start_vid` (little-endian)
3. `uint32 end_vid` (little-endian)

This first GREEN always uses `type = 0` as the bootstrap projectile presentation. Richer visual type meanings stay deferred. `start_vid` is the selected owner's visible VID; `end_vid` is the currently selected combat target VID.

## Clean-room evidence summary

Client-source inspection of the TMP4-compatible client shows separate game-phase receive handlers for server-originated fly effects:
- `FLY_TARGETING` and `ADD_FLY_TARGETING` both identify a shooter, an optional target, and fallback world coordinates.
- `CREATE_FLY` identifies a fly/effect type plus start and end actor VIDs.

The same source also shows client-originated `FLY_TARGETING` / `ADD_FLY_TARGETING` and `SHOOT` requests from bow-style event handlers. Those client packets are already owned as `GAME` ingress. This note keeps `ADD_FLY_TARGETING` fail-closed and reuses the already-owned `CREATE_FLY` codec on three existing seams: an accepted client `FLY_TARGETING` whose `target_vid` matches the session's currently selected visible combat target, one accepted client `USE_SKILL` with bootstrap presentation `skill_vnum = 1` against that same selected target, and one accepted client `SHOOT` whose `shoot_type` is the bootstrap presentation value `1` while that same target is already selected. The `SHOOT` packet carries only `shoot_type`, so the target stays the session's current selection. The accepted `FLY_TARGETING` seam also echoes one self-only server `FLY_TARGETING` using the request coordinates as the fallback world position already named by that packet.

It does not invent skill resource or cooldown tables, projectile travel duration, hit-timing formulas, a type catalog beyond bootstrap `type = 0`, or a second damage path.

## Current runtime rule

The shipped runtime now emits `CREATE_FLY` on three presentation-only seams that share the same selected-target policy:

- the session is already in `GAME` with a live selected character above the bootstrap `0`-HP floor
- that character currently holds a selected combat target accepted through the existing `TARGET` path
- the request `target_vid` matches that selected target exactly and is still a currently visible in-range combat target
- `CREATE_FLY` names actor VIDs only

The seams are:

1. client `FLY_TARGETING(0x0404)`; the request coordinates are copied into the self-only server echo and are not used as hit timing, travel, or a second target
2. client `USE_SKILL(0x0402)` whose `skill_vnum` is the bootstrap presentation value `1`
3. client `SHOOT(0x0403)` whose `shoot_type` is the bootstrap presentation value `1`; the packet has no target field, so the end VID is the already selected combat target

On any accepted request the owner socket receives:

1. `GC CREATE_FLY(type = 0, start_vid = owner_vid, end_vid = target_vid)`

An accepted client `FLY_TARGETING` also returns, on that same owner socket and before the projectile:

1. `GC FLY_TARGETING(shooter_vid = owner_vid, target_vid = target_vid, x = request_x, y = request_y)`

That echo stays self-only. The same accepted request still queues only `CREATE_FLY` to currently visible live peers that can already see the selected combat target. Peers already at the bootstrap `0`-HP floor stay skipped. Bootstrap presentation `USE_SKILL` and `SHOOT` stay one self-only `CREATE_FLY` and do not emit server `FLY_TARGETING`. The companions are presentation only, not a second combat simulation:

- it does not mutate selected-target HP
- it does not rewrite the selected target
- it does not change normal-attack cadence, retaliation, death, respawn, restart, inventory, points, or persistence
- it does not spend skill points, start a cooldown, or apply hit timing
- it does not emit server `ADD_FLY_TARGETING`, knockdown, or PvP/duel packets
- the server `FLY_TARGETING` echo does not replace `CREATE_FLY`, `DAMAGE_INFO`, `TARGET`, or `DEAD`

Unsupported `FLY_TARGETING` without that policy stays fail-closed: missing selection, `target_vid = 0`, VID mismatch, stale/dead/invisible/out-of-range targets, zero-HP owners, and any path that is not this accepted selected-target request return no frames and leave combat state unchanged. `USE_SKILL` uses the same fail-closed rule, and any `skill_vnum` other than bootstrap presentation `1` also stays fail-closed. `SHOOT` uses the same selected-target rule, and any `shoot_type` other than bootstrap presentation `1` also stays fail-closed. Client `ADD_FLY_TARGETING` and ordinary `ATTACK` still do not emit `CREATE_FLY` or server `FLY_TARGETING`. A repeat of the same accepted client `FLY_TARGETING` emits another self-only echo beside another `CREATE_FLY`. A repeat of the same accepted `USE_SKILL` or `SHOOT` emits another presentation `CREATE_FLY` and still omits the server echo; this slice does not own a skill cooldown or a shot cooldown.

## Relationship to current combat slices

Current accepted normal attacks still use the already-owned combat presentation surfaces:
- non-lethal hits use `TARGET(target_vid, hp_percent)` plus `DAMAGE_INFO` according to `combat-damage-info-bootstrap.md`,
- killing hits use `DEAD(vid)` plus `TARGET(0, 0)` before any owned reward feedback,
- content practice-mob retaliation continues to use `PLAYER_POINT_CHANGE` and the current delayed server-frame cadence,
- sitting standalone dummy hits may still queue the owned self-only `STUN` companion.

This first GREEN adds the selected-target `FLY_TARGETING` → `CREATE_FLY` presentation companion, queues that same `CREATE_FLY` frame to currently visible live peers, returns one self-only server `FLY_TARGETING` echo beside it, and reuses the self-only `CREATE_FLY` companion for one accepted bootstrap presentation `USE_SKILL` and one accepted bootstrap presentation `SHOOT`. Later skill resource/cooldown tables, hit-timing, shot-type catalogs, skill/shoot peer fanout, peer fanout of the server `FLY_TARGETING` echo, or server `ADD_FLY_TARGETING` echoes must freeze their own policy instead of widening this seam by implication.

## Non-goals

This slice does not freeze:
- ranged `SHOOT` gameplay beyond the one presentation-only bootstrap `shoot_type = 1` → `CREATE_FLY` companion,
- skill resource costs, cooldowns, hit timing, or skill combat beyond the one presentation-only `USE_SKILL(skill_vnum = 1)` → `CREATE_FLY` companion,
- projectile hit timing or travel duration,
- visual effect type meanings beyond bootstrap `CREATE_FLY` `type = 0`,
- multi-target or chained projectile behavior, including client/server `ADD_FLY_TARGETING`,
- peer fanout of bootstrap presentation `USE_SKILL` or `SHOOT` fly effects,
- peer fanout of the server `FLY_TARGETING` echo,
- killing-hit fly effects,
- server `ADD_FLY_TARGETING` runtime emission,
- any replacement for `DAMAGE_INFO`, `TARGET`, or `DEAD` as the current combat result surfaces.

## Success definition

After this slice:
- `FLY_TARGETING`, `ADD_FLY_TARGETING`, and `CREATE_FLY` remain listed in the packet matrix as documented server combat/fly-effect packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- an accepted client `FLY_TARGETING` against the currently selected visible combat target emits one self-only `GC FLY_TARGETING(shooter_vid = owner_vid, target_vid = target_vid, x = request_x, y = request_y)` followed by one `GC CREATE_FLY(type = 0, start_vid = owner_vid, end_vid = target_vid)`, and queues only that `CREATE_FLY` frame to currently visible live peers,
- peers already at the bootstrap `0`-HP floor receive no fly-effect frame, and no peer receives the server `FLY_TARGETING` echo,
- an accepted client `USE_SKILL` with bootstrap presentation `skill_vnum = 1` against that same selected target emits the same self-only `CREATE_FLY`,
- an accepted client `SHOOT` with bootstrap presentation `shoot_type = 1` while that same target is selected emits the same self-only `CREATE_FLY`,
- that `CREATE_FLY` does not mutate HP, cadence, retaliation, selection, points, inventory, or persistence, and it does not spend skill points or start a cooldown,
- visible peers receive no fly-effect frame from `USE_SKILL` or `SHOOT`,
- unsupported `FLY_TARGETING` without the new policy, other `USE_SKILL` vnums, other `SHOOT` types, `ADD_FLY_TARGETING`, and ordinary `ATTACK` stay fail-closed for fly emission,
- later ranged/projectile/skill slices can start from this tested packet shape and this first `FLY_TARGETING` emission rule instead of re-discovering them.
