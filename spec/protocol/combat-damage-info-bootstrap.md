# Combat Damage Info Bootstrap

This note freezes the first server-to-client damage-info packet shape used by the current TMP4-compatible client as a hit-effect carrier.

It sits next to, but does not replace:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `player-death-bootstrap.md`

## Scope

This slice owns the packet codec, the internal damage descriptor, and the first self-only runtime emission for accepted non-lethal normal attacks.

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

For the first runtime emission slice, an accepted non-lethal normal attack against a standalone bootstrap `training_dummy` now returns one self-only `DAMAGE_INFO` frame immediately after the authoritative `GC TARGET(target_vid, hp_percent)` refresh:
1. `GC TARGET(target_vid, updated_hp_percent)`
2. `GC DAMAGE_INFO(vid = target_vid, flag = 0, damage = applied_bootstrap_damage)`

The `damage` value comes from the authoritative shared-world attack attempt, which already derives it from the same combat-profile formula that mutates runtime HP. The session/runtime layer must not recompute the number independently when encoding the hit-effect companion.

Killing hits deliberately do **not** append `DAMAGE_INFO` in this slice. They keep the existing death-first choreography:
1. `GC DEAD(vid)`
2. `GC TARGET(0, 0)` for the attacking session when that target was still selected
3. any owned reward feedback after the death/clear pair

The current client-visible response contract is therefore still conservative:
- standalone bootstrap `training_dummy` non-lethal hits are authoritative through the selected-target HP refresh and now carry one self-only hit-effect companion,
- killing hits still use the existing death + clear-target choreography without a synthetic final damage-info frame,
- no peer fanout, critical/miss flag policy, or broader hit-result gameplay semantics are owned here.

## Non-goals

This slice does not freeze:
- exact damage formulas beyond the existing bootstrap combat-profile HP mutation rules,
- critical, miss, block, poison, or special flag meanings,
- player-vs-player damage info,
- skill damage, projectile damage, or multi-target damage,
- whether spawn-backed practice mobs, registered-profile actors, killing hits, peer viewers, skills, or player-vs-player hits should emit a damage info packet,
- whether peers should receive damage info fanout,
- any replacement for `TARGET` as the current HP percentage carrier.

## Success definition

After this slice:
- `DAMAGE_INFO` is listed in the packet matrix as a documented server combat packet,
- `internal/proto/combat` can encode and decode the exact fixed-width payload,
- malformed or wrong-header frames fail closed at the codec layer,
- the shared-world normal-attack attempt exposes the applied bootstrap damage amount as an internal descriptor,
- accepted standalone bootstrap `training_dummy` non-lethal normal attacks append one self-only plain-flag `DAMAGE_INFO` frame after the `TARGET` HP refresh,
- later runtime slices can broaden peer fanout, flag meanings, killing-hit presentation, or other hit-effect policy without re-discovering the packet layout or recomputing damage outside the authoritative attack seam.
