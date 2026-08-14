# Character Position and Change Speed Bootstrap

This note freezes the first owned stance/position presentation path and the server movement-speed refresh codec for `go-metin2-server`.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `player-stun-bootstrap.md`
- `visible-world-bootstrap.md`

## Scope

This slice owns the fixed server-to-client packet shapes plus the first deliberately narrow runtime emission policy for client-originated sit/stand requests.

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

The current Go runtime accepts only the smallest legacy-compatible stance presentation subset on client `CHARACTER_POSITION` (`0x0A60`) ingress:
- `position = 0` (`POSITION_GENERAL`) is treated as a stand/general presentation request,
- `position = 3` (`POSITION_SITTING_CHAIR`) is accepted as a conservative sit request and normalized onto the currently owned ground-sit presentation,
- `position = 4` (`POSITION_SITTING_GROUND`) is treated as a ground-sit presentation request,
- any other position byte, including the battle-position byte, still fails closed with no response.

When accepted in `GAME`, the socket must still own its live shared-world session, and the selected live character must exist and be above the bootstrap `0`-HP floor. The runtime emits exactly one self `GC CHARACTER_POSITION(selected_vid, position)` frame and queues the same frame to currently visible live peers through the existing shared-world visibility seam. Chair and ground-sit requests both publish `position = 4` because the first clean-room server-owned rendering policy deliberately avoids chair placement/chair-object semantics and reuses the already-visible ground-sit carrier.

The session starts in the general/standing presentation state. A request that repeats the already active presentation state is accepted as a no-op and emits no self or peer frames. This mirrors the observed standup/sitdown guard shape from the behavior oracle and avoids stale duplicate stance spam while still letting a later opposite transition publish the expected update.

This is presentation-only for now:
- it does not persist stance to account or content snapshots,
- it does not mutate selected combat target, runtime HP, normal-attack cadence, retaliation timers, inventory, points, or static/non-player actor state,
- accepted normal attacks continue to use `TARGET`, optional `DAMAGE_INFO`, `PLAYER_POINT_CHANGE`, `DEAD`, and target clear according to the combat docs,
- movement and sync continue to use the existing move/sync acknowledgement and peer fanout families,
- player death/restart and non-player death/respawn do not add extra stance packets unless a later slice freezes that companion.

The current Go runtime still does not emit `CHANGE_SPEED`. Unsupported battle-mode, speed-buff, slow, haste, stun, knockdown, skill, or AI effects stay out of scope until later tests freeze a concrete runtime policy.

## Non-goals

This slice does not freeze:
- the semantic meaning of every `position` byte beyond `0`, `3`, and `4`,
- persisted stance state,
- battle-mode transitions,
- speed formulas, buffs, debuffs, equipment speed effects, or mob chase/leash speed behavior,
- compatibility-grade choreography around stun, knockdown, skills, or projectile combat.

## Success definition

After this slice:
- `internal/proto/world` can encode and decode `GC CHARACTER_POSITION(vid, position)` exactly,
- `internal/proto/world` can encode and decode `GC CHANGE_SPEED(vid, moving_speed)` exactly,
- wrong-header and invalid-payload frames fail closed at the codec layer,
- accepted client `CHARACTER_POSITION(position=0|4)` emits self + visible-peer `GC CHARACTER_POSITION(selected_vid, position)` without mutating combat state,
- accepted client `CHARACTER_POSITION(position=3)` emits the same ground-sit presentation as `position=4`,
- duplicate stand/sit requests are accepted no-ops with no repeated presentation frame,
- unsupported position bytes still fail closed through the existing combat/targeting ingress guard,
- `CHANGE_SPEED` remains documented but currently non-emitted by the bootstrap runtime,
- later movement/combat presentation slices can start from tested packet shapes rather than guessing these layouts.
