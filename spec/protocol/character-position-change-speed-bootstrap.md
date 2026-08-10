# Character Position and Change Speed Bootstrap

This note freezes the first owned server-side presentation codecs for character stance/position and movement-speed refreshes without adding runtime emission policy yet.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `player-stun-bootstrap.md`
- `visible-world-bootstrap.md`

## Scope

This slice owns only the fixed server-to-client packet shapes and the current non-emission rule.

The first packet is:
- name: `CHARACTER_POSITION`
- direction: server -> client
- phase: `GAME`
- header: `0x020B`
- payload length: `5`
- status: documented and codec-owned in `internal/proto/world`

Payload layout:
1. `uint32 vid` (little-endian)
2. `uint8 position`

The second packet is:
- name: `CHANGE_SPEED`
- direction: server -> client
- phase: `GAME`
- header: `0x0218`
- payload length: `6`
- status: documented and codec-owned in `internal/proto/world`

Payload layout:
1. `uint32 vid` (little-endian)
2. `uint16 moving_speed` (little-endian)

## Relationship to existing packet families

The server `CHARACTER_POSITION` packet in this note is deliberately separate from the client-originated `CHARACTER_POSITION` guard documented in `combat-normal-attack-bootstrap.md`:
- client -> server `CHARACTER_POSITION` uses header `0x0A60` and a one-byte position payload,
- server -> client `CHARACTER_POSITION` uses header `0x020B` and names the visible actor `vid` plus the position byte.

`CHANGE_SPEED` is likewise a compact presentation refresh for one visible actor's moving speed. The current movement path remains owned by `MOVE`, `SYNC_POSITION`, `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE`; no shipped bootstrap movement, combat, death, restart, or respawn path emits `CHANGE_SPEED` yet.

## Current runtime rule

The current Go runtime does not emit either packet.

That means the existing owned flows remain unchanged:
- accepted normal attacks continue to use `TARGET`, optional `DAMAGE_INFO`, `PLAYER_POINT_CHANGE`, `DEAD`, and target clear according to the combat docs,
- movement and sync continue to use the existing move/sync acknowledgement and peer fanout families,
- player death/restart and non-player death/respawn do not add these packets as extra companions,
- unsupported stance, battle-mode, speed-buff, slow, haste, stun, knockdown, skill, or AI effects stay out of scope until later tests freeze a concrete runtime policy.

## Non-goals

This slice does not freeze:
- the semantic meaning of every `position` byte,
- sit/stand/battle-mode transitions,
- speed formulas, buffs, debuffs, equipment speed effects, or mob chase/leash speed behavior,
- peer fanout policy for future emitted stance or speed updates,
- compatibility-grade choreography around stun, knockdown, skills, or projectile combat.

## Success definition

After this slice:
- `internal/proto/world` can encode and decode `GC CHARACTER_POSITION(vid, position)` exactly,
- `internal/proto/world` can encode and decode `GC CHANGE_SPEED(vid, moving_speed)` exactly,
- wrong-header and invalid-payload frames fail closed at the codec layer,
- the packet matrix lists both families as documented but currently non-emitted by the bootstrap runtime,
- later movement/combat presentation slices can start from tested packet shapes rather than guessing these layouts.
