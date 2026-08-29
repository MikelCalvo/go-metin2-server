# Combat Normal Attack Bootstrap

This document freezes the first owned attack-intent and clear-target contract for `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `shared-world-peer-visibility.md`
- `runtime-reconnect-cleanup.md`

Those documents already freeze:
- one visible `training_dummy` target class addressed by client-visible `VID`
- the first self-only `GC TARGET` acknowledgement for accepted target selection
- the current visibility/range/runtime ownership rules that decide whether a dummy can stay targetable at all
- the existing reconnect/reclaim cleanup style that later combat slices must reuse instead of inventing separate ownership semantics

What this document adds is the next narrower question:

**What is the smallest honest attack-intent step the project can own next without pretending that full damage, death, aggro, or mob AI already exist?**

The first owned death / respawn follow-up now lives in:
- `non-player-death-respawn-bootstrap.md`

The first owned owner-side zero-HP retaliation follow-up now lives in:
- `player-death-bootstrap.md`

The first deliberately narrow non-player reward seam now lives in:
- `non-player-reward-bootstrap.md`

## Scope

This contract currently applies only to:
- one connected `GAME` session with a selected live character
- one active selected combat target already accepted through the existing `TARGET` selection path
- one currently visible in-range non-player actor still marked as `training_dummy`
- one immediate attack-intent request against that already selected target
- one tiny target-refresh surface that can still describe `current target`, `updated hp percent`, or `no active target`
- one decode-and-fail-closed skill-intent guard so client `USE_SKILL` traffic cannot fall through as an unknown combat header
- one decode-and-fail-closed projectile targeting guard so client `FLY_TARGETING` / `ADD_FLY_TARGETING` traffic cannot fall through as unknown combat headers
- one decode-owned `ON_CLICK` ingress that fail-closes unsupported click targets while also owning guest private-shop browse open against an already-open peer MYSHOP
- one narrow character-position ingress seam so client `CHARACTER_POSITION(position=0|3|4)` traffic can drive the first self/peer stance presentation while unsupported/battle-position bytes still fail closed instead of falling through as unknown target/UI headers
- one read-only runtime snapshot of the session's current selected combat target for local/debug surfaces

This contract does **not** yet claim:
- the full gameplay meaning of every non-zero `attack_type` value beyond the first narrow bootstrap ownership boundary
- combo chains, animation timing, attack speed, or projectile choreography
- richer attack-result packets, hit effects, floating damage numbers, or accepted skill systems
- combat against player targets
- aggro, retaliation, patrol, or movement AI
- broader reward systems beyond the narrow `non-player-reward-bootstrap.md` descriptor seam
- corpse state, quest hooks, party distribution, loot ownership expiry, or level progression
- final persistence rules for non-player combat state

## Why freeze attack intent before full combat

The repository already has a real target-selection slice:
- `TARGET` can now select one visible in-range `training_dummy`
- that selection already reuses shared-world visibility, map, and ownership rules
- the runtime already knows how to reject invisible, out-of-range, stale, or non-targetable candidates fail-closed

What is still missing is the next concrete step after target selection.

Without a written attack-intent contract, later slices would risk:
- inventing ad-hoc attack ingress straight inside `internal/minimal`
- coupling HP updates to a guessed packet layout too early
- introducing a separate clear-target packet family before proving the smaller reuse path is insufficient

So this document freezes the smallest next ownership boundary first.

## First owned attack-intent family

The first owned combat request is now frozen exactly as:
- name: `ATTACK`
- direction: client -> server
- phase: `GAME`
- header: `0x0401`
- payload length: `7`
- status: documented and codec-owned in `internal/proto/combat`

The exact payload layout is:
1. `uint8 attack_type`
2. `uint32 target_vid` (little-endian)
3. `uint8 crc_proc_piece`
4. `uint8 crc_file_piece`

What is frozen now about those fields:
- the first live dummy attack path accepts only `attack_type = 0` (`normal attack`) in this slice
- `target_vid` is the wire-visible target identity the client places in the request
- `crc_proc_piece` and `crc_file_piece` are currently owned as exact trailing raw bytes in the codec, but their higher-level validation role remains intentionally narrow in the clean-room runtime for now

This exact codec ownership matters because the next flow slices no longer need to guess the attack header or open-code a one-off byte layout inside `internal/minimal`.

## First owned ranged-shot ingress guard

Client and legacy-oracle source inspection also shows a separate client -> server `SHOOT` request in the same combat family:
- name: `SHOOT`
- direction: client -> server
- phase: `GAME`
- header: `0x0403`
- payload length: `1`
- payload: `uint8 shoot_type`

The bootstrap runtime now owns this packet only as a safe ingress guard, not as ranged combat behavior.
The `GAME` dispatcher decodes the fixed-width packet and can route it through a narrow handler seam, but the shipped minimal runtime leaves it unsupported and fail-closed:
- no target HP mutation
- no selected-target rewrite
- no normal-attack cadence change
- no self response frame
- no queued peer frame

This prevents a real client or packet harness from turning a known combat-family request into an unexpected-packet disconnect/error while keeping projectile, bow, skill, and hit-resolution policy out of this slice.

## First owned projectile-targeting ingress guards

Client and legacy-oracle source inspection also shows two client -> server projectile targeting requests in the same combat family:
- name: `FLY_TARGETING`
- direction: client -> server
- phase: `GAME`
- header: `0x0404`
- payload length: `12`
- payload: `uint32 target_vid` + `int32 x` + `int32 y` (little-endian)

and:
- name: `ADD_FLY_TARGETING`
- direction: client -> server
- phase: `GAME`
- header: `0x0405`
- payload length: `12`
- payload: `uint32 target_vid` + `int32 x` + `int32 y` (little-endian)

The current bootstrap runtime owns these packets only as safe ingress guards, not as accepted projectile or skill gameplay.
The `GAME` dispatcher decodes both fixed-width packets and can route them through narrow handler seams, but the shipped minimal runtime leaves them unsupported and fail-closed:
- no target HP mutation
- no selected-target rewrite
- no normal-attack cadence change
- no immediate or delayed retaliation scheduling side effect
- no self response frame
- no queued peer frame
- no point, inventory, or account-persistence side effect

This preserves the known wire layout for skill/bow target-position traffic without pretending that server-created fly effects, projectile hit resolution, multi-target skills, or ranged combat are already owned.

## First owned on-click ingress

Client and legacy-oracle source inspection also shows a client -> server click request in the target/UI family:
- name: `ON_CLICK`
- direction: client -> server
- phase: `GAME`
- header: `0x0A02`
- payload length: `4`
- payload: `uint32 vid` (little-endian)

The current bootstrap runtime owns this packet as a narrow guest private-shop browse open path when the clicked visible peer still has an accepted open `MYSHOP`, while unsupported click targets stay fail-closed:
- guest browse success emits one guest-only `GC::SHOP START` stock table for that host VID; inventory/gold stay unchanged
- guest open merchant / safebox / refine / exchange rejects with the already-owned requester busy info-chat string and no START
- guest own open MYSHOP / unknown / non-open-MyShop / NPC click targets stay silent no-frame
- no selected-target rewrite
- no target HP mutation
- no normal-attack cadence change
- no immediate or delayed retaliation scheduling side effect
- no authored static-actor `INTERACT` / quest side effect beyond the private-shop browse seam
- no point, inventory, or account-persistence side effect

This prevents real-client click traffic from becoming an unexpected-packet disconnect/error while keeping the authored-service interaction path on the separate `INTERACT` contract for NPC/static actors. Guest browse leave/`END` and buy/sell stay deferred.

## First owned character-position ingress guard

Client and legacy-oracle source inspection also shows a client -> server position / battle-position request in the target/UI family:
- name: `CHARACTER_POSITION`
- direction: client -> server
- phase: `GAME`
- header: `0x0A60`
- payload length: `1`
- payload: `uint8 position`

The current bootstrap runtime owns this packet as a narrow presentation-only stance ingress. The `GAME` dispatcher decodes the fixed-width packet and routes it through a minimal handler seam:
- `position = 0` (`POSITION_GENERAL`) emits self `GC CHARACTER_POSITION(selected_vid, 0)` and queues the same frame to currently visible live peers when the session was not already standing,
- `position = 3` (`POSITION_SITTING_CHAIR`) is accepted as a conservative sit request and normalized to the same `GC CHARACTER_POSITION(selected_vid, 4)` presentation used by ground sit,
- `position = 4` (`POSITION_SITTING_GROUND`) emits self `GC CHARACTER_POSITION(selected_vid, 4)` and queues the same frame to currently visible live peers when the session was not already sitting,
- duplicate stand/sit requests are accepted no-ops with no repeated self/peer frame,
- unsupported bytes, including the battle-position byte, still fail closed with no frames.

Accepted stance presentation deliberately remains side-effect-free for combat and persistence:
- no selected-target rewrite
- no target HP mutation
- no normal-attack cadence change
- no immediate or delayed retaliation scheduling side effect
- no point, inventory, or account-persistence side effect

The server -> client `CHARACTER_POSITION` update family and the still-non-emitted `CHANGE_SPEED` family are documented separately in `character-position-change-speed-bootstrap.md`. This ingress handling prevents known client sit/stand traffic from becoming an unexpected-packet disconnect/error while leaving final chair-object placement, battle-mode, persistent stance, speed, stun, skill, and projectile presentation policy to later dedicated slices.

## Active-target prerequisite

The first owned attack-intent path is intentionally target-relative rather than free-form.

Even though the exact wire request carries a `target_vid`, the bootstrap runtime contract still treats that field as **subordinate to the currently selected combat target** rather than as permission to attack an arbitrary visible actor.

An `ATTACK` request is only eligible when all of the following are true:
- the session is already in `GAME`
- the session still owns a selected live character
- that live character currently holds one active combat target from the existing `TARGET` selection contract
- the game-flow dispatcher decodes the fixed-width packet and rejects any `attack_type != 0` before invoking runtime combat handlers, so unsupported attack modes stay silent and cannot mutate world combat state
- the request uses `attack_type = 0` for the first normal-attack bootstrap path
- the request `target_vid` exactly matches the session's currently selected combat target
- that selected target still resolves to a visible same-map bootstrap practice target (`training_dummy` or `practice_mob` today)
- if the selected target is a spawn-backed actor, its authored-home/current-position leash classification is not `return_required`; the shared-world attempt seam reports this exact lifecycle gate as `target_return_required` rather than collapsing it into ordinary non-targetable content
- if the selected target is a spawn-backed actor already engaged by a different live owner in the current aggro-lite loop, the shared-world attempt seam reports this exact ownership gate as `target_engaged` rather than collapsing it into ordinary non-targetable content
- that selected target still passes the current bootstrap combat band

This keeps the first attack slice aligned with the already-owned `TARGET` path instead of creating a second competing target-identity model.

## First clear-target representation

The first owned clear-target companion is now frozen as a **reuse of the existing server -> client `TARGET` family**, not as a separate dedicated clear packet.

The working contract is:
- server -> client `TARGET` with `target_vid = 0`
- server -> client `TARGET` with `hp_percent = 0`
- combined meaning: **no active combat target remains bound to the session**

Why this reuse is the current preferred contract:
- the repository already owns `GC TARGET` as the smallest self-only target-state surface
- the same packet family can already describe `current target + hp percent`
- reusing it for `no target` avoids inventing a second clear-only family before tests prove a richer path is needed

So the first owned target-state surface is now intentionally tiny but expressive enough for three states:
1. `TARGET(target_vid > 0, hp_percent = 100)` — selected live dummy with fresh full bootstrap HP on first owned selection
2. `TARGET(target_vid > 0, hp_percent = updated)` — same selected dummy after accepted bootstrap attack-driven HP changes
3. `TARGET(0, 0)` — selected target cleared or no longer valid

The client-originated clear request is deliberately separate from that server clear companion:
- client -> server `TARGET(target_vid = 0)` is accepted as a silent clear-target intent for the current live selected session
- it emits no self `GC TARGET(0, 0)` echo because the current client clears local target state before sending the zero-VID request
- it clears the session's active target binding, resets the current first-owned normal-attack cadence window, cancels any pending practice-mob delayed retaliation beat owned by that selected target, and releases that session's current practice-mob engagement so another visible live session may select the still-live mob
- an accepted non-zero retarget is deliberately different: it may cancel the session-local pending delayed retaliation beat for the old selected target, but it must **not** release any current-life practice-mob engagement that was already established by an accepted hit, so fresh third-party `TARGET` attempts against that still-live mob continue to fail closed until an explicit release boundary such as client `TARGET(0)`, owner disappearance/rebootstrap, owner zero-HP stale cleanup, actor update/removal, or mob death/respawn
- a later `ATTACK` using the old target VID must fail closed until the session sends a fresh accepted non-zero `TARGET`

## Runtime combat-target snapshot

The runtime now also owns read-only selected-combat-target snapshots for local/debug callers.
They are not new client packets and do not replace the existing self-only `GC TARGET` wire surface.

For a live shared-world session with an active selected static-actor combat target, each snapshot reports:
- `subject_entity_id`
- `subject`, using the same effective connected-character snapshot shape exposed by `/local/players`
- `target_vid`
- the target `snapshot_version` captured from runtime combat ownership
- current target `hp_percent`
- current target `target_current_hp`, resolved from runtime-owned combat HP
- profile-owned `target_max_hp`
- profile-owned `normal_attack_damage`, using the same compact attack/defense formula currently applied by accepted normal hits
- profile-owned `target_attack_value` and `target_defense_value`, so local QA can inspect the authored formula inputs beside the resolved damage result
- the same compact static-actor snapshot shape used by local static-actor/visibility introspection; combat-profile actors now expose `combat_max_hp`, `combat_normal_damage`, `combat_attack_value`, and `combat_defense_value` alongside `combat_level` / `combat_rank`
- optional `engaged_by_entity_id` once an accepted hit has established the current practice-mob engagement owner
- optional `engaged_by`, using the same effective connected-character snapshot shape, when that owner still resolves as a live connected player
- optional `retaliation_point_delta` for engaged spawn-backed practice mobs whose bootstrap combat profile owns immediate or delayed owner-side retaliation
- optional `retaliation_server_origin = true` when the current selected target has an engaged owner on the server-origin retaliation cadence seam
- optional `retaliation_pending = true`, `retaliation_ready_at`, and `retaliation_remaining_ms` when that selected session currently has a delayed server-origin retaliation beat armed for the same target snapshot; `retaliation_remaining_ms` is clamped to `0` for a due-but-not-yet-flushed beat

The per-subject snapshot fails closed when the subject is missing, no longer has a live session hook, is already at the current bootstrap zero-HP floor, no target is selected, the selected target is no longer visible, outside the current combat band, classified as spawn-group `return_required`, blocked by the current aggro-lite owner gate (`target_engaged`) for that subject, or the selected actor no longer has owned bootstrap combat HP semantics.
After accepted non-lethal hits, snapshots report the runtime-owned damaged `hp_percent`, exact current HP, profile max HP, normal-hit damage, engagement owner, and current retaliation delta rather than resetting to full HP or hiding the aggro-lite ownership boundary; after the subject reaches the zero-HP floor or the target reaches the zero-HP death edge and selected-target ownership is cleared, the per-subject and aggregate snapshots omit that stale target instead of reporting a dead active selection.
The read-only surface intentionally mirrors the same target/attack authority gates already used by gameplay, but without mutating stale engagement records or selected-target state merely because an operator endpoint was read.
The embedded `subject` and `engaged_by` fields let local operator/debug consumers verify the selected subject and current engagement owner's effective map, position, empire/guild, and dead-state without joining the combat-target result to a separate `/local/players` response.
The loopback `/local/combat-target/{name}` operator/debug endpoint exposes that per-subject snapshot by exact character name.
The aggregate runtime snapshot skips invalid/stale entries, including selected targets whose owning session hook has already disappeared, and returns active selections in deterministic `subject_entity_id` order; the loopback `/local/combat-targets` endpoint exposes that list for local debugging.
The loopback `/local/maps/{map_index}/combat-targets` endpoint exposes the same aggregate snapshot shape filtered by the selected subject's effective map, so map-local spawn QA can inspect active selections beside the adjacent map-scoped spawn-group and respawn views.
It rejects malformed or zero map-index path values with `400`, returns `404` when the map is not currently represented in runtime occupancy, and returns an empty JSON array for a known map with no active selected combat targets.
This gives local operator surfaces a stable read-only seam without granting stale sockets or global actor lookups a new authoritative combat path.

## First damage-info hit-effect codec

The repository now owns the fixed-width server `DAMAGE_INFO` (`0x0410`) codec, self and visible-peer runtime emission for standalone bootstrap combat-profile hits, and the first self plus visible-peer emission for content-loaded spawn-backed practice-mob hits.
Its focused protocol note is `combat-damage-info-bootstrap.md`.

That hit-effect companion is intentionally separate from the authoritative combat-state carrier in this document:
- `TARGET(target_vid, hp_percent)` still owns the selected-target HP refresh for non-lethal bootstrap hits and is still sent first.
- Standalone bootstrap combat-profile non-lethal normal hits append one self `DAMAGE_INFO(target_vid, flag=0, damage=applied_damage)` after that target refresh and queue the same hit-effect companion to currently visible live peers.
- Spawn-backed practice-mob non-lethal normal hits now append one owner self `DAMAGE_INFO(target_vid, flag=0, damage=applied_damage)` after the existing immediate retaliation `PLAYER_POINT_CHANGE` when the owner remains alive, then one owner self `DAMAGE_INFO(owner_vid, flag=0, damage=abs(retaliation_delta))`, and queue both the mob and owner retaliation hit-effect frames to currently visible live peers; peers do not receive the owner's self `TARGET` refresh or retaliation point-change.
- Non-floor delayed server-origin retaliation beats now append the same owner self `DAMAGE_INFO(owner_vid, flag=0, damage=abs(retaliation_delta))` after their ordinary point-change and queue that same owner companion to currently visible live peers for both hit-armed and proximity-armed delayed cadences; owner-floor retaliation beats still omit that companion and keep `PLAYER_POINT_CHANGE(value=0)` -> `DEAD(owner_vid)` -> `TARGET(0, 0)`.
- `DEAD(vid)` plus `TARGET(0, 0)` still owns the zero-HP edge, and the current damage-info slice deliberately does not append a synthetic final damage-info frame on killing hits or owner-floor beats.
- Richer flag meanings, killing-hit damage-info, and broader hit-result policy remain later runtime emission policy.

## Relationship to later HP / death work

This document freezes the first deterministic bootstrap HP mutation and the preferred visible HP-refresh carrier.
The first owned death / respawn wire contract is now documented separately in `non-player-death-respawn-bootstrap.md`, and the live implementation now uses that contract for the visible zero-HP edge plus the timed respawn rebuild.

The current owned bootstrap combat state is intentionally tiny:
- visible `training_dummy` combat state is runtime-owned and starts at `10` HP
- each accepted bootstrap normal attack decrements the dummy by `1` HP until the zero-HP edge is reached
- accepted non-lethal hits keep the visible refresh on server `TARGET(target_vid, hp_percent)` using the current runtime HP converted to percent in `10`-point steps (`100`, `90`, `80`, ...)
- the final accepted hit now switches surfaces at `0` HP: visible sessions receive `GC DEAD(vid)`, selected sessions clear the stale target on `GC TARGET(0, 0)`, and the dummy stays dead until the owned timed respawn rebuild runs

What this still freezes about the **visible state carrier** for later slices:
- accepted later attack-driven HP refreshes should continue preferring server `TARGET` with the same selected `target_vid` plus the updated `hp_percent`
- target loss, invalidation, death cleanup, reconnect cleanup, transfer cleanup, or reclaim cleanup should prefer the zero-target `TARGET(0, 0)` companion before introducing a new clear-target family
- when subject movement or sync updates make the selected dummy leave current visibility or the bootstrap combat band, the runtime should proactively clear the active target with one self-only `TARGET(0, 0)` companion
- when the zero-HP death transition is reached, it keeps death-triggered target clear on the same `TARGET(0, 0)` surface while `GC DEAD(vid)` and respawn rebuild stay owned by `non-player-death-respawn-bootstrap.md`

The profile-stat formula slice now freezes that combat-profile defaults can carry `attack_value` and `defense_value` alongside the legacy `damage_per_normal_attack` fallback.
Registered combat profiles use the first deterministic formula for normal-attack HP mutation: `max(1, attack_value - defense_value)`.
Built-in bootstrap profiles keep their current one-point behavior because their owned defaults are `attack_value = 1` and `defense_value = 0`.
The shared-world attack attempt now also reports the actual non-negative damage amount applied by that formula as an internal runtime descriptor; standalone bootstrap combat-profile hit-effect emission uses that descriptor for its self-only `DAMAGE_INFO` companion, including custom registered formula profiles, and later broader emission policy should keep using the same authoritative damage value instead of letting presentation code recompute damage separately.
If a registered profile or portable `combat_profiles` snapshot omits `attack_value` but supplies legacy `damage_per_normal_attack`, registration and content-bundle / static-actor snapshot canonicalization expand `attack_value = damage_per_normal_attack + defense_value` so older tests/content keep the same visible damage even after adding defense metadata; that compatibility path now fails closed if the sum cannot fit the current `uint16` profile carrier.
If a registered profile or portable `combat_profiles` snapshot instead omits legacy `damage_per_normal_attack` but provides explicit non-zero `attack_value` / `defense_value`, the same seams canonicalize the legacy fallback from the deterministic formula so formula-first authored profiles can be accepted without carrying two duplicate damage fields.
If a registered profile supplies both legacy `damage_per_normal_attack` and explicit formula values, registration now requires those two surfaces to agree with the same deterministic formula; contradictory profiles fail closed instead of letting authored metadata claim one damage value while runtime attacks apply another.
Profiles whose explicit formula damage would exceed `max_hp` now fail closed instead of silently capping overkill metadata into the current bootstrap HP carrier.
Profiles that omit both legacy damage and explicit attack formula input fail closed instead of silently becoming one-damage combatants.
Combat-profile defaults now also carry presentation metadata: `level` and descriptor-only `rank`. The built-in bootstrap `training_dummy` and `practice_mob` profiles both currently freeze `level = 1` and `rank = 0`, and registered profile lookups preserve explicit level/rank values for later mob presentation, reward, or formula slices. Visible static-actor bootstrap now copies the resolved combat-profile `level` into the actor's `CHAR_ADDITIONAL_INFO.level` field, while `rank` remains runtime metadata only for now. These fields do not yet change the current normal-attack formula, target HP carrier, reward payout, or respawn timing.

If future captures or tests prove this carrier insufficient, the repository may add a richer combat packet family later.
But the next slices should begin from this smaller contract first.

## First skill-intent guard

The current bootstrap runtime now also owns `USE_SKILL` as a safe `GAME`-phase ingress guard, not as accepted skill combat.

The frozen client packet shape is:
- name: `USE_SKILL`
- direction: client -> server
- phase: `GAME`
- header: `0x0402`
- payload length: `8`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `uint32 skill_vnum` (little-endian)
2. `uint32 target_vid` (little-endian)

The current shipped behavior is intentionally conservative:
- `internal/game` decodes `USE_SKILL` in `GAME` and routes it through a dedicated handler seam
- the minimal runtime leaves that handler unset, so every ordinary `USE_SKILL` request fails closed with no frames
- unsupported `USE_SKILL` does not mutate the selected target, target HP, normal-attack cadence, retaliation timers, peer queues, points, inventory, or account persistence
- malformed `USE_SKILL` still fails at the codec/flow boundary with `ErrInvalidPayload`

This slice exists so skill packets from a real client are a known safe combat-family ingress while skill formulas, buffs/debuffs, cooldowns, visual effects, and accepted skill damage remain later work.

## Repeated-hit loop and runtime-only HP ownership

The current bootstrap repeated-hit rule is now frozen as narrowly as possible:
- a visible `training_dummy` starts the live combat loop at authored/bootstrap max HP
- each accepted normal attack against the still-selected dummy decrements current live HP exactly once by the current bootstrap step
- the server reuses self-only `GC TARGET(target_vid, hp_percent)` after each accepted hit so the same selected target surface shows the updated percentage
- re-selecting that same still-visible dummy during the same live world runtime should return the current runtime-owned `hp_percent`, not silently recreate full HP on every request

The ownership rule is equally important:
- dummy HP belongs to shared-world runtime state, not to account or character persistence
- accepted dummy hits must not write inventory, equipment, player points, or any other character save payload as a side effect of combat alone
- this document does **not** yet freeze whether a reconnect, transfer, or future world rebuild should preserve or recreate dummy HP; it only freezes that the current bootstrap loop is runtime-owned and non-persistent

## First target-relative normal-attack cadence window

The next owned timing rule is still intentionally tiny:
- the bootstrap runtime now owns one fixed `250ms` cadence window after each accepted normal `ATTACK`
- the first accepted normal hit on a live selected snapshot starts the server-owned window
- another same-target normal `ATTACK` that arrives before the `250ms` window expires fails closed with no combat-visible frames, no extra HP mutation, no extra immediate retaliation, and no extra delayed retaliation scheduling side effect
- once the `250ms` window expires, the next same-target normal `ATTACK` can be accepted again if the rest of the current target/visibility/range/dead-state checks still pass
- the window is measured from server-owned runtime time (`runtime.now` in tests, wall-clock time otherwise), not from client animation or any client-supplied timestamp
- clearing the active selected target resets this first owned cadence window, but replacing it through an accepted retarget does **not** reset the timer: an accepted hit against one target still suppresses immediate normal attacks against a newly selected second target until the same `250ms` window expires
- this retarget-preserved window is covered directly in the minimal runtime regression suite so future target-selection changes cannot accidentally turn accepted retargets into an attack-speed bypass
- client-originated `TARGET(0)` is now one of the explicit clear-selected-target boundaries that resets the window and releases that session's current practice-mob engagement; accepted non-zero retargets remain cadence-preserving and do not release already-engaged still-live practice mobs as above

## Failure semantics

The first owned attack-intent path must stay fail-closed.

An `ATTACK` request must fail closed when any of these are true:
- wrong phase
- malformed codec payload
- no selected live character exists
- no active combat target is currently bound to the session
- the request uses a non-normal bootstrap `attack_type`
- the request `target_vid` does not match the session's active combat target
- the selected target is no longer visible
- the selected target is no longer a `training_dummy`
- the selected target is no longer within the current bootstrap combat band
- the selected target no longer matches the current runtime snapshot bound to the session's accepted target selection
- the selected target is now at `0` HP / dead under runtime-owned dummy state
- the selected spawn-backed target has drifted far enough from authored home to classify `return_required` under the current bootstrap leash seam; runtime target/attack attempts expose this as `target_return_required` while the client-visible denial remains silent/no-frame
- the selected spawn-backed target is currently engaged by another live owner under the aggro-lite gate; runtime target/attack attempts expose this as `target_engaged` while the client-visible denial remains silent/no-frame
- the engaged owner's current bootstrap HP is already `0` after the current practice-mob retaliation slice reached the floor
- the session already lost authoritative live ownership because another session reclaimed the same character

The current visible failure expectations are intentionally narrow:
- malformed or wrong-phase requests may still stop at codec/flow rejection without a visible combat packet
- plain denied attack attempts do not yet require chat spam, peer fanout, or richer combat-result frames
- when runtime state already held a previously selected combat target and subject movement/sync makes that target invisible or out of the current combat band, the preferred first visible reset companion is one self-only `TARGET(0, 0)` plus local active-target cleanup

## Ownership and lifecycle rule

The first owned attack-intent contract must inherit the existing shared-world ownership model:
- attack authority belongs to the current live selected-character session
- stale reclaimed sockets must not authoritatively damage runtime-owned dummy HP, clear or replace the live owner's selected combat target, or queue combat-visible refresh frames to the replacement owner
- accepted target ownership now binds both the current dummy `target_vid` and the current runtime snapshot behind that `VID`; later attacks fail closed if the dummy was replaced before the session reselects it
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear session-local active combat target ownership before later attacks can proceed again; a transfer/rebootstrap boundary that had a selected combat target also carries one self-only `GC TARGET(0, 0)` clear in the origin rebuild frames, while fresh bootstrap re-entry and reconnect keep their reset silent
- operator/runtime removal of the currently selected dummy now also counts as an immediate combat-reset boundary: visible sessions first receive the normal actor `CHARACTER_DEL` teardown, and any still-selected live session also receives one queued self-only `GC TARGET(0, 0)` so later stale `ATTACK` attempts fail closed without retaining authoritative target ownership
- operator/runtime in-place update of the currently selected dummy now counts as the same kind of combat-reset boundary for snapshot-bound target ownership: after the ordinary actor refresh or visibility-transition frames from that update, any still-selected live session also receives one queued self-only `GC TARGET(0, 0)` so later stale `ATTACK` attempts fail closed until the dummy is reselected, and any current practice-mob engagement on that life is released so fresh third-party `TARGET` attempts can succeed again after the update
- non-player HP/dead state belongs to runtime world ownership, not to character persistence

Only some lifecycle edges are owned so far.
This document now freezes movement/sync invalidation plus fresh bootstrap/rebootstrap cleanup, including EnterGame reclaim of a stale same-character owner that already had a pending delayed practice-mob retaliation beat armed: reclaim clears that shared-world pending timer, releases aggro-lite engagement, and lets a still-visible peer reacquire the damaged mob without waiting for death/respawn. Remaining broader lifecycle edges should keep aligning with that same runtime ownership model instead of creating a second combat-only ownership model.

## First sustained delayed server-origin retaliation cadence for engaged content practice mobs

The first owned delayed server-origin retaliation cadence is still narrow, but it is now autonomous once engagement has started:
- it currently applies only to content-loaded practice mobs imported from `spawn_groups` whose `combat_profile` resolves through the bootstrap combat-profile registry, including the built-in `training_dummy` / `practice_mob` profiles and custom registered or bundled profiles
- the first accepted owner-side normal hit that leaves that engaged mob alive arms one additional self-only `GC POINT_CHANGE` HP decrement after a fixed `1s` delay
- proximity aggro-radius acquisition that establishes the same aggro-lite `engaged_by` ownership without a selected combat target may also arm that same delayed self-only cadence for the engaged owner; acquisition still invents no selected-target ownership and still emits no immediate retaliation piggyback
- when that same proximity-only engagement later loses the owner through `MOVE` / `SYNC_POSITION` outside `DefaultSpawnAggroRadius` without an active selected combat target, the runtime cancels the pending delayed beat, releases `engaged_by`, clears chase schedules for that actor, and stays silent (no invented self `TARGET(0, 0)`); selected-target movement invalidation continues to use the existing combat-band / visibility clear path instead
- once that delayed beat fires while the same engaged mob still owns the same live owner, it automatically arms the next delayed beat after the same fixed `1s` delay even if the player sends no later `ATTACK`
- profile-authored per-mob delayed reaction arming delay beyond the bootstrap `1s` default is frozen separately as optional `combat_profiles.reaction_delay_ms` in `content-spawn-groups-bootstrap.md`; until that seam is GREEN, live delayed server-origin retaliation arming / re-arm keep using `bootstrapPracticeMobServerOriginRetaliationDelay`
- each queued beat is server-origin only: it arrives through the pending server-frame path instead of piggybacking only on a fresh client attack frame
- socket-level regression coverage now proves this delayed path through the legacy TCP listener: no beat is emitted before the fixed delay, fired beats re-arm the next one while the engagement remains live, and a delayed floor transition emits the owned `GC POINT_CHANGE` -> `GC DEAD` -> `GC TARGET(0, 0)` sequence before stale same-target attacks fail closed
- it reuses the same bootstrap player-point carrier and defaults to the `-1` HP decrement already used by the immediate owner-side retaliation piggyback
- custom registered or bundled combat profiles may now author a negative `retaliation_point_delta`; the immediate owner-side retaliation piggyback and every delayed server-origin follow-up beat use that same profile-owned delta, omitted or `0` values canonicalize to the bootstrap `-1` decrement, and positive deltas fail closed at profile registration/import because this first hostility seam cannot heal the owner
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor instead of driving the owner's visible HP negative; once the owner's live HP reaches `0`, later immediate or delayed retaliation point-loss beats fail closed until broader player-death semantics are owned separately
- those immediate and delayed owner-side retaliation point-loss beats currently mutate only the engaged selected-session live runtime until the bootstrap `0`-HP floor; partial (above-floor) loss does **not** write the persisted account snapshot, and later successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves now keep their authored use/equip-effect point delta plus consumed or carried/equipped item state without overwriting that pre-retaliation point value. Once either retaliation beat reaches the `0`-HP floor, the selected-character bootstrap HP point is persisted as `0` with the owned death/clear frames so a fresh `/phase_select` re-entry, reconnect, or `ENTERGAME` rebuilds from the dead snapshot instead of the pre-retaliation live value; accepted `/restart_here` / `/restart_town` restore race create MaxHP into that persisted snapshot
- when an immediate or delayed retaliation beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC POINT_CHANGE(HP, amount = final clamped negative retaliation delta, value = 0)`, then one self-only `GC DEAD(owner_vid)`, then one self-only `GC TARGET(0, 0)` instead of leaving the stale engagement selected while broader player-death choreography stays out of scope; that same floor ordering applies to proximity-armed delayed beats that never invented selected-target ownership and never accepted an owner `ATTACK`, and the self `TARGET(0, 0)` companion remains intentional on that edge even when no prior selection existed (unlike silent proximity walk-away release outside aggro radius)
- after that owner-side floor transition, no later delayed retaliation beat is re-armed for the stale engagement, and a stale same-target normal `ATTACK` from the floored owner fails closed until a separate owned recovery path such as `/restart_here` or `/restart_town` rebuilds the live player state and the target is freshly reselected
- if that same immediate or delayed zero-HP transition happens while the owner still has an active merchant window open, the runtime appends one self-only `GC::SHOP END` after the point-change/dead/clear-target sequence and clears the active merchant context so later `SHOP END` / `SHOP BUY` attempts fail closed until a fresh merchant interaction opens a new window
- if that same immediate or delayed zero-HP transition happens while the owner still has an accepted host-only `MYSHOP` open, the runtime appends one empty-sign `GC::SHOP_SIGN` after the point-change/dead/clear-target sequence (after any merchant `GC::SHOP END`, before any safebox `CloseSafebox` or exchange `END`), fans that same empty sign to currently visible peers, and clears the private-shop busy flag so later `/close_myshop` stays silent without inventory/gold mutation
- that same zero-HP owner transition also queues one visibility-gated peer `GC DEAD(owner_vid)` only for currently visible live peers; connected recipients already sitting at the same bootstrap `0`-HP floor are skipped from later peer-death fanout
- once that engaged owner's live HP is already `0` because an earlier immediate or delayed retaliation beat reached the current bootstrap floor, later combat `TARGET` and normal `ATTACK` attempts from that same owner against engaged practice mobs also fail closed instead of continuing the combat loop while broader player-death semantics remain out of scope
- once that same owner-side floor has already been reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed before live-position mutation, peer relocation fanout, or transfer-trigger rebootstrap work can run
- once that same owner-side floor has already been reached, later owner-side slash `/inventory_move` attempts also fail closed before carried-slot mutation can run
- once that same owner-side floor has already been reached, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed before carried/equipped item movement, self appearance refresh, or template-backed point mutation can run
- the runtime still keeps at most one pending delayed beat at a time for that engaged owner/target pair; if another accepted hit lands while a delayed beat is already pending, it does not stack, accelerate, or reset the already-owned cadence timer, and focused runtime coverage now pins the case where a second accepted hit lands halfway through the first pending beat but the delayed point-change still fires only at the original due time
- the cadence fails closed and stops if the engaged owner loses live shared-world ownership, clears or replaces the selected target, successfully crosses a transfer / rebootstrap reset boundary, an EnterGame reclaim removes a stale same-character owner before a replacement joins, an operator return-home trigger resets the engaged actor, or the engaged actor dies / rebuilds before the next delay expires; this live-ownership-loss rule now also covers partial-teardown states where either the shared-world session hook or the live player entity has already disappeared before the other index is cleaned up, and at-home return-home advances the combat snapshot version so stale pre-reset delayed beats cannot fire after a fresh reengage
- when the engaged actor reaches the accepted non-player zero-HP death edge, the killing hit keeps the owned mob death choreography (`GC DEAD(target_vid)` + `GC TARGET(0, 0)`) without appending another owner-side retaliation point-change, cancels any pending delayed retaliation beat before the respawn delay elapses, and keeps the cadence stopped after the respawn rebuild until a fresh post-respawn `TARGET` / accepted hit starts a new engagement
- client-originated `TARGET(0)` is also a current live target-intent loss boundary for that same delayed cadence: it cancels the pending delayed beat, releases that owner's current engagement on the still-live practice mob, clears any pending chase-step deadline for that abandoned engagement, and does not send a compensating self target-clear echo
- same-socket `/quit`, `/logout`, and `/phase_select` now all count as immediate live-ownership loss boundaries for that cadence, and abrupt session close does too: each path removes the owner from shared-world visibility, cancels any pending delayed beat, and releases the current practice-mob engagement right away; the same release now also happens lazily on the next fresh third-party `TARGET` attempt if only one side of the engaged owner's live shared-world ownership survived a partial teardown, or if the recorded owner still has a shared-world player snapshot but that snapshot is already at the bootstrap `0`-HP floor; `/quit` still remains in `GAME` just long enough to return its self `CHAT_TYPE_COMMAND quit` delivery, `/logout` continues toward close, `/phase_select` now returns to character select while any later fresh bootstrap still requires a new `TARGET`, and close tears the session down without a compensating gameplay packet
- once that first authoritative hit establishes or preserves the current spawn-group combat engagement, any other session's already-selected shared-world combat-target ownership for that same mob is cleared immediately, and each still-live affected third party now also receives one queued self-only `GC TARGET(0, 0)` companion so a preselected third party cannot keep or visually retain a stale selected-target bypass before later `ATTACK` or fresh `TARGET` retries fail closed against a mob another session already engaged first; this aggro-lite ownership gate applies to every registered bootstrap combat profile resolved through `BootstrapStaticActorCombatProfileDefaults`, not only the built-in `training_dummy` / `practice_mob` names
- a successful transfer / rebootstrap now also counts as an immediate combat-reset boundary for that cadence: it clears the active practice-mob target, cancels any pending delayed beat, releases the previous engagement right away, and keeps that same still-live mob targetable again at its current runtime-owned HP instead of leaving it orphan-locked until disconnect or mob death / respawn
- this is still a tiny deterministic cadence, not broader AI: it remains owner-only, fixed-delay, and bound to the current engaged live target instead of widening into movement, chase, or mob packet families yet

## Explicit unknowns still left beyond the current bootstrap contract

Later flow/gameplay slices still need to prove or freeze:
- whether the runtime should validate or currently only preserve the two trailing raw CRC bytes
- whether later attack-speed ownership should stay target-relative or widen into a broader session-wide/global policy across target swaps
- how and when above-floor (partial) owner retaliation loss should eventually hand off into broader persisted player-death / revive state instead of staying session-local; the bootstrap `0`-HP floor itself is already persisted with the owned death/clear frames
- whether later hostile retaliation should widen beyond the current fixed-delay owner-only cadence into broader AI or richer mob-origin packet surfaces

Those unknowns are deliberate.
The codec now owns the exact wire shape, but the gameplay contract is still intentionally narrower than full combat semantics.

## Explicit non-goals

This slice does **not** yet freeze:
- the final gameplay meaning of every `attack_type` value
- full legacy damage formulas and wider attack types beyond the current compact registered bootstrap combat-profile defaults (`max_hp`, `damage_per_normal_attack`, `attack_value`, `defense_value`, `level`, `rank`, `respawn_delay`, optional `retaliation_point_delta`, and optional death reward); the playable authored formula seam is already owned through portable `combat_profiles` plus the repository example at `docs/examples/bootstrap-combat-profile-formula-bundle.json`
- broad authored combat-profile fields beyond the current runtime registry / portable `combat_profiles` seam
- broader attack-speed rules beyond the first fixed session-local `250ms` normal-attack cadence window
- miss/crit/block results
- ranged `SHOOT` gameplay beyond the current decode-and-fail-closed guard
- accepted `USE_SKILL` gameplay beyond the current decode-and-fail-closed guard
- accepted `ON_CLICK` interaction/shop/quest gameplay beyond the owned guest private-shop browse open seam against an already-open peer MYSHOP
- broader `CHARACTER_POSITION` / battle-position gameplay beyond the current presentation-only `position=0|3|4` stance echo/no-op guard and unsupported-byte fail-closed guard
- the broader server-driven respawn/delete-readd choreography details beyond the already-owned fixed timed rebuild that the separate death / respawn doc now freezes
- broader hostile retaliation beyond the current owner-side self-only point-loss surfaces: one immediate piggyback on accepted practice-mob hits plus one sustained fixed-delay delayed server-origin follow-up cadence at a time
- broader player-death / respawn semantics or broader non-combat gameplay gating for zero-HP owners after that floor is reached beyond the self-only `GC DEAD(owner_vid)` signal frozen in `player-death-bootstrap.md`
- player-vs-player attack semantics
- skills, buffs, debuffs, or status effects
- projectile targeting or server fly-effect gameplay beyond the current decode-and-fail-closed client `FLY_TARGETING` / `ADD_FLY_TARGETING` guards and the codec-only server `FLY_TARGETING` / `ADD_FLY_TARGETING` / `CREATE_FLY` packet shapes frozen in `combat-fly-effect-bootstrap.md`
- broader reward systems beyond the narrow non-player death descriptor seam
- corpse gameplay, aggro movement, or independent mob AI


## Success definition

After this document lands, the repository should be able to say:
- the next combat ingress is no longer vague; `ATTACK` is frozen exactly as client -> server header `0x0401`
- the project now owns the first clean-room `ATTACK` codec layout: `attack_type`, `target_vid`, `crc_proc_piece`, `crc_file_piece`
- the first live dummy attack path accepts only `attack_type = 0` and keeps gameplay target-relative by requiring the request `target_vid` to match the active selected combat target
- the first accepted bootstrap attack now mutates runtime-owned `training_dummy` HP deterministically from `10` downward in `1`-HP steps while reusing self-only `GC TARGET(target_vid, hp_percent)` as its visible success refresh
- accepted reselection of the same damaged dummy reuses the same current runtime `hp_percent` instead of resetting the visible target state back to `100`
- subject movement/sync that makes the selected dummy leave current visibility or the bootstrap combat band now proactively emits one self-only `GC TARGET(0, 0)` and clears the session-local active target
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear the session-local active target too; fresh bootstrap re-entry and reconnect keep that lifecycle reset silent, while a successful transfer/rebootstrap with a selected combat target emits one self-only `GC TARGET(0, 0)` in the origin rebuild frames, cancels any pending delayed retaliation beat, and releases the previous engagement immediately
- duplicate-live reclaim now inherits the same shared-world hardening model as movement, whisper, item use, and merchant seams: stale `TARGET` / `ATTACK` packets fail closed and cannot mutate runtime dummy HP or the replacement owner's target state
- accepted target ownership now also carries the current runtime snapshot behind the selected dummy `VID`, so later `ATTACK` requests fail closed if that dummy is replaced before the session reselects it
- the zero-HP transition is now live: the final accepted hit drives the dummy from `1` to `0`, emits `GC DEAD(vid)` to visible sessions, and clears any selected session's combat target on the existing self-only `GC TARGET(0, 0)` surface
- a dead dummy is no longer targetable or attackable through the current bootstrap `TARGET` / `ATTACK` loop until the owned timed respawn-reset rebuild completes
- if a live session is shown that same still-dead dummy again before respawn through fresh bootstrap, visibility re-entry, or a later retained delete-plus-rebootstrap refresh, the repo now reuses the ordinary actor add/info/update burst and immediately replays one trailing `GC DEAD(vid)` so the dummy does not silently look alive again
- the first owned clear-target representation is now `GC TARGET(0, 0)`
- later HP refreshes stay on the same `GC TARGET(target_vid, hp_percent)` carrier until the zero-HP death edge, after which the repo switches to `GC DEAD(vid)` + target clear rather than inventing richer combat-result packets early
- the first death / respawn wire contract is now frozen separately in `non-player-death-respawn-bootstrap.md`, and this attack slice now interoperates with that already-owned timed server-driven respawn reset instead of inventing a second rebuild path here
- content-loaded `spawn_groups` practice mobs now own the first aggro-lite post-hit target gate too: once the first authoritative hit is accepted, fresh third-party `TARGET` attempts fail closed with the explicit runtime reason `target_engaged` until the existing death / respawn reset boundary, affected preselected third parties also receive one queued self-only `GC TARGET(0, 0)` stale-selection clear, and retaliation-driven owner death can still clear the current engagement first without claiming broader mob hostility yet
- repeated normal `ATTACK` attempts are now also rate-owned in one narrow bootstrap shape: after an accepted hit, the same selected session rejects further normal attacks for `250ms`, including attacks after a successful retarget to another visible dummy, then accepts again once that fixed server-owned window expires
- that same first hostility seam is now slightly richer but still deterministic: while the engaged content-loaded practice mob stays alive, each accepted owner-side normal hit still appends one immediate self-only `GC POINT_CHANGE` HP decrement to the attack success frames, and the first accepted live hit now starts a delayed self-only `GC POINT_CHANGE` follow-up cadence that keeps firing every `1s` while the same engagement remains live; proximity aggro-radius acquisition that establishes the same aggro-lite engagement without a selected combat target may also arm that delayed cadence without inventing selected-target ownership or an immediate piggyback; custom registered or bundled spawn-group combat profiles use the same owner-side immediate and delayed retaliation cadence as the built-in bootstrap practice profiles, with a profile-owned negative `retaliation_point_delta` overriding the default `-1` amount for both immediate and delayed beats
- those immediate and delayed owner-side retaliation point-loss beats stay runtime-only for the engaged selected session until the bootstrap `0`-HP floor: partial (above-floor) loss does **not** write the persisted account snapshot, and later position-only persistence helpers (`MOVE`, `SYNC_POSITION`, or transfer rebootstrap saves), successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves now keep their coordinate, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot, or purchase state without overwriting that pre-retaliation point value. Once either retaliation beat reaches the `0`-HP floor, the selected-character bootstrap HP point is persisted as `0` with the owned death/clear frames so a fresh `/phase_select` re-entry, reconnect, or `ENTERGAME` rebuilds from the dead snapshot instead of the pre-retaliation live value; accepted `/restart_here` / `/restart_town` restore race create MaxHP into that persisted snapshot
- those owner-side retaliation point-loss beats now stop at the bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed server-origin cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further point-loss without yet claiming broader player-death choreography
- when either the immediate retaliation tick or a delayed follow-up beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC DEAD(owner_vid)` and one self-only `GC TARGET(0, 0)` clear so the stale engaged mob is no longer kept as the active combat target
- if that same zero-HP floor is reached while a merchant window is open, the runtime appends one self-only `GC::SHOP END` after the point-change/dead/clear-target sequence and consumes the merchant context before any later `SHOP END` / `SHOP BUY` request can run
- if that same zero-HP floor is reached while an accepted host-only `MYSHOP` is open, the runtime appends one empty-sign `GC::SHOP_SIGN` after the point-change/dead/clear-target sequence (after any merchant `GC::SHOP END`, before any safebox `CloseSafebox` or exchange close), fans that same empty sign to currently visible peers, and clears the private-shop busy flag before any later `/close_myshop` request can invent a second empty sign; browsing guests of that host also receive one queued self-only `GC::SHOP END`, and a dying guest who was browsing another open MYSHOP appends one self-only `GC::SHOP END` after death/clear
- that same owner-side `0`-HP transition also queues one visibility-gated peer `GC DEAD(owner_vid)` for currently visible live peers, while already-dead connected recipients are skipped from later peer-death fanout
- when that same owner-side `0`-HP floor is reached while a content-loaded practice mob still remains alive, the dead owner also stops holding that mob's aggro-lite engagement gate: a different visible live session may reacquire the same still-live mob with a fresh `TARGET` without waiting for the mob to die / respawn or for the dead owner to disconnect first
- once that retaliation floor has already reached `0`, later same-owner combat `TARGET` and normal `ATTACK` attempts against still-engaged content practice mobs now fail closed too, so the current hostility seam no longer lets a zero-HP owner keep reacquiring or advancing dummy combat state while broader player-death semantics are still pending
- accepted hits while one delayed follow-up beat is already pending do not stack, accelerate, or reset the current cadence timer; the runtime keeps only one queued delayed beat outstanding at a time, so a later accepted hit before the first due time keeps the original due time and produces only one delayed `PLAYER_POINT_CHANGE` when it fires
- same-target normal `ATTACK` attempts denied inside that `250ms` cadence window stay fully silent: they do not refresh target HP, do not append immediate retaliation, and do not create or reset delayed retaliation work
- client `USE_SKILL(0x0402)` is now codec- and dispatch-owned as an unsupported skill-combat guard; the minimal runtime decodes it in `GAME` but returns no frames and leaves selected-target HP, normal-attack cadence, retaliation timers, peer queues, points, inventory, and account persistence unchanged
- client `SHOOT(0x0403)` is now codec- and dispatch-owned as an unsupported ranged-shot guard; the minimal runtime decodes it in `GAME` but returns no frames and leaves selected-target HP/cadence/peer queues unchanged
- client `FLY_TARGETING(0x0404)` and `ADD_FLY_TARGETING(0x0405)` are now codec- and dispatch-owned as unsupported projectile-targeting guards; the minimal runtime decodes them in `GAME` but returns no frames and leaves selected-target HP, normal-attack cadence, retaliation timers, peer queues, points, inventory, and account persistence unchanged
- client `ON_CLICK(0x0A02)` is now codec- and dispatch-owned as guest private-shop browse open against an already-open peer MYSHOP (one guest-only `GC::SHOP START`, busy-shell rejects reuse exchange merchant/safebox/refine busy info-chat strings, guest own open MYSHOP / unknown targets stay silent no-frame) while leaving selected-target HP, normal-attack cadence, peer queues, points, inventory, and account persistence unchanged; NPC/quest click gameplay beyond that browse seam stays unsupported
- client `CHARACTER_POSITION(0x0A60)` is now codec- and dispatch-owned as a narrow stance-presentation ingress: while the selected owner is live and above the bootstrap zero-HP floor, `position=0` and `position=4` return `GC CHARACTER_POSITION(selected_vid, position)` to the selected socket and currently visible live peers while leaving selected-target HP, normal-attack cadence, retaliation timers, points, inventory, and account persistence unchanged; after retaliation has driven that owner to the current zero-HP floor, later stance requests fail closed before self/peer position presentation, and unsupported bytes, including the current battle-position byte, still fail closed with no frames or side effects
- if that engaged owner loses live shared-world ownership, clears or replaces target intent, or the engaged actor dies / rebuilds before a pending delay expires, the queued follow-up beat fails closed and the current cadence stops instead of claiming broader AI cleanup
- when the engaged actor dies, the killing hit does not append an extra owner-side retaliation point-change, any pending delayed follow-up beat is canceled before respawn, and the respawn rebuild does not resurrect stale retaliation work without a fresh target / accepted hit
- same-socket `/quit`, `/logout`, and `/phase_select` now all count as immediate owner-disappearance boundaries for that queued delayed cadence, and abrupt session close does too: each path removes the owner from shared-world visibility, cancels any pending delayed beat, and releases the same still-live practice mob right away; `/quit` still remains in `GAME` just long enough to return its self `CHAT_TYPE_COMMAND quit` delivery, `/logout` continues toward close, `/phase_select` returns to character select while any later fresh bootstrap still requires a new `TARGET`, and close tears the session down without a compensating gameplay packet
