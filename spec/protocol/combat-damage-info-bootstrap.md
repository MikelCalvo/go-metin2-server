# Combat Damage Info Bootstrap

This note freezes the first server-to-client damage-info packet shape used by the current TMP4-compatible client as a hit-effect carrier.

It sits next to, but does not replace:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `player-death-bootstrap.md`

## Scope

This slice owns the packet codec, plus the internal damage descriptor needed before a later attack-flow slice can emit the packet without recomputing damage outside the authoritative shared-world attack attempt.

The packet is:
- name: `DAMAGE_INFO`
- direction: server -> client
- phase: `GAME`
- header: `0x0410`
- payload length: `9`
- status: documented and codec-owned in `internal/proto/combat`

The payload layout is:
1. `uint32 vid` (little-endian)
2. `uint8 flag`
3. `int32 damage` (little-endian)

The current client-side rendering surface treats `vid` as the actor receiving the visible damage effect. A non-negative `damage` value is eligible for the normal damage-effect display path. The first Go slice deliberately keeps `flag` as an owned raw byte: `0` means the plain bootstrap damage style, while critical, block, miss, poison, and other flag semantics remain future work until a dedicated slice freezes them.

## Relationship to current attack flow

The current accepted normal-attack runtime still uses `GC TARGET(target_vid, hp_percent)` as the authoritative HP refresh and switches to `GC DEAD(vid)` plus `GC TARGET(0, 0)` at the zero-HP edge.

`DAMAGE_INFO` is now codec-owned so a later combat slice can add a visible hit-effect companion without guessing the wire layout, but the current runtime still does not emit it. The normal-attack shared-world attempt now carries the owned non-negative damage descriptor derived from the same combat-profile formula that mutates HP, so packet emission can later reuse that value without recomputing combat state outside the authoritative attack seam.

The current client-visible response contract is still unchanged. In particular:
- non-lethal hits are still authoritative through the selected-target HP refresh,
- killing hits still use the existing death + clear-target choreography,
- no peer fanout, damage-number policy, or hit-result gameplay semantics are owned here.

## Non-goals

This slice does not freeze:
- exact damage formulas beyond the existing bootstrap combat-profile HP mutation rules,
- critical, miss, block, poison, or special flag meanings,
- player-vs-player damage info,
- skill damage, projectile damage, or multi-target damage,
- whether every accepted hit should emit a damage info packet,
- whether peers should receive damage info fanout,
- any replacement for `TARGET` as the current HP percentage carrier.

## Success definition

After this slice:
- `DAMAGE_INFO` is listed in the packet matrix as a codec-owned server combat packet,
- `internal/proto/combat` can encode and decode the exact fixed-width payload,
- malformed or wrong-header frames fail closed at the codec layer,
- the shared-world normal-attack attempt exposes the applied bootstrap damage amount as an internal descriptor,
- later runtime slices can add packet emission with focused attack-flow tests instead of re-discovering the packet layout or recomputing damage outside the authoritative attack seam.
