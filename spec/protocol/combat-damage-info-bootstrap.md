# Combat Damage Info Bootstrap

This note freezes the first server-to-client damage-info packet shape used by the current TMP4-compatible client as a hit-effect carrier.

It sits next to, but does not replace:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `player-death-bootstrap.md`

## Scope

This slice owns the packet codec, the internal damage descriptor, visible-peer fanout for standalone bootstrap combat-profile actors, the first self plus visible-peer emission for content-loaded spawn-backed practice-mob hits, and the owner self plus visible-peer hit-effect companion for non-floor practice-mob retaliation beats.

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

For standalone bootstrap combat-profile actors, an accepted non-lethal normal attack returns one self `DAMAGE_INFO` frame immediately after the authoritative self `GC TARGET(target_vid, hp_percent)` refresh:
1. `GC TARGET(target_vid, updated_hp_percent)`
2. `GC DAMAGE_INFO(vid = target_vid, flag = 0, damage = applied_bootstrap_damage)`

The same plain `DAMAGE_INFO` packet is also queued to currently visible live peer sessions for that standalone actor. This first peer path is intentionally smaller than a full combat-result fanout: peers receive only the hit-effect companion, not the attacker's self-only `TARGET` HP refresh, and connected recipients already at the bootstrap `0`-HP floor are skipped by the shared-world visibility gate.

The `damage` value comes from the authoritative shared-world attack attempt, which already derives it from the same combat-profile formula that mutates runtime HP. The session/runtime layer must not recompute the number independently when encoding either the self hit-effect companion or the peer queued copy. The standalone emission set is intentionally bounded to actors with no `spawn_group_ref` whose `combat_profile` resolves through the bootstrap combat-profile registry: built-in `training_dummy`, built-in `practice_mob`, and custom registered formula profiles.

Content-loaded `spawn_groups` practice mobs now use the same hit-effect packet for both the attacking owner socket and currently visible live peer sockets. On an accepted non-lethal owner hit where the owner survives the immediate retaliation tick, the existing spawn-backed self ordering stays stable and appends the mob hit-effect companion after the already-owned retaliation point-change, then one owner retaliation hit-effect companion:
1. `GC TARGET(target_vid, updated_hp_percent)`
2. `GC PLAYER_POINT_CHANGE(owner_vid, HP, retaliation_delta, updated_owner_hp)` for the current retaliation slice
3. `GC DAMAGE_INFO(vid = target_vid, flag = 0, damage = applied_bootstrap_damage)`
4. `GC DAMAGE_INFO(vid = owner_vid, flag = 0, damage = abs(retaliation_delta))`

The same spawn-backed mob `DAMAGE_INFO` frame and the matching owner retaliation `DAMAGE_INFO` frame are also queued to currently visible live peer sessions after the attacker's runtime-only retaliation point mutation commits. Peers receive those two hit-effect companions only; they do not receive the attacker's self-only `TARGET` refresh or retaliation `PLAYER_POINT_CHANGE`. Connected recipients already at the bootstrap `0`-HP floor remain skipped by the shared-world visibility gate.

That owner-side spawn-backed ordering is now also frozen across the plain legacy TCP listener for the first accepted non-lethal practice-mob hit, so socket-level regressions must preserve `TARGET` -> `PLAYER_POINT_CHANGE` -> mob `DAMAGE_INFO` -> owner `DAMAGE_INFO` on the attacking connection rather than proving the hit effect only through direct runtime calls.

Delayed server-origin retaliation beats that leave the owner above the bootstrap `0`-HP floor reuse the same owner companion after their ordinary point-change and queue the same owner `DAMAGE_INFO` to currently visible live peers:
1. `GC PLAYER_POINT_CHANGE(owner_vid, HP, retaliation_delta, updated_owner_hp)`
2. `GC DAMAGE_INFO(vid = owner_vid, flag = 0, damage = abs(retaliation_delta))`

That delayed owner companion applies to both hit-armed and proximity-armed delayed beats. Peers still do not receive the owner's delayed `PLAYER_POINT_CHANGE`; they receive only the owner retaliation `DAMAGE_INFO` companion. Focused shared-world coverage now freezes that proximity-armed peer path explicitly: walking into aggro radius without selecting or hitting still arms the delayed cadence, the engaged owner keeps no selected combat target, and a currently visible live peer receives only the owner `DAMAGE_INFO` companion when each non-floor delayed beat fires.

Killing hits and owner-floor retaliation beats deliberately do **not** append `DAMAGE_INFO` in this slice. Mob killing hits keep the existing death-first choreography:
1. `GC DEAD(vid)`
2. `GC TARGET(0, 0)` for the attacking session when that target was still selected
3. any owned reward feedback after the death/clear pair

Owner-floor immediate or delayed retaliation beats keep the owned player-death ordering without a synthetic final owner damage-info frame:
1. `GC PLAYER_POINT_CHANGE(owner_vid, HP, final_clamped_delta, value = 0)`
2. `GC DEAD(owner_vid)`
3. `GC TARGET(0, 0)`

The current client-visible response contract is therefore still conservative:
- standalone bootstrap combat-profile non-lethal hits are authoritative through the selected-target HP refresh and carry one self hit-effect companion,
- killing hits still use the existing death + clear-target choreography without a synthetic final damage-info frame,
- visible live peers now receive the same standalone or spawn-backed mob hit-effect companion through the queued server-frame path,
- content-loaded spawn-backed practice mobs now append one owner self mob hit-effect companion plus one owner self retaliation hit-effect companion on accepted non-lethal owner-surviving hits after the existing target refresh plus retaliation point-change, and queue both companions to currently visible live peers,
- non-floor delayed retaliation beats now also append one owner self retaliation hit-effect companion after their point-change and queue that same owner companion to currently visible live peers,
- no killing-hit damage-info, owner-floor damage-info, critical/miss flag policy, or broader hit-result gameplay semantics are owned here.

## Non-goals

This slice does not freeze:
- exact damage formulas beyond the existing bootstrap combat-profile HP mutation rules,
- critical, miss, block, poison, or special flag meanings,
- player-vs-player damage info,
- skill damage, projectile damage, or multi-target damage,
- whether killing hits, owner-floor retaliation beats, skills, or player-vs-player hits should emit a damage info packet,
- whether peer damage-info fanout should widen beyond currently visible live peers for standalone or spawn-backed bootstrap combat-profile actors,
- any replacement for `TARGET` as the current HP percentage carrier.

## Success definition

After this slice:
- `DAMAGE_INFO` is listed in the packet matrix as a documented server combat packet,
- `internal/proto/combat` can encode and decode the exact fixed-width payload,
- malformed or wrong-header frames fail closed at the codec layer,
- the shared-world normal-attack attempt exposes the applied bootstrap damage amount as an internal descriptor,
- accepted standalone bootstrap combat-profile non-lethal normal attacks append one self plain-flag `DAMAGE_INFO` frame after the `TARGET` HP refresh, using the authoritative attack/defense-derived damage descriptor for registered formula profiles,
- currently visible live peers receive that same standalone plain-flag `DAMAGE_INFO` through the queued server-frame path without receiving the attacker's self-only target refresh,
- accepted spawn-backed practice-mob non-lethal normal attacks now append one owner self plain-flag mob `DAMAGE_INFO` after the existing immediate retaliation point-change when that owner remains alive, then one owner self plain-flag retaliation `DAMAGE_INFO(owner_vid, abs(delta))`, queue both the mob and owner retaliation plain-flag hit effects to currently visible live peers, and preserve that owner-side frame order over the plain legacy TCP listener,
- non-floor delayed server-origin retaliation beats now append the same owner self plain-flag retaliation `DAMAGE_INFO` after their point-change and queue that same owner companion to currently visible live peers for both hit-armed and proximity-armed delayed cadences,
- later runtime slices can broaden flag meanings, killing-hit or floor presentation, or other hit-effect policy without re-discovering the packet layout or recomputing damage outside the authoritative attack seam.
