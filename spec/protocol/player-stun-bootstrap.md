# Player Stun Bootstrap

This note freezes the first owned server `STUN` packet shape for `go-metin2-server` and the first deliberately narrow runtime emission policy: one self-only presentation companion on an evidence-backed, skill-less sitting dummy hit.

It sits next to:
- `player-death-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `character-position-change-speed-bootstrap.md`

## Scope

This slice owns the fixed server-to-client packet codec plus the first sitting-hit emission rule.

The packet is:
- name: `STUN`
- direction: server -> client
- phase: `GAME`
- header: `0x0216`
- payload length: `4`
- status: documented and codec-owned in `internal/proto/world`

The payload layout is:
1. `uint32 vid` (little-endian)

The `vid` identifies the currently visible actor whose stun state should be presented by the client. This first GREEN always names the accepted dummy `target_vid`, never an invisible, unselected, or already-dead actor.

## Clean-room evidence summary

The current TMP4-compatible client registers `GC::STUN` in the game-phase handler table and reads one `uint32 vid` field before applying a stun presentation to the matching visible actor. The legacy behavior oracle also uses the same compact `{header, length, vid}` shape when a character enters the stunned state.

This repository keeps that finding in project-owned terms. The first runtime policy reuses that already-owned codec on one existing seam: an accepted non-lethal standalone dummy hit while the owner is already in the owned ground-sit presentation. It does not invent a stun-chance table, duration, recovery timer, knockdown, skill, or PvP/duel path.

## Current runtime rule

The shipped runtime now emits `STUN` only on this combined hit-plus-presentation seam:

- the session is already in `GAME` with a live selected character above the bootstrap `0`-HP floor
- that character currently holds the owned ground-sit presentation (`CHARACTER_POSITION` position `4`, including chair requests already normalized onto ground-sit)
- the request is an accepted non-lethal normal `ATTACK` against the currently selected standalone bootstrap combat-profile dummy (`training_dummy`, standalone `practice_mob`, or another registered standalone combat profile with no `spawn_group_ref`)
- the dummy is still a currently visible in-range combat target, so the ordinary `TARGET` refresh plus `DAMAGE_INFO` burst still fires first

On that accepted hit the owner socket already receives:

1. `GC TARGET(target_vid, updated_hp_percent)`
2. `GC DAMAGE_INFO(vid = target_vid, flag = 0, damage = applied_bootstrap_damage)`

The same accepted sitting hit then queues exactly one self-only `GC STUN(target_vid)` through the pending server-frame path. Visible peers keep the existing standalone `DAMAGE_INFO`-only fanout; they do not receive `STUN` in this first GREEN.

The companion is skill-less presentation, not a second combat simulation:

- it does not roll a stun chance, duration, or recovery timer
- it does not mutate selected-target HP beyond the already-owned dummy decrement
- it does not change normal-attack cadence, retaliation, death, respawn, restart, inventory, points, or persistence
- it does not emit knockdown, skill, projectile, or PvP/duel packets

Standing / general presentation hits stay on the previously owned `TARGET` + `DAMAGE_INFO` burst with no `STUN`. Duplicate sit/stand requests remain no-ops and still do not emit `STUN` by themselves. Cadence-denied repeats, spawn-backed practice-mob hits, killing hits, delayed/proximity retaliation, death, respawn, restart, `USE_SKILL`, `SHOOT`, fly targeting, and item/equipment paths still do not emit `STUN`.

## Relationship to current death and combat slices

`STUN` remains deliberately separate from the death surfaces already owned today:
- player or non-player zero-HP edges continue to use `GC DEAD(vid)` plus the documented target-clear / reward / restart companions,
- accepted non-lethal practice-mob and standing dummy hits continue to use `TARGET`, `PLAYER_POINT_CHANGE`, and `DAMAGE_INFO` according to the current combat docs,
- killing hits, retaliation, death, respawn, and restart still do not add a stun companion.

This first GREEN only adds the sitting standalone dummy-hit presentation companion. Later stun-chance, knockdown, or PvP slices must freeze their own policy instead of widening this seam by implication.

## Non-goals

This slice does not freeze:
- stun chances, formulas, duration, or recovery timers,
- mob or player skill effects that cause stun,
- knockdown / standing-up choreography,
- interaction with player-death or non-player-death transitions,
- peer fanout of `STUN` beyond the current self-only sitting-hit companion,
- PvP or duel stun presentation.

Any later emitted stun must still reference a currently visible actor.

## Success definition

After this slice:
- `internal/proto/world` can encode and decode `GC STUN(vid)` exactly,
- malformed or wrong-header frames fail closed at the codec layer,
- an accepted non-lethal standalone dummy hit while the owner is in the owned ground-sit presentation queues one self-only `GC STUN(target_vid)` after the ordinary `TARGET` + `DAMAGE_INFO` burst,
- that `vid` is the currently visible selected dummy, not the owner and not an invisible actor,
- visible peers still receive only the matching `DAMAGE_INFO` companion,
- standing hits, spawn-backed hits, killing hits, cadence-denied repeats, sit/stand presentation itself, death, respawn, restart, skill, and PvP still do not emit `STUN`,
- later combat/stun slices can start from this tested packet shape and this first sitting-hit emission rule instead of re-discovering them.
