# Player Stun Bootstrap

This note freezes the first owned server `STUN` packet shape for `go-metin2-server` without yet adding runtime stun gameplay.

It sits next to:
- `player-death-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`

## Scope

This slice owns only the fixed server-to-client packet codec and the current non-emission rule.

The packet is:
- name: `STUN`
- direction: server -> client
- phase: `GAME`
- header: `0x0216`
- payload length: `4`
- status: documented and codec-owned in `internal/proto/world`

The payload layout is:
1. `uint32 vid` (little-endian)

The `vid` identifies the visible actor whose stun state should be presented by the client. The current Go runtime does not emit this packet yet.

## Clean-room evidence summary

The current TMP4-compatible client registers `GC::STUN` in the game-phase handler table and reads one `uint32 vid` field before applying a stun presentation to the matching visible actor. The legacy behavior oracle also uses the same compact `{header, length, vid}` shape when a character enters the stunned state.

This repository keeps that finding in project-owned terms and freezes only the packet shape needed by future gameplay work.

## Relationship to current death and combat slices

`STUN` is deliberately separate from the death surfaces already owned today:
- player or non-player zero-HP edges continue to use `GC DEAD(vid)` plus the documented target-clear / reward / restart companions,
- accepted non-lethal practice-mob hits continue to use `TARGET`, `PLAYER_POINT_CHANGE`, and `DAMAGE_INFO` according to the current combat docs,
- no current attack, retaliation, death, respawn, or restart path emits `STUN` as an extra companion.

This prevents a later stun/knockdown slice from guessing the wire layout while also preventing this codec slice from changing existing gameplay choreography.

## Non-goals

This slice does not freeze:
- stun chances, formulas, duration, or recovery timers,
- mob or player skill effects that cause stun,
- knockdown / standing-up choreography,
- interaction with player-death or non-player-death transitions,
- peer fanout policy beyond the future rule that any emitted stun must reference a currently visible actor.

## Success definition

After this slice:
- `STUN` is listed in the packet matrix as documented but not currently emitted by the bootstrap runtime,
- `internal/proto/world` can encode and decode `GC STUN(vid)` exactly,
- malformed or wrong-header frames fail closed at the codec layer,
- later combat/stun slices can start from a tested packet shape instead of re-discovering it.
