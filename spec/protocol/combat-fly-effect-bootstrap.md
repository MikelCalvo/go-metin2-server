# Combat Fly-Effect Bootstrap

This note freezes the first owned server fly-effect packet shapes for `go-metin2-server` without yet adding runtime projectile or skill gameplay.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `combat-damage-info-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`

## Scope

This slice owns only three fixed server-to-client packet codecs and the current non-emission rule.

The packets are:

### `FLY_TARGETING`

- direction: server -> client
- phase: `GAME`
- header: `0x0411`
- payload length: `16`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `uint32 shooter_vid` (little-endian)
2. `uint32 target_vid` (little-endian)
3. `int32 x` (little-endian)
4. `int32 y` (little-endian)

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

## Clean-room evidence summary

Client-source inspection of the TMP4-compatible client shows separate game-phase receive handlers for server-originated fly effects:
- `FLY_TARGETING` and `ADD_FLY_TARGETING` both identify a shooter, an optional target, and fallback world coordinates.
- `CREATE_FLY` identifies a fly/effect type plus start and end actor VIDs.

The same source also shows client-originated `FLY_TARGETING` / `ADD_FLY_TARGETING` and `SHOOT` requests from bow-style event handlers. Those client packets are already owned as safe ingress guards in the current bootstrap runtime. This note freezes the matching server presentation packet shapes only; it does not make the runtime emit them.

## Relationship to current combat slices

Current accepted normal attacks still use the already-owned combat presentation surfaces:
- non-lethal hits use `TARGET(target_vid, hp_percent)` plus `DAMAGE_INFO` according to `combat-damage-info-bootstrap.md`,
- killing hits use `DEAD(vid)` plus `TARGET(0, 0)` before any owned reward feedback,
- content practice-mob retaliation continues to use `PLAYER_POINT_CHANGE` and the current delayed server-frame cadence.

The Go runtime does not currently emit `FLY_TARGETING`, `ADD_FLY_TARGETING`, or `CREATE_FLY` from `ATTACK`, `SHOOT`, `USE_SKILL`, or any mob-retaliation path. Any later accepted projectile, bow, or skill slice must add its own transcript-level tests before these codecs become runtime-visible behavior.

## Non-goals

This slice does not freeze:
- accepted ranged `SHOOT` gameplay,
- accepted skill combat,
- projectile hit timing or travel duration,
- visual effect type meanings,
- multi-target or chained projectile behavior,
- peer fanout policy for projectile effects,
- killing-hit fly effects,
- any replacement for `DAMAGE_INFO`, `TARGET`, or `DEAD` as the current combat result surfaces.

## Success definition

After this slice:
- `FLY_TARGETING`, `ADD_FLY_TARGETING`, and `CREATE_FLY` are listed in the packet matrix as documented server combat/fly-effect packet shapes,
- `internal/proto/combat` can encode and decode their exact fixed-width payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- later ranged/projectile/skill slices can start from tested packet shapes instead of re-discovering them while preserving the current no-runtime-emission rule.
