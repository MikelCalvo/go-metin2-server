# Combat Fly-Effect Bootstrap

This note freezes the first owned server fly-effect packet shapes for `go-metin2-server` and a narrow runtime emission policy: `CREATE_FLY` accompanies accepted client `FLY_TARGETING`, bootstrap presentation `USE_SKILL` and `SHOOT`, and a selected-target normal `ATTACK` that kills a visible combat actor **after an accepted fly-targeting intent**. Accepted client `FLY_TARGETING` and `SHOOT(shoot_type = 1)` also queue that same `CREATE_FLY` to currently visible live peers that can see the selected target; the owner socket receives a self-only server `FLY_TARGETING` echo only for accepted client `FLY_TARGETING`.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `combat-damage-info-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`

## Scope

This note owns three fixed server-to-client packet codecs plus selected-target `FLY_TARGETING`/`ADD_FLY_TARGETING`, bootstrap presentation `USE_SKILL`/`SHOOT`, and one selected-target killing-hit emission rule.

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
- status: documented and codec-owned in `internal/proto/combat`; emitted self-only for accepted same-selected-target client `ADD_FLY_TARGETING`

Payload layout matches `FLY_TARGETING`:
1. `uint32 shooter_vid` (little-endian)
2. `uint32 target_vid` (little-endian)
3. `int32 x` (little-endian)
4. `int32 y` (little-endian)

An accepted client `ADD_FLY_TARGETING` naming the same currently selected, visible, in-range living combat target returns one self-only `GC ADD_FLY_TARGETING(shooter_vid = owner_vid, target_vid = target_vid, x = request_x, y = request_y)` followed by one self-only `GC CREATE_FLY(type = 0, start_vid = owner_vid, end_vid = target_vid)`. Coordinates are copied unchanged as fallback position, not interpreted as a second target or hit position. No peer receives either frame from this seam. It does not arm a killing-hit fly intent or alter selection, HP, cadence, or damage. Missing selection, zero or mismatched VID, dead/invisible/out-of-range/stale targets, and owners at the zero-HP floor receive no response. Repeats independently emit the same pair, not a chained list. Multi-target and chained projectile presentation remain later policy.

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

The same source also shows client-originated `FLY_TARGETING` / `ADD_FLY_TARGETING` and `SHOOT` requests from bow-style event handlers. Those client packets are already owned as `GAME` ingress. This note reuses the already-owned `CREATE_FLY` codec on selected-target `FLY_TARGETING`, same-target `ADD_FLY_TARGETING`, bootstrap `USE_SKILL(skill_vnum = 1)`, and bootstrap `SHOOT(shoot_type = 1)`. The `SHOOT` packet carries only `shoot_type`, so the target stays the session's current selection. The accepted targeting seams echo their corresponding server targeting packet with the request coordinates copied as fallback world position.

It does not invent skill resource or cooldown tables, projectile travel duration, hit-timing formulas, a type catalog beyond bootstrap `type = 0`, or a second damage path.

## Current runtime rule

The shipped runtime now emits `CREATE_FLY` on four presentation-only seams that share the same selected-target policy:

- the session is already in `GAME` with a live selected character above the bootstrap `0`-HP floor
- that character currently holds a selected combat target accepted through the existing `TARGET` path
- the request `target_vid` matches that selected target exactly and is still a currently visible in-range combat target
- `CREATE_FLY` names actor VIDs only

The seams are:

1. client `FLY_TARGETING(0x0404)`; the request coordinates are copied into the self-only server echo and are not used as hit timing, travel, or a second target
2. client `ADD_FLY_TARGETING(0x0405)` naming that same currently selected target; the request coordinates are copied into its self-only server echo, with no chain/list state or killing-hit intent
3. client `USE_SKILL(0x0402)` whose `skill_vnum` is the bootstrap presentation value `1`
4. client `SHOOT(0x0403)` whose `shoot_type` is the bootstrap presentation value `1`; the packet has no target field, so the end VID is the already selected combat target

On any accepted request the owner socket receives:

1. `GC CREATE_FLY(type = 0, start_vid = owner_vid, end_vid = target_vid)`

An accepted client `FLY_TARGETING` also returns, on that same owner socket and before the projectile:

1. `GC FLY_TARGETING(shooter_vid = owner_vid, target_vid = target_vid, x = request_x, y = request_y)`

An accepted client `ADD_FLY_TARGETING` instead returns its matching `GC ADD_FLY_TARGETING` echo before its own `CREATE_FLY`, both self-only. Accepted primary `FLY_TARGETING` and bootstrap `SHOOT(shoot_type = 1)` each queue only `CREATE_FLY` to currently visible live peers that can already see the selected combat target. Peers already at the bootstrap `0`-HP floor stay skipped. Bootstrap presentation `USE_SKILL` stays self-only; neither skill nor shoot emits server targeting echoes. The companions are presentation only, not a second combat simulation:

- it does not mutate selected-target HP
- it does not rewrite the selected target
- it does not change normal-attack cadence, retaliation, death, respawn, restart, inventory, points, or persistence
- it does not spend skill points, start a cooldown, or apply hit timing
- it does not emit knockdown or PvP/duel packets
- neither targeting echo replaces `CREATE_FLY`, `DAMAGE_INFO`, `TARGET`, or `DEAD`

Unsupported `FLY_TARGETING` and `ADD_FLY_TARGETING` stay fail-closed: missing selection, `target_vid = 0`, VID mismatch, stale/dead/invisible/out-of-range targets, and zero-HP owners return no frames and leave combat state unchanged. `USE_SKILL` uses the same fail-closed rule, and any `skill_vnum` other than bootstrap presentation `1` also stays fail-closed. `SHOOT` uses the same selected-target rule, and any `shoot_type` other than bootstrap presentation `1` also stays fail-closed. Non-lethal or rejected `ATTACK` still do not emit `CREATE_FLY` or a targeting echo; killing `ATTACK` emits no targeting echo. Repeating either accepted targeting request emits another self-only corresponding echo and `CREATE_FLY`. Repeating accepted `USE_SKILL` or `SHOOT` emits another presentation `CREATE_FLY` and still omits targeting echoes; this slice does not own a skill cooldown or a shot cooldown.

## Relationship to current combat slices

Current accepted normal attacks still use the already-owned combat presentation surfaces:
- non-lethal hits use `TARGET(target_vid, hp_percent)` plus `DAMAGE_INFO` according to `combat-damage-info-bootstrap.md`,
- killing hits use `DEAD(vid)` plus `TARGET(0, 0)` before any owned reward feedback,
- content practice-mob retaliation continues to use `PLAYER_POINT_CHANGE` and the current delayed server-frame cadence,
- sitting standalone dummy hits may still queue the owned self-only `STUN` companion.

The killing-hit companion reuses the selected-target visibility/version check before damage: an accepted client `FLY_TARGETING` for the current selection arms **one** subsequent accepted normal hit on that same actor/version. `ADD_FLY_TARGETING` alone does not arm or consume this intent. Only if that hit actually crosses the zero-HP edge does it emit an additional self-only `CREATE_FLY(type=0, start_vid=owner_vid, end_vid=target_vid)`. A non-lethal accepted hit consumes the intent without a killing fly; rejected/cadence-denied hits do not consume it. Explicit target clear, changed selection/version, or session reset discards the intent; stale or dead targets still fail closed. On the owner socket the killing fly follows `DEAD(target_vid)`, `TARGET(0, 0)` and the existing killing-hit `DAMAGE_INFO`, and precedes any owned reward. It does not add hit delay, a second damage path, another targeting echo or peer fly fanout; peers keep their existing death/damage visibility surfaces. A plain normal killing hit without a preceding accepted primary fly intent retains the existing frame count. Peer killing-hit fly fanout belongs to COMBAT-FLY-PEER-FANOUT; chained ADD_FLY_TARGETING, travel duration and visual types beyond type=0 remain deferred.

## Non-goals

This slice does not freeze:
- ranged `SHOOT` gameplay beyond the one presentation-only bootstrap `shoot_type = 1` → `CREATE_FLY` companion,
- skill resource costs, cooldowns, hit timing, or skill combat beyond the one presentation-only `USE_SKILL(skill_vnum = 1)` → `CREATE_FLY` companion,
- projectile hit timing or travel duration,
- visual effect type meanings beyond bootstrap `CREATE_FLY` `type = 0`,
- multi-target or chained projectile behavior beyond the same-selected-target `ADD_FLY_TARGETING` self-only echo,
- peer fanout of bootstrap presentation `USE_SKILL` fly effects,
- peer fanout of the server `FLY_TARGETING` echo,
- peer killing-hit fly effects,
- peer fanout for the server `ADD_FLY_TARGETING` echo or its `CREATE_FLY` companion,
- any replacement for `DAMAGE_INFO`, `TARGET`, or `DEAD` as the current combat result surfaces.

## Success definition

After this slice:
- `FLY_TARGETING`, `ADD_FLY_TARGETING`, and `CREATE_FLY` remain listed in the packet matrix as documented server combat/fly-effect packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- an accepted client `FLY_TARGETING` against the currently selected visible combat target emits one self-only `GC FLY_TARGETING(shooter_vid = owner_vid, target_vid = target_vid, x = request_x, y = request_y)` followed by one `GC CREATE_FLY(type = 0, start_vid = owner_vid, end_vid = target_vid)`, and queues only that `CREATE_FLY` frame to currently visible live peers,
- peers already at the bootstrap `0`-HP floor receive no fly-effect frame, and no peer receives the server `FLY_TARGETING` echo,
- an accepted client `USE_SKILL` with bootstrap presentation `skill_vnum = 1` against that same selected target emits the same self-only `CREATE_FLY`,
- an accepted client `SHOOT` with bootstrap presentation `shoot_type = 1` while that same target is selected emits the same `CREATE_FLY` to the owner and currently visible live peers that can see the target, skipping peers at the bootstrap `0`-HP floor,
- that `CREATE_FLY` does not mutate HP, cadence, retaliation, selection, points, inventory, or persistence, and it does not spend skill points or start a cooldown,
- visible peers receive no fly-effect frame from `USE_SKILL`; `SHOOT` queues only `CREATE_FLY` (never a targeting echo), without replacing `DAMAGE_INFO`, `TARGET`, or `DEAD`,
- an accepted same-selected-target client `ADD_FLY_TARGETING` emits exactly one self-only `GC ADD_FLY_TARGETING` before one self-only `GC CREATE_FLY`; unmatched or unsupported `ADD_FLY_TARGETING`, other `USE_SKILL` vnums, other `SHOOT` types, and non-lethal or rejected `ATTACK` stay fail-closed for fly emission,
- one accepted selected-target normal killing hit after an accepted `FLY_TARGETING` intent emits one self-only `CREATE_FLY` after its death/clear/damage prefix and before owned reward; non-lethal, unarmed and rejected hits emit none,
- later ranged/projectile/skill slices can start from this tested packet shape and the selected-target rule instead of re-discovering them.
