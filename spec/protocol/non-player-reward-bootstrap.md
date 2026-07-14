# Non-Player Reward Bootstrap

This document freezes the first deliberately narrow reward contract for bootstrap non-player combatants in `go-metin2-server`.

It sits on top of:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `content-spawn-groups-bootstrap.md`
- `player-point-change-bootstrap.md`

Those documents already freeze:
- the selected-target `ATTACK` loop for `training_dummy` / `practice_mob` combatants
- the zero-HP `DEAD(vid)` plus `TARGET(0, 0)` death edge
- authored `spawn_groups` with optional reward descriptor fields
- the existing self-only point-change and ground-item packet families reused here

## Scope

This contract currently applies only to an accepted killing hit against a runtime-owned bootstrap non-player combatant whose death edge is already committed.

The current reward descriptor is intentionally tiny:
- `reward_experience uint64`
- `reward_gold uint64`
- `reward_drop_vnums []uint32`

It does **not** yet claim:
- level-up choreography or stat recalculation
- party reward sharing or contribution splits
- randomized loot tables or probabilities
- corpse interaction, pickup ownership expiry, or public loot release
- quest credit, achievements, or scripted on-death hooks
- persisted non-player reward state across process restart

## Descriptor ownership

Reward descriptors can come from authored `spawn_groups` or from the live static-actor snapshot that was materialized from that authored content.
Standalone static actors that are not backed by a non-empty `spawn_group_ref` must remain rewardless; both the static snapshot store and the in-memory non-player directory reject reward metadata on those actors so rewards do not become a generic static-actor feature by accident.

The descriptor is not character persistence by itself.
It becomes player state only after the already-accepted death edge tries to apply the reward through the selected live session.

Default bootstrap `training_dummy` and `practice_mob` profile defaults remain rewardless unless content or tests explicitly author a non-zero descriptor.
Both profile-level default helpers are covered directly so a later combat-profile broadening cannot accidentally turn either bootstrap profile into an implicit reward source.

Registered bootstrap combat profiles may carry a validated death-reward default for authored spawn-group use, and lookup returns cloned descriptor slices so callers cannot mutate the registry by editing returned defaults.
Both full combat-profile default lookup and reward-only lookup return isolated reward descriptors; mutating either returned value must not alter later profile resolutions.
Runtime coverage freezes that a spawn-backed actor using such a registered profile can apply the profile's default EXP/gold/drop descriptor on the accepted killing hit.
That includes a drop-only profile default case where the killing hit emits the normal death/clear frames followed by one `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` pair and registers the deterministic reward ground item.
That runtime coverage now also includes formula-only registered profiles whose `damage_per_normal_attack` is canonicalized from `attack_value - defense_value`, so the profile can both drive deterministic HP mutation and apply its reward descriptor on the same death edge.
A formula-only registered profile may carry a combined descriptor as well; the killing hit still uses the formula-derived HP mutation first, then emits death/clear, scalar EXP/gold point changes, and deterministic owned drop frames in the normal reward ordering.
If the authored spawn-group snapshot also carries an explicit reward descriptor, that authored descriptor wins over the profile-level default for the current actor life; profile defaults are only the fallback for spawn-backed actors whose live/authored snapshot has no explicit reward descriptor.
Dedicated coverage freezes this precedence and clone isolation so a registered profile default cannot silently replace an authored EXP/gold/drop override.
That profile-level default still does **not** make standalone runtime static actors reward-bearing: without a non-empty `spawn_group_ref` or explicit authored live snapshot descriptor, the shared-world death attempt returns a rewardless descriptor.

## Validation

Reward descriptors fail closed when:
- `reward_experience` or `reward_gold` exceeds the current signed 32-bit `PLAYER_POINT_CHANGE` carrier range (`2,147,483,647`)
- applying `reward_experience` or `reward_gold` would overflow the selected character's current signed 32-bit visible point / gold carrier
- any `reward_drop_vnums` entry is `0`
- any `reward_drop_vnums` entry is duplicated in the same descriptor
- a runtime-generated ground-entry VID for a configured item-shaped or gold-shaped drop would be `0`
- the reward owner login is empty, blank after trimming Unicode whitespace, padded with leading/trailing whitespace, or contains embedded whitespace when registering a configured item-shaped or gold-shaped drop
- the reward owner character name is empty, blank after trimming Unicode whitespace, padded with leading/trailing whitespace, or contains embedded whitespace when registering a configured item-shaped or gold-shaped drop
- a runtime-generated ground-item instance for a configured item-shaped drop would have `vnum = 0`
- a runtime-generated ground-item instance for a configured item-shaped drop would have zero count
- a runtime-generated ground-item instance for a configured item-shaped drop would exceed the current `GC ITEM_GET` count carrier (`255`)
- a runtime-generated ground-item instance for a configured item-shaped drop would carry equipped/locked state or stale equipment-slot metadata
- multiple drops in the same descriptor would collide on the generated ground-item VID
- a configured drop would reuse an already-live ground-item VID

A live ground-item VID collision is intentionally treated as a per-drop failure, not as a combat rollback: the accepted killing hit still emits `DEAD(target_vid)` and the killer's `TARGET(0, 0)` clear, the pre-existing ground item remains registered, and the colliding reward drop emits no `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` frames.
If the same descriptor also carries valid EXP, gold, or other non-colliding drops, those independent reward families still apply and emit their ordinary frames; one colliding drop does not suppress independent scalar rewards or later non-colliding drop entries.

The descriptor validator itself owns the static authoring checks before runtime kill handling begins:
- signed 32-bit scalar carrier maximums are accepted
- scalar values above that maximum are rejected
- zero-valued drop vnums are rejected
- duplicate drop vnums in one descriptor are rejected
- multiple distinct fixed drop vnums in one descriptor are accepted
- validated drop-vnum lists are normalized into ascending deterministic order and deduplicated when cloned into runtime/default snapshots, so later lookup, update, respawn, and preview paths do not depend on authored JSON/list ordering or caller-side duplicate entries
- registered combat-profile reward defaults are validated before clone normalization, so duplicate authored/default drop vnums still fail closed instead of being silently deduplicated during registration
- runtime reward overrides used by the bootstrap shared-world seam are also validated before replacing an existing descriptor; malformed overrides fail closed and leave the last valid descriptor intact
- file-backed static actor snapshots reject malformed spawn-group reward descriptors before loading or saving runtime state

Malformed reward descriptors must not roll back the already-accepted combat death edge.
They simply suppress reward mutation and reward frames for that kill.

## Killing-hit ordering

The killing hit keeps death choreography first.
Reward frames are appended only after:
1. `GC DEAD(target_vid)`
2. `GC TARGET(0, 0)` for the killer if that target was still selected

After those frames, successful reward feedback is ordered as:
1. optional EXP `GC PLAYER_POINT_CHANGE`
2. optional gold `GC PLAYER_POINT_CHANGE`
3. one `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` pair per configured drop, in normalized ascending drop-vnum order

This ordering keeps combat lifecycle visible before reward side effects.

## Scalar EXP and gold rewards

EXP and gold rewards reuse `GC PLAYER_POINT_CHANGE` as the visible self-only carrier.

Current rules:
- EXP uses the bootstrap experience point type already used by selected-character points
- gold uses the bootstrap gold point type
- each scalar reward is applied to the selected live player runtime first
- the updated selected-character account snapshot is saved before the corresponding point-change frame is appended
- if the player runtime rejects the scalar reward because the descriptor or resulting live values would overflow the signed 32-bit visible carriers, the accepted death/clear frames remain, scalar reward frames are omitted, the live EXP/gold scalar values stay at their pre-reward values, and independent valid drop rewards still continue through their normal ground-add / ownership path
- dedicated runtime coverage freezes this independent-drop behavior for both overflowing EXP and overflowing gold rewards so either scalar channel can fail closed without suppressing valid item-shaped drop feedback
- if account persistence fails after a scalar reward was tentatively applied, the accepted death/clear frames remain, scalar reward frames are omitted, and the live EXP/gold scalar values roll back to their pre-reward values; other live runtime state such as the current in-world position must not be clobbered

## Drop rewards

Drop rewards reuse the existing bootstrap ground-item families:
- `ITEM_GROUND_ADD`
- `ITEM_OWNERSHIP`
- later pickup still uses the normal owned `ITEM_PICKUP` path, producing `ITEM_GROUND_DEL`, `ITEM_SET` for item-shaped entries, gold `PLAYER_POINT_CHANGE` for gold-shaped entries, and `ITEM_GET` confirmation as appropriate

Ground reward registration is guarded by the same live-owner rule for both item-shaped and gold-shaped ground entries.
If the would-be owner is already at the current bootstrap `0` HP floor, registering either a ground item or ground gold fails closed and leaves no live ground occupancy behind.
The guard checks the current registered owner snapshot, not only the caller-supplied character snapshot, so a stale pre-death owner copy cannot create item-shaped or gold-shaped ground rewards after that owner has already reached the bootstrap `0`-HP floor.
Ground reward visibility fanout and lookup are also live-recipient guarded: dead visible peers do not receive `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` on registration, do not receive `ITEM_GROUND_DEL` when the ground entry is later removed, and cannot resolve pending ground handles through the visibility/pickup seam while they remain at the current bootstrap `0`-HP floor.
Transfer/rebootstrap destination visibility uses the same recipient gate for both item-shaped and gold-shaped entries: moving a zero-HP owner into a map/AOI with existing ground entries queues no destination `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` burst to that dead recipient, while the ground entry remains available to living visible sessions.
Ordinary session position updates that rebuild destination visibility use the same zero-HP gate for item-shaped and gold-shaped ground entries, so a dead recipient cannot reacquire ground-entry visibility by arriving through a movement/sync-style shared-world update path instead of the structured transfer path.
Fresh same-socket rebootstrap uses the same guard before building the selected character's trailing ground-entry burst, so a still-dead session cannot reacquire `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` visibility frames for pending ground item-shaped or gold-shaped rewards merely by re-entering the map while still at the bootstrap `0`-HP floor.
Operator/runtime relocation previews and map-occupancy helpers now treat appended ground-entry occupancy as a cloned result rather than mutating or aliasing their input occupancy snapshots, so preview composition can safely layer item-shaped or gold-shaped ground entries onto player/static-actor snapshots without leaking transient ground state back into the caller's baseline view even when no ground entries are appended.
The runtime also exposes the same pending ground occupancy as a flat loopback-only `/local/ground-items` debug snapshot, sorted by visible ground `vid`; this is an operator view only and does not add a gameplay pickup or mutation surface. Pending ground-entry snapshots include the display `owner_name` plus owner identity fields (`owner_login`, `owner_character_id`, `owner_vid`) so operator/debug consumers can inspect the same identity anchor used by the stale-pickup and stale-owner guards without relying on display text alone.
Ground pickup also skips party-style delivery back into an owner that has since reached the bootstrap `0` HP floor: the ground handle keeps its owner metadata for display/ownership, but the live owner snapshot is withheld so a living collector either takes the item through the ordinary collector path or fails closed on collector capacity without mutating the dead owner's inventory/gold.
Ground pickup also withholds party-style owner delivery if the registered owner entity no longer matches the identity and exact runtime location captured when the ground reward was created; this preserves the display ownership label while preventing stale handles from mutating a rebound or relocated owner snapshot.
Ground reward registration re-checks the registered owner snapshot before trusting the caller-supplied character copy, so a stale owner snapshot cannot create item-shaped or gold-shaped reward ground entries after the registered owner has already changed selected-character identity (`id`, `vid`, or name) or moved to a different runtime location.
Lookup, pickup-resolution, and removal also re-check the registered collector snapshot before trusting the caller-supplied character copy, so a stale collector snapshot cannot mutate item-shaped or gold-shaped reward ground entries after that collector has already reached the bootstrap `0`-HP floor, changed selected-character identity (`id`, `vid`, or name), changed the current bootstrap HP point value, or moved out of the exact runtime location represented by the supplied pickup attempt.
Dedicated coverage now freezes the item-shaped and gold-shaped reward halves of stale collector-death, stale collector-identity, stale collector-point, stale collector-location, stale owner-identity owner-delivery, and stale owner-location owner-delivery cases because both reward families share the same ground-entry pickup/removal seam.
For collector identity specifically, a stale collector snapshot fails closed for visibility lookup, pickup resolution, and ground-entry removal after the registered collector has rebound to a different selected-character identity (`id`, `vid`, or name); the ground entry remains available to a fresh valid collector/owner path.
For collector point state specifically, a stale collector snapshot still leaves read-only same-map ground visibility available, but pickup resolution and removal fail closed after the registered collector's bootstrap HP point changes, even if both snapshots are still positive and otherwise at the same location; the ground entry remains available to a fresh valid collector/owner path.
For collector location specifically, this guard applies in both directions for item-shaped and gold-shaped rewards: a stale near snapshot cannot pick up after the registered collector has moved away or a stale far snapshot cannot become valid merely because the registered collector later moved near the ground reward.

When that living, identity-current, and currently reachable collector succeeds, the collector receives the ordinary self pickup shape (`ITEM_GROUND_DEL`, inventory `ITEM_SET`, and normal `ITEM_GET` feedback) for item-shaped rewards. Gold-shaped self-pickup now mirrors that visible confirmation style as `ITEM_GROUND_DEL`, gold `PLAYER_POINT_CHANGE`, and normal `ITEM_GET(vnum=1,count=1)` so the client gets both the currency mutation and the same pickup-notice cue already used by ordinary item pickups. The dead, rebound, or relocated owner receives no queued party-style pickup feedback.

This keeps death/restart cleanup and concurrent movement from leaking new pickup surfaces, stale delete noise, debug/runtime-visible pickup affordances, stale collector pickup affordances, stale owner-location ground registration, transfer-entry ground visibility, or late owner-delivery mutations for players that are already dead or no longer at the supplied reward location.

Current rules:
- each configured drop spawns at the killer's current position
- each drop has count `1`
- each drop is owned by the killer's character name
- item drops are runtime ground items first; they do not mutate persisted inventory until an explicit pickup succeeds
- the killer receives the drop's `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` pair inline after the killing-hit death and scalar-reward frames
- currently visible, live peers receive the same ground-add / ownership pair through the queued server-frame path after the already-owned `DEAD(target_vid)` visibility notification
- currently visible peers that are already at the bootstrap `0`-HP floor receive neither the non-player `DEAD(target_vid)` fanout nor the reward ground-add / ownership pair for that kill; the killer still receives the ordinary self-visible reward frames
- replayed pickup of the same ground VID fails closed after the first successful pickup removes it
- ground pickup removal now uses the same bootstrap reachability gate as pickup lookup: a collector who can see the ground reward but is outside the current pickup radius cannot remove it, and the ground entry remains available to reachable living sessions
- a successful collector pickup now queues the `ITEM_GROUND_DEL` cleanup to the collector's own socket as well as to other living visible peers, so loopback/runtime visibility and client-visible self state agree immediately after pickup
- when a connected owner leaves the shared world, currently owned reward ground entries are removed with deterministic `ITEM_GROUND_DEL` fanout to living peers that can still see the ground entry's original map/AOI location; connected peers already at the bootstrap `0`-HP floor are skipped for both the owner leave visibility delete and owned-ground delete noise
- gold-shaped ground rewards use the same live ground-entry lifecycle as item-shaped drops: empty or blank owner-login metadata, zero amounts, and amounts above the signed 32-bit `PLAYER_POINT_CHANGE` carrier range fail closed before registration, duplicate `VID` registration fails closed without replacing the original live amount or ground handle, and removal queues the same `ITEM_GROUND_DEL` visibility cleanup to living visible peers

## Respawn / lifecycle ownership

Reward descriptors belong to the authored spawn snapshot, not to the transient live HP/dead state.

Current rules:
- killing a rewarded practice mob may apply that descriptor once for the accepted death edge
- the timed respawn rebuild restores the same authored actor identity and preserves `spawn_group_ref`, `reward_experience`, `reward_gold`, and `reward_drop_vnums`
- preserving the descriptor across respawn does **not** mean rewards are automatically granted on respawn; the descriptor is only applied again after a later accepted killing hit in a fresh live loop
- runtime helpers that clone authored static-actor snapshots must deep-copy `reward_drop_vnums` so update/remove/rollback paths cannot mutate the caller's descriptor slice by alias
- dead/live HP state remains runtime-owned and separate from account or content persistence

## Combined reward descriptor coverage

The current runtime test coverage explicitly freezes the combined descriptor case as one kill-side transaction:
- one accepted killing hit may carry EXP, gold, and fixed drop-vnum entries together
- the self-visible frame order stays death/clear first, then EXP point-change, gold point-change, ground-add, and ownership
- the scalar EXP/gold account snapshot is saved before those scalar point-change frames are emitted
- the player runtime's scalar reward helper intentionally ignores `reward_drop_vnums` while applying EXP/gold, so combined descriptors are not rejected merely because a separate drop channel is present
- the drop is registered as a runtime ground item after the same accepted kill and remains non-persistent until pickup

This regression coverage matters because scalar persistence and drop registration use different runtime seams.
A combined descriptor must not accidentally suppress one reward family just because the other family is present.

## Success definition

The repository can now say:
- non-player rewards are a documented bootstrap seam rather than an implied future system
- default bootstrap combatants are still rewardless
- authored spawn groups may carry deterministic EXP, gold, and fixed drop-vnum descriptors
- registered formula-only combat profiles can drive both deterministic HP mutation and profile-default EXP/gold reward payout on the same accepted death edge
- a single accepted kill can emit EXP, gold, and owned drop feedback together in documented order
- accepted non-player death is preserved even when reward application fails
- scalar rewards persist before their point-change frames are emitted
- item drops become owned ground items and persist to inventory only through the normal pickup path
- timed respawn rebuild preserves authored reward descriptor metadata so later kills continue to use the same content contract

Broader reward, loot-table, party, quest, and level-up systems remain future work.
