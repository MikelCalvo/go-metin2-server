# Spawn Leash Bootstrap

This document freezes the first tiny runtime seams for future mob chase/leash/return work in `go-metin2-server`.

It sits on top of:
- `non-player-entity-bootstrap.md`
- `content-spawn-groups-bootstrap.md`
- `world-topology-bootstrap.md`

Those documents already freeze the current player/non-player identity, map placement, authored `spawn_groups`, target/attack/death/respawn loop, and topology visibility rules.

What this document adds is deliberately narrower:

**How does the runtime classify a spawn-backed actor's current position against its authored spawn position, and how does it apply one deterministic return-home step before any autonomous movement/pathing AI exists?**

## Current owned contract

The first bootstrap leash seam is a pure runtime classification helper in `internal/worldruntime`, exposed through the bootstrap runtime for read-only operator inspection and through one controlled operator return-home trigger. The second tiny seam is the pure return-step planner in the same package plus a loopback-only runtime trigger that applies exactly one planned step through the existing static-actor persistence and visibility-rebuild path. The first server-owned executor now reuses that same one-step path from the pending server-frame flush loop for live spawn-backed actors that already classify `return_required`, including actors restored from a persisted static-actor snapshot that was already outside leash at process start; this is still an aggro-lite/bootstrap lifecycle seam, not full chase AI.

Inputs:
- authored/home `Position { map_index, x, y }`
- current `Position { map_index, x, y }`
- positive leash `radius`

For spawn-backed static actors, the authored/home position is the actor's `spawn_group_ref` placement. The runtime preserves that authored home separately from the materialized actor's current `Position`, including generic runtime/operator position edits that keep the same `spawn_group_ref`. Older snapshots that lack a preserved home position fall back to the current position and classify as `at_home` until moved by a later owned seam.

In the current stationary practice-mob runtime, freshly imported mobs normally classify as `at_home`. If an owned runtime/operator update changes only the materialized actor position, the same read-only leash inspection must continue to compare that current position against the preserved authored home and can report `within_radius` or `return_required` without mutating the actor.

The pure return-step planning primitive remains `PlanStaticActorSpawnLeashReturnStep(actor, radius, max_step)`:
- it first reuses the same preserved-home classifier and fails closed for invalid/non-spawn actors, non-positive leash radius, or non-positive `max_step`
- if the actor does not require return, it returns the current position with `complete = true`
- if the actor is on the same map and outside leash radius, it returns one deterministic x/y step toward authored home, capped by `max_step`; if the actor is already within one step, it returns the exact authored home with `complete = true`
- if the actor is on a different map from authored home, it returns the authored home directly with `complete = true` because no client-facing chase/warp packet choreography is owned yet
- by itself it never changes `StaticEntity.Position`, never updates the static-actor store, never changes HP/death/engagement state, and never queues visibility frames

The first live return-step consumer is deliberately local/operator-only:
- endpoint: `POST /local/spawn-groups/{entity_id}/return-step?max_step=<positive-int>`
- scope: `gamed` local/operator tooling only
- request body: none
- success response: `{ actor, step }`, where `actor` is the stepped spawn-group snapshot and `step` embeds the leash snapshot that was evaluated before the move plus `next` and `complete`

On success, the trigger plans one return step with the default bootstrap leash radius, persists the materialized static-actor position to `step.next`, then mutates runtime state through the same static-actor visibility transition helpers used by operator actor updates. Same-map retained viewers receive one server-driven `MOVE` replication at the stepped position; old-position-only viewers receive `CHARACTER_DEL`; newly visible viewers receive the normal add/info/update burst. Cross-map return-step remains on delete/readd because no return warp packet seam is owned yet. A moving step preserves HP, death timers, reward descriptors, and combat profile metadata, but now releases current practice-mob engagement, clears selected combat targets bound to that actor's visible `VID`, and advances the runtime combat snapshot version so stale selected `ATTACK` and stale delayed retaliation beats fail closed until fresh target acquisition. If that manual/operator step leaves the actor still `return_required`, the runtime refreshes the pending automatic return-step deadline from the manual step time and replaces any older pre-manual deadline; the old deadline must not fire immediately after the operator has already moved the actor. If the step completes or moves the actor back inside leash, the pending deadline is cleared. If the actor is already `at_home` or merely `within_radius`, the trigger returns a no-op `complete = true` step and does not persist, mutate runtime position, clear targets, release engagement, advance snapshot ownership, queue frames, or schedule a new automatic step. Dead actors waiting on respawn fail closed until the server-owned respawn boundary rebuilds them.

The first server-owned executor deliberately does not introduce a general mob scheduler or goroutine loop. When an owned runtime/operator update leaves a live spawn-backed actor in `return_required`, or when runtime startup restores a persisted live spawn-backed actor that already classifies `return_required`, the runtime arms one pending return-step deadline. If a same-profile runtime/operator update happens while a spawn-backed actor is still in its server-owned dead interval, the actor can remain visible/debuggable and even classify `return_required`, but return-step scheduling stays suppressed: the dead actor keeps its respawn ownership and does not arm an automatic return-step deadline until the respawn rebuild makes it live again. When that respawn rebuild succeeds, the runtime resynchronizes the return-step schedule from the rebuilt actor snapshot, clearing any stale pre-death deadline when the actor returned to authored home or inside leash and only arming a new one if the live respawned actor still classifies `return_required`. Each `FlushServerFrames()` pass first flushes due respawns, then applies any due spawn return steps with the same fixed `max_step = 100` path used by the loopback trigger, including the same engagement release, selected-target clear, and combat snapshot reset semantics for sessions that still had the actor selected, then due homeward steps, then due chase steps, then proximity acquisition / session-local delayed retaliation. A fresh `EnterGame` / visibility bootstrap that starts after a pending return-step deadline is already due should preflight the same ready return-step flush before encoding static-actor visibility, so a newly entering nearby client sees the current server-owned stepped position and does not first receive stale displaced spawn visibility followed by a redundant queued rebuild. A due automatic step persists the stepped materialized position before mutating runtime, queues the same retained/removed/added visibility frames as the operator step, and then arms the next one-second return step only if the post-step actor snapshot is still live and `return_required`. If a due step moves the actor back inside the leash radius without landing exactly at authored home, the executor clears the pending deadline and does not schedule a later no-op flush. Unengaged actors that classify `within_radius` after chase/engagement release are now recovered by the owned pending-frame homeward-step executor toward authored home; exact-home snaps for arbitrary drift without that homeward seam still remain the controlled loopback return-home trigger, and operator `return-step` still no-ops `within_radius`.

That same preflight applies to transfer/rebootstrap visibility deltas: before a `MOVE` / `SYNC_POSITION` transfer trigger builds the owner's destination static-actor visibility burst, the runtime flushes any already-due respawn, return-step, homeward-step, and chase-step timers so the moved client sees the current server-owned stepped spawn position in the transfer response instead of stale destination visibility followed by a duplicate queued rebuild. The transfer preflight preserves the existing rebootstrap packet families; it only changes which static-actor snapshot is used when a due lifecycle timer has already advanced server-owned spawn state.

The paired read-only pending-return schedule surfaces are:
- `GET /local/spawn-group-return-steps`
- `GET /local/spawn-group-return-steps/{entity_id}`
- `GET /local/maps/{map_index}/spawn-group-return-steps`

Rows are local/operator snapshots, not gameplay packets. Each row exposes `entity_id`, `ready_at`, `remaining_ms`, the current return-required spawn-group `actor`, and the planned `step` that the fixed-`max_step = 100` executor would apply on the next due flush. Rows are sorted by `entity_id`; already-due but unflushed schedules remain visible with `remaining_ms = 0`; stale schedules whose actor is gone, dead, no longer spawn-backed, no longer `return_required`, or no longer safely plannable are omitted and return `404` for exact lookup. Successful content-bundle replacement also prunes those stale pending deadlines after the replacement commits so a removed actor's old due time cannot later fire against unrelated replacement content. A canonical no-op content-bundle import runs the same pruning pass without rewriting content or replacing the still-valid due time for a live return-required actor. The map-local endpoint returns the same row shape filtered by the pending actor's current effective `map_index`, returns an empty JSON array for a known map with no pending return-step timers, rejects malformed or zero map indexes with `400`, and returns `404` when the runtime cannot resolve that map-scoped snapshot.

The paired read-only map-local leash surface is `GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>`. It returns the same leash snapshot rows as the exact actor endpoint, filtered to the materialized spawn-backed actors on one effective runtime map and sorted by the existing static-actor snapshot order. It returns an empty JSON array for a known map that has static actors but no spawn-backed actors, rejects malformed or zero map indexes and missing/non-positive radii with `400`, and returns `404` when the runtime cannot resolve that map-scoped snapshot. Like the exact leash endpoint, it is local/operator inspection only and does not mutate position, HP, death/respawn timers, engagement, target ownership, pending return-step deadlines, or visible-world membership.

A spawn-backed actor whose default leash classification is already `return_required` is now deliberately outside the owned stationary combat loop until a later return/chase slice moves or rebuilds it back into leash. Fresh `TARGET` selection for that actor fails closed with no self frame and the shared-world attempt seam reports `target_return_required` instead of the generic non-targetable reason; a stale already-selected `ATTACK` against an actor that became return-required also fails closed with the same explicit reason before HP mutation, engagement, immediate retaliation, delayed retaliation, damage-info, reward, or respawn side effects. Actors that still classify `at_home` or `within_radius` keep using the existing target -> attack -> death -> respawn contract.

The first live consumer of that preserved home was respawn, not chase AI. When a spawn-backed combatant respawns after its server-owned dead timer, the runtime restores the materialized actor position to the preserved authored home before rebuilding visibility. Viewers that only saw the old displaced runtime position receive the ordinary `CHARACTER_DEL`, viewers that only see the authored home receive the normal add/info/update burst, and viewers that can see both receive the usual delete-plus-readd refresh. The respawned actor reports `status = at_home`, full bootstrap HP, and no active target binding.

The next controlled consumer is the loopback-only return-home trigger:
- endpoint: `POST /local/spawn-groups/{entity_id}/return-home`
- scope: `gamed` local/operator tooling only
- request body: none
- success response: the same spawn-group leash snapshot shape, now reporting the actor at authored `home` / `current` with `status = at_home`

On success, the trigger returns one live spawn-backed actor's materialized position to its preserved authored home. That applies to any non-home live displacement, including actors still classified as `within_radius`; the trigger is not limited to `return_required` recovery. For displaced actors, the runtime persists the static-actor snapshot update before mutating runtime state; if that save fails, the trigger fails closed before position, engagement, selected-target ownership, automatic return-step schedule cleanup, or queued visibility frames change. For actors already exactly at authored home, no static-actor snapshot write is attempted; the trigger still performs the lifecycle reset so selected-target ownership and aggro-lite engagement do not stay stale just because persistence is temporarily unavailable. It does not change HP, death timers, reward descriptors, or combat profile metadata. It releases any current practice-mob engagement, clears selected combat targets bound to that actor's visible `VID`, clears any pending automatic return-step deadline for that actor, and advances the actor's runtime combat snapshot version even when the actor was already at home and no coordinate mutation is needed. That at-home snapshot reset is intentional: stale session-local pending delayed retaliation beats must fail closed until the player freshly reselects/reengages the mob, and stale pre-return server-owned return-step deadlines must not fire after the actor is already back at authored home. This matches the existing explicit reset boundaries for actor update/removal. When the return changes visible-world membership, it reuses the owned static-actor visibility transition path: old-position-only viewers receive `CHARACTER_DEL`, home-position viewers receive the normal add/info/update burst, and same-map retained viewers receive one server-driven `MOVE` replication at authored home. Cross-map return-home keeps delete/readd because no return warp packet seam is owned yet. If the actor was still within the same visible set, retained viewers still receive that same-map `MOVE` at the authored home followed by target clears for sessions that had that actor selected. Removing a spawn-backed actor also clears any pending automatic return-step deadline for that entity ID after the removal commits, so stale lifecycle timers do not survive content/operator teardown.

The result classifies the current position as one of:
- `at_home`
  - current position equals authored/home position
  - `return_required = false`
- `within_radius`
  - current position is on the same map and within the leash radius
  - `return_required = false`
- `return_required`
  - current position is outside the leash radius or on a different map
  - `return_required = true`
  - `return_target` is the authored/home position

The first distance rule is Euclidean squared-distance on the current bootstrap `x` / `y` coordinates. Cross-map positions always require return to the authored spawn position.

The runtime-facing JSON snapshot for a materialized spawn group contains:
- `actor` — the same spawn-backed static actor row exposed by `/local/spawn-groups/{entity_id}`
- `home` — `{map_index,x,y}` from the authored spawn placement
- `current` — `{map_index,x,y}` for the current runtime actor position
- `radius`
- `status`
- `return_required`
- optional `return_target` only when `return_required = true`

The exact read-only loopback endpoint remains `GET /local/spawn-groups/{entity_id}/leash?radius=<positive-int>`, with `GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` exposing the same shape for every materialized spawn group on one effective map. These are operator/debug tooling only; they are not gameplay packets and they do not mutate actor position, target ownership, HP, death state, respawn timers, return-step schedules, or visible-world membership. Their results are still meaningful after a runtime actor-position update: `home` remains the authored spawn position while `current` reflects the materialized actor position at lookup time.

The mutating loopback return endpoints are deliberately separate:
- `POST /local/spawn-groups/{entity_id}/return-step?max_step=<positive-int>` applies one capped planned step only when the actor is currently `return_required`; invalid `max_step`, malformed entity IDs, missing/non-spawn actors, dead actors waiting on respawn, or actors whose static snapshot cannot be stepped safely fail closed.
- `POST /local/spawn-groups/{entity_id}/return-home` performs the controlled exact-home return described above and returns `404` for missing/non-spawn actors, dead actors waiting on respawn, or actors whose static snapshot cannot be returned safely.
- `GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` returns `400` for malformed map indexes or missing/non-positive radii, `404` for unknown maps, and a deterministic row array for known maps, including an empty array when no materialized spawn groups belong to that map.
- `GET /local/spawn-group-return-steps/{entity_id}` returns `400` for malformed entity IDs and `404` for absent, stale, non-spawn, dead, in-radius, or unsafe pending return-step schedules.
- `GET /local/maps/{map_index}/spawn-group-return-steps` returns `400` for malformed map indexes, `404` for unknown maps, and the same deterministic pending-row array for known maps, including an empty array when no currently armed return-step schedule belongs to that map.

## Fail-closed cases

The classifier and return-step planner refuse to classify/plan:
- invalid zero-map home/current positions
- non-positive leash radii; the return-step planner also rejects non-positive `max_step`
- non-spawn actors without `spawn_group_ref`
- spawn actors whose `spawn_group_ref` or combat profile is invalid under the current runtime validators

## Why this seam exists now

The current practice-mob loop is still stationary, but the world lane needs safe boundaries before adding chase, leash, return, patrol, or target-switching behavior.

Freezing the classification and pure return-step planner first lets later slices add movement or server-origin AI steps without duplicating ad hoc distance/map checks in `internal/minimal`.

## Explicit non-goals

This slice does **not** yet implement:
- full autonomous mob movement or chase AI
- path-aware return cadence beyond the first due return-step applied from the existing pending-frame flush loop
- inventing cross-map return MOVE / `GC WARP` choreography (cross-map return is frozen as delete/readd / direct-home rebuild in the later section below)
- pathfinding, patrol routes, sectors, or navmesh logic
- aggro radius acquisition or target switching
- persistence of live mob position distinct from authored spawn position

The existing content-loaded practice mobs use the already-owned target -> attack -> death -> respawn lifecycle while they classify `at_home` or `within_radius`, plus the current capped return-step recovery path and the first pending-frame chase-step executor for already-engaged owners. Successful same-map chase steps, same-map return-step / return-home recovery, and same-map live spawn-backed operator/runtime position-only updates now replicate retained-viewer movement with server `MOVE`, while presentation/name/race refreshes, respawn rebuild, content-bundle replacement, and cross-map return-home still use delete/readd visibility. A materialized spawn-backed actor that already classifies `return_required` is kept visible/debuggable but is not accepted as a combat target again until an owned respawn, operator return-home, operator return-step, update, or server-owned return-step executor places it back inside leash; runtime attempt callers can now distinguish this specific gate as `target_return_required`. The exact and map-local `GET` leash endpoints are only read-only inspection bridges over that classifier, while the `POST` return-step and return-home endpoints are controlled local triggers for QA and lifecycle recovery, not final mob AI. The exact return-home trigger can also be used on a live `within_radius` mob to restore exact authored placement and reset selected-target/engagement ownership without changing HP or reward metadata; the one-step trigger leaves already-`within_radius` mobs untouched and selected, and the server-owned return-step executor stops re-arming as soon as a step brings the actor back inside that radius.

## First owned chase-step planning seam

The next tiny chase seam starts as a pure planner before any live mob movement executor exists.

Question frozen here:

**Given one engaged owner's current position and one live spawn-backed practice mob that still classifies `at_home` or `within_radius`, what is the smallest deterministic one-step chase plan the runtime can compute without inventing pathfinding, chase packets, or a second AI scheduler?**

Contract for `PlanStaticActorSpawnChaseStep(actor, ownerPosition, radius, max_step)`:
- fail closed for invalid/non-spawn actors, non-positive leash radius, non-positive `max_step`, or invalid owner positions
- if the actor currently classifies `return_required`, fail closed / do not plan chase; return-home ownership stays with the already-owned return-step seam
- if the actor and owner are on different maps, fail closed; no cross-map chase/warp choreography is owned yet
- if the actor is already exactly on the owner position, return that position with `complete = true` and no movement
- otherwise return one deterministic same-map x/y step toward the owner position, capped by `max_step`, using the same step math family as return-step planning
- the planned `next` must still classify `at_home` or `within_radius` against the actor's preserved authored home; if the uncapped step toward the owner would leave leash, clamp to the farthest on-segment point that remains inside leash and mark `complete = true` when that clamped point is reached
- by itself the planner never mutates actor position, never updates the static-actor store, never changes HP/death/engagement state, and never queues visibility or chase packets

Explicit non-goals for this chase-step planner freeze alone:
- live automatic chase execution from `FlushServerFrames()`
- server-driven `MOVE` fanout or chase packet families
- aggro-radius acquisition / target switching beyond the already-owned post-hit engagement gate
- pathfinding, navmesh, or multi-actor flocking

The pure planner helper is now implemented as `PlanStaticActorSpawnChaseStep` in `internal/worldruntime` with focused unit coverage for same-map chase steps, already-on-owner / within-one-step completion, leash-boundary clamping, and fail-closed return-required / cross-map / invalid-input cases. That helper is the prerequisite for the first live chase executor below.

## First owned chase-step executor seam

Question frozen here:

**Once `PlanStaticActorSpawnChaseStep` exists, what is the smallest server-owned live consumer that can apply one planned chase step for an already-engaged practice mob without inventing chase packets, a second AI scheduler, or aggro-radius acquisition?**

The first live chase executor deliberately mirrors the return-step pending-frame pattern and does not introduce a general mob goroutine loop.

Arming rules:
- arm one pending chase-step deadline only for a live spawn-backed practice mob that currently holds aggro-lite engagement ownership and still classifies `at_home` or `within_radius`
- arm from the owned post-hit engagement gate, from proximity aggro-radius acquisition that newly establishes that same engagement ownership, and from any later same-engagement accepted hit that keeps that ownership, using a bootstrap chase delay of `5s` and the same fixed `max_step = 100` / default leash radius family as return-step scheduling
- the chase delay is intentionally longer than the owned `1s` delayed retaliation beat so multi-beat hostility cadence remains independently observable before the first chase step
- never arm chase while the actor classifies `return_required`, is dead/waiting on respawn, lacks `spawn_group_ref`, or has no live same-map engaged owner
- return-step ownership always wins: if an actor becomes `return_required`, clear any pending chase deadline and leave recovery to the already-owned return-step executor

Execution rules:
- each `FlushServerFrames()` pass keeps the owned order of due respawns, then due return steps, then due homeward steps, then due chase steps, then proximity acquisition / session-local delayed retaliation
- a due chase step resolves the current engaged owner's live position, plans with fixed bootstrap `max_step = 100` and the default leash radius, persists the stepped materialized position before mutating runtime, and fans visibility with retained-viewer `MOVE` replication while remove/add membership still uses the ordinary static-actor delete/bootstrap helpers
- focused coverage now also freezes that live-owner replan when the engaged owner moves between chase arm and the first due flush: the due retained-viewer `MOVE` plans toward the post-move owner coords rather than an arm-time snapshot, while engagement / selected-target ownership stay preserved (`TestGameRuntimeFlushServerFramesReplansSpawnGroupChaseTowardOwnerMovedBetweenArmAndDue`)
- unlike return-step recovery, a successful chase step preserves current practice-mob engagement and does not clear selected combat targets solely because the actor moved; stale delayed retaliation remains governed by the already-owned engagement/reset seams
- if planning fails closed (lost owner, cross-map owner, return-required, dead, invalid actor), clear the pending chase deadline without mutating position
- if the planned step is a complete no-move because the actor already occupies the owner position, clear the pending chase deadline without persisting or queueing visibility frames
- if the step moves the actor and the post-step snapshot is still live, still engaged by the same owner, still same-map, and still `at_home` / `within_radius`, arm the next `5s` chase deadline; otherwise clear it
- leash-clamped complete steps that stop on the leash boundary clear the pending chase deadline even when the actor has not reached the owner; further chase requires a later owned re-arm after the actor is again safely inside leash and still engaged

Preflight rules:
- a fresh `EnterGame` / visibility bootstrap and a `MOVE` / `SYNC_POSITION` transfer rebootstrap that start after a pending chase-step deadline is already due must flush that due chase step before encoding static-actor visibility, matching the return-step preflight contract so clients do not first observe a stale pre-chase position followed by a redundant queued rebuild
- the owned same-socket `/restart_here` and `/restart_town` recovery seams use the same due chase-step preflight before rebuilding visibility; focused coverage now owns both recovery paths with a separate live engager keeping chase eligible while a floored restarter recovers

Cleanup / fail-closed rules:
- clear pending chase deadlines on owner disconnect/logout/close, phase-select leave, EnterGame reclaim that drops stale engagement ownership, owner death floor, owner transfer/warp to a different map, client-originated `TARGET(0)` clear-target that releases the current engagement, actor death, successful return-home / return-step, operator/runtime `UpdateStaticActor` (including same-map position-only MOVE updates that release engagement), content-bundle replacement that removes or replaces the actor, and any other engagement release that drops the actor's `engaged_by` ownership
- proximity-only leave-radius walk-away (no selected combat target) is one of those engagement-release boundaries: walking outside the actor's effective aggro radius releases `engaged_by`, clears pending chase, and cancels delayed retaliation without inventing self `TARGET(0, 0)`
- hit-armed / still-selected engagement is intentionally asymmetric: walking outside aggro while the owner still holds a selected combat target for that actor and remains inside combat-target range / leash / visibility must **not** release engagement or clear pending chase solely because aggro radius was left; chase / retaliation continue under the owned hit-engagement rules until an explicit release boundary such as `TARGET(0)`, death floor, disconnect/transfer, return recovery, operator update, or combat-range loss clear
- focused shared-world coverage now freezes that hit-armed asymmetry twin beside the proximity-only release proof: `TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius` keeps `engaged_by`, selected combat target, and the pending chase deadline after the owner walks outside aggro, continues delayed retaliation, and still applies the due retained-viewer chase `MOVE`
- dead actors waiting on respawn do not arm chase; a respawn rebuild starts unengaged at authored home and therefore does not inherit a pre-death chase deadline
- no new operator chase-step POST surface is required for this first executor freeze
- profile-authored per-mob chase arming delay beyond the bootstrap `5s` default is owned as optional `combat_profiles.chase_delay_ms` in `content-spawn-groups-bootstrap.md`; live arming / re-arm consume `EffectiveStaticActorSpawnChaseDelay` for the actor's combat profile (omit/zero keeps bootstrap `5s`)
- profile-authored per-mob return-step arming delay beyond the bootstrap `1s` default is owned as optional `combat_profiles.return_delay_ms` in `content-spawn-groups-bootstrap.md`; live return arming / re-arm consume `EffectiveStaticActorSpawnReturnDelay` for the actor's combat profile (omit/zero keeps bootstrap `1s`)

## First owned pending chase-step inspection seam

Question frozen here:

**Once the pending-frame chase executor is live, what is the smallest read-only loopback inspection surface that can expose currently armed chase-step deadlines without inventing a chase POST trigger, chase packets, or a second scheduler?**

The paired read-only pending-chase schedule surfaces mirror return-step inspection:
- `GET /local/spawn-group-chase-steps`
- `GET /local/spawn-group-chase-steps/{entity_id}`
- `GET /local/maps/{map_index}/spawn-group-chase-steps`

Rows are local/operator snapshots, not gameplay packets. Each row exposes:
- `entity_id`
- `ready_at`
- `remaining_ms`
- the current still-eligible spawn-group `actor`
- the planned chase `step` that the fixed-`max_step = 100` / default-leash executor would apply on the next due flush (`home` / `current` / `radius` / `status` / `return_required` plus `next` and `complete`)

Row rules:
- rows are sorted by `entity_id`
- already-due but unflushed schedules remain visible with `remaining_ms = 0`
- stale schedules whose actor is gone, dead, no longer spawn-backed, `return_required`, no longer engaged by a live same-map owner, or no longer safely plannable are omitted and return `404` for exact lookup
- the map-local endpoint returns the same row shape filtered by the pending actor's current effective `map_index`, returns an empty JSON array for a known map with no pending chase-step timers, rejects malformed or zero map indexes with `400`, and returns `404` when the runtime cannot resolve that map-scoped snapshot
- `GET /local/spawn-group-chase-steps/{entity_id}` returns `400` for malformed entity IDs and `404` for absent/stale/ineligible pending chase-step schedules
- these endpoints never mutate actor position, engagement, selected-target ownership, HP, death/respawn timers, return-step schedules, chase deadlines, or visible-world membership
- no `POST` chase-step operator surface is owned by this inspection freeze

Current implementation status:
- the pending-frame chase executor is now live in `internal/minimal`
- accepted non-lethal content practice-mob hits arm the `5s` chase deadline
- proximity aggro-radius acquisition that newly establishes engagement also arms that same `5s` chase deadline without inventing selected-target ownership; when the deadline becomes due, the executor applies the owned chase MOVE choreography for retained viewers while still preserving engagement and still inventing no selected combat target
- due chase steps persist position, queue retained-viewer `MOVE` replication (with remove/add visibility still using delete/bootstrap), preserve engagement / selected-target ownership, and re-arm while the actor remains eligible
- focused live-owner replan coverage now freezes owner movement between chase arm and the first due flush: the due retained-viewer `MOVE` plans toward the live post-move owner coords rather than an arm-time snapshot (`TestGameRuntimeFlushServerFramesReplansSpawnGroupChaseTowardOwnerMovedBetweenArmAndDue`)
- leash-clamped complete chase steps that stop on the effective leash boundary now have focused live coverage: the pending chase deadline clears even when the owner was not reached, engagement / selected-target stay preserved, no automatic follow-up fires while cleared, a later same-engagement accepted hit re-arms the owned `5s` deadline, and after the owner walks inward so the actor is again safely inside leash the re-armed due chase applies another retained-viewer `MOVE` (`TestGameRuntimeFlushServerFramesClearsLeashClampedSpawnGroupChaseStepAndRearmsOnHit`)
- return-step, respawn, remove, return-home, operator/runtime `UpdateStaticActor`, and content-bundle prune/restore paths clear or restore chase deadlines alongside the return-step schedule; focused coverage now also proves that a same-map position-only `UpdateStaticActor` clears any armed chase deadline so a stale `5s` chase MOVE cannot fire after that engagement-release boundary
- the read-only pending chase inspection endpoints above are now live over that already-owned schedule

Explicit non-goals for this chase-step executor freeze:
- pathfinding, navmesh, patrol, or multi-actor flocking
- chasing while `return_required` or across map boundaries
- persisting a live mob position schema distinct from the current static-actor snapshot path
- operator POST chase-step triggers

## First owned chase MOVE packet choreography seam

Question frozen here:

**Once the pending-frame chase executor already applies one planned same-map step for a still-engaged practice mob, what is the smallest client-visible packet change that replaces the retained-viewer delete/readd refresh with a server-driven `MOVE` replication without inventing pathfinding, return-step MOVE fanout, or a second AI scheduler?**

Contract for the first chase MOVE choreography:
- reuse the already-owned server `MOVE` / `MOVE_ACK` wire shape (`0x0302`) from `move-peer-fanout.md` rather than inventing a dedicated chase packet family
- apply only to a successful pending-frame chase step that actually changes the materialized actor position while the actor remains live, engaged by the same owner, same-map, and still `at_home` / `within_radius`
- retained viewers that already had the actor visible before and after the step receive one queued `MOVE` replication for the actor's visible `VID` at the planned `next` coordinates instead of the current delete-plus-readd refresh
- viewers that lose visibility across the step still receive `CHARACTER_DEL`
- viewers that newly gain visibility across the step still receive the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` bootstrap burst
- chase MOVE fanout does **not** clear selected combat targets, does **not** release aggro-lite engagement, and does **not** invent selected-target ownership for proximity-armed chase
- at the time of the chase MOVE freeze, return-step recovery, return-home, respawn rebuild, operator actor updates, and content-bundle replacement kept using their already-owned delete/readd visibility paths; the later return-step / return-home MOVE seam and the later same-map live operator/runtime position MOVE seam now cover same-map retained viewers while presentation refreshes / respawn / content-bundle replacement / cross-map return-home remain on delete/readd
- no client-originated mob MOVE ingress is owned; this is server-origin replication only
- no chase-specific duration/interpolation policy is owned beyond reusing the existing `MOVE` payload fields with a deterministic bootstrap duration suitable for the fixed `max_step = 100` step

Current implementation status:
- this chase MOVE choreography is now live for retained viewers of a successful pending-frame chase step
- remove/add visibility membership across the same step still uses the ordinary `CHARACTER_DEL` / add-info-update bootstrap path
- same-map return-step / return-home retained-viewer MOVE and same-map live operator/runtime position MOVE later landed as separate seams; presentation refreshes, respawn rebuild, content-bundle replacement, and cross-map return-home remain on delete/readd

Explicit non-goals for this chase MOVE freeze alone:
- pathfinding beyond one discrete planned chase step (return-step / return-home MOVE fanout later landed separately)
- pathfinding, navmesh, patrol, or continuous interpolation beyond one discrete planned step
- cross-map chase or chase while `return_required`
- a dedicated chase packet family distinct from `MOVE`
- operator POST chase-step triggers

## First owned return-step MOVE packet choreography seam

Question frozen here:

**Once retained-viewer chase steps already reuse server `MOVE` replication, what is the smallest honest packet change that can also replace retained-viewer delete/readd for a successful same-map return-step (and exact return-home) without inventing pathfinding, chase/return packet families, or a second AI scheduler?**

Contract for the first return-step / return-home MOVE choreography:
- reuse the already-owned server `MOVE` / `MOVE_ACK` wire shape (`0x0302`) from `move-peer-fanout.md` and the same bootstrap duration family already used by chase MOVE, rather than inventing a dedicated return packet family
- apply to a successful pending-frame / operator return-step that actually changes the materialized actor position while the actor remains live and still on the same map as its authored home, and to a successful same-map `return-home` that actually changes coordinates
- retained viewers that already had the actor visible before and after the step / home return receive one queued `MOVE` replication for the actor's visible `VID` at the planned `next` / authored home coordinates instead of the current delete-plus-readd refresh
- viewers that lose visibility across the step / home return still receive `CHARACTER_DEL`
- viewers that newly gain visibility across the step / home return still receive the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` bootstrap burst
- return-step / return-home MOVE fanout still releases practice-mob engagement and still clears selected combat targets bound to that actor's visible `VID`, matching the already-owned return recovery lifecycle (this is intentionally different from chase MOVE, which preserves engagement / selected-target ownership)
- cross-map return-home remains outside MOVE choreography for now and keeps the existing delete/readd / direct-home rebuild path because no client-facing return warp packet seam is owned yet
- respawn rebuild, presentation/name/race operator refreshes, and content-bundle replacement keep using their already-owned delete/readd visibility paths; this freeze does not convert those seams to MOVE (the later same-map live operator/runtime position MOVE seam covers position-only updates separately)
- no client-originated mob MOVE ingress is owned; this is server-origin replication only
- no return-specific duration/interpolation policy is owned beyond reusing the existing `MOVE` payload fields with a deterministic bootstrap duration suitable for the fixed `max_step = 100` return-step and the exact-home return trigger

Current implementation status:
- this return-step / return-home MOVE choreography is now live for retained viewers of a successful same-map pending-frame / operator return-step and same-map return-home
- remove/add visibility membership across the same step / home return still uses the ordinary `CHARACTER_DEL` / add-info-update bootstrap path
- cross-map return-home, respawn rebuild, presentation/name/race operator refreshes, and content-bundle replacement remain on delete/readd
- engagement release and selected-target clear still follow the already-owned return recovery lifecycle
- cross-map return-home / return-step membership is also part of the Track A #6 anti-leak matrix in `content-spawn-groups-bootstrap.md`: a successful cross-map return must restore exactly one entity to authored home and leave no dual-map occupancy / duplicate `spawn_group_ref`; focused coverage now owns both the operator exact-home trigger and the automatic pending-frame return-step flush after a cross-map `UpdateStaticActor` displace (delete/readd, no invented MOVE)

Explicit non-goals for this return-step MOVE freeze alone:
- pathfinding, navmesh, patrol, or continuous interpolation beyond one discrete planned return step / exact-home snap
- converting respawn rebuild or presentation/name/race operator refreshes to MOVE
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild below)
- a dedicated return packet family distinct from `MOVE`
- changing the already-owned engagement-release / selected-target-clear semantics of return recovery

## First owned same-map live operator/runtime position MOVE seam

Question frozen here:

**Once chase and same-map return-step / return-home already reuse server `MOVE` for retained viewers, what is the smallest honest packet change that can also replace retained-viewer delete/readd for a live same-map spawn-backed operator/runtime position update without inventing pathfinding, a second AI scheduler, or converting presentation refreshes / respawn rebuild to MOVE?**

Contract for the first live operator/runtime position MOVE choreography:
- reuse the already-owned server `MOVE` / `MOVE_ACK` wire shape (`0x0302`) and the same bootstrap duration family already used by chase / return MOVE
- apply only when a live spawn-backed practice mob (`spawn_group_ref` present, current HP > 0) receives an operator/runtime update that keeps the same map, same presentation identity (`name` / `race_num` / combat profile), and only changes the materialized current coordinates
- retained viewers that already had the actor visible before and after the position update receive one queued `MOVE` replication for the actor's visible `VID` at the updated coordinates instead of the current delete-plus-readd refresh
- viewers that lose visibility across the update still receive `CHARACTER_DEL`
- viewers that newly gain visibility across the update still receive the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` bootstrap burst
- presentation changes (name / race / combat profile), dead-actor refreshes that must replay trailing `GC DEAD`, cross-map updates, non-spawn static actors, respawn rebuild, and content-bundle replacement keep using their already-owned delete/readd paths
- engagement release, selected-target clear, and pending chase-deadline clear for this operator/runtime update seam continue to follow the already-owned update lifecycle (they are released/cleared today and this freeze does not invent chase-like ownership preservation)
- no client-originated mob MOVE ingress is owned; this is server-origin replication only

Current implementation status:
- this seam is now live for retained viewers of a successful same-map live spawn-backed operator/runtime position-only update
- remove/add visibility membership across that same position MOVE is now owned by focused coverage: old-position-only viewers receive `CHARACTER_DEL`, newly-visible destination viewers receive the ordinary add/info/update burst, and retained midway viewers still receive only `MOVE` (`TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd`)
- chase / return-step / return-home / homeward already reuse the same `RelocateStaticActorTargetDiff` remove/add fanout; dedicated multi-viewer twins for those executors are optional symmetry proofs, not a missing runtime seam
- presentation/name/race refreshes, dead trailing-`DEAD` refreshes, respawn rebuild, content-bundle replacement, and cross-map updates remain on delete/readd
- engagement release, selected-target clear, and pending chase-deadline clear still follow the already-owned operator/runtime update lifecycle
- cross-map return client choreography is frozen as delete/readd / direct-home rebuild (not a future MOVE/WARP seam); live damaged-HP daemon-restart durability is owned separately by the content-spawn-groups daemon-restart seam
- absolute pending chase / return / homeward deadline persistence across daemon restart is frozen as **re-arm-from-now** rather than absolute mid-timer due-at rematerialize: `loadPersistedStaticActors` re-arms eligible return/homeward from now when the rematerialized actor classifies `return_required` / unengaged `within_radius`; chase stays unarmed across restart until fresh post-restart engagement; engagement / selected-target ownership stay fail-closed across restart beside the owned proximity-suppress rematerialize; speculative absolute due-at rematerialize RED is cancelled for Track A bootstrap (see [absolute chase/return/homeward deadline rematerialize contract freeze](../../docs/plans/2026-08-25-absolute-chase-return-homeward-deadline-rematerialize-contract-freeze.md)); focused composite coverage now also owns that posture in one restart twin (`TestGameRuntimeDaemonRestartRearmsReturnAndHomewardFromNowAndLeavesChaseUnarmed`)

Explicit non-goals for this operator/runtime position MOVE freeze alone:
- converting presentation/name/race refreshes, dead trailing-`DEAD` refreshes, respawn rebuild, or content-bundle replacement to MOVE
- inventing cross-map operator MOVE / `GC WARP` choreography (cross-map return stays on frozen delete/readd)
- preserving engagement / selected-target ownership across operator position updates
- pathfinding, navmesh, patrol, or continuous interpolation beyond one discrete operator/runtime coordinate write
- a dedicated operator-move packet family distinct from `MOVE`
- daemon-restart persistence of live damaged HP above the death floor; that seam is owned separately by the content-spawn-groups daemon-restart follow-on beside still-dead timer persistence

## First owned profile-authored leash-radius seam

Question frozen here:

**Once leash / chase / return consumers already hard-code `DefaultSpawnLeashRadius = 400` and optional authored `aggro_radius` already exists, what is the smallest honest authored combat-profile extension that can widen or narrow that leash radius per registered profile without inventing pathfinding or cross-map return MOVE?**

The full portable-bundle / registration / effective-radius contract lives in `content-spawn-groups-bootstrap.md` under "First owned profile-authored leash-radius seam". This document only records the leash-lane consumer expectation:

- `EffectiveStaticActorSpawnLeashRadius(profile)` / `...ForActor(actor)` resolve omitted/zero to `DefaultSpawnLeashRadius` (`400`)
- live classification, return-step / return-home, chase-step planning/execution, and `target_return_required` gating reuse that effective radius
- operator leash GET endpoints may keep an explicit query `radius` override; defaulted lookups use the actor's effective leash radius
- positive authored leash below the profile's effective aggro radius fails closed
- cross-map return client choreography stays on the frozen delete/readd / direct-home rebuild path below; this seam does not invent MOVE/WARP

Current implementation status:
- optional authored `leash_radius` is owned on portable `combat_profiles` / `StaticActorCombatProfileDefaults`
- `EffectiveStaticActorSpawnLeashRadius(profile)` / `...ForActor(actor)` resolve omitted/zero to `DefaultSpawnLeashRadius` (`400`)
- live classification, return-step / return-home, chase-step planning/execution, and `target_return_required` gating reuse that effective radius
- operator leash GET endpoints keep an explicit query `radius` override; omitted lookups resolve through the actor's effective leash radius
- positive authored leash below the profile's effective aggro radius fails closed; positive authored aggro must also stay within the profile's effective leash

## Frozen: cross-map return client choreography stays delete/readd

Question frozen here:

**Once same-map chase / return-step / return-home / operator position MOVE already reuse server `MOVE`, what is the smallest honest client-visible choreography for a successful cross-map return-home (or cross-map return-step) without inventing pathfinding, a second AI scheduler, or a guessed warp packet family?**

Answer (now frozen for Track A bootstrap scope):

Cross-map return keeps the already-owned delete/readd / direct-home rebuild path. It is **not** waiting on a future cross-map mob `MOVE` or `GC WARP` seam.

Owned packet / membership contract:
- origin-map / old-position viewers receive ordinary `CHARACTER_DEL` only; no warp/teardown companion frame is owned for non-player return
- home-map / newly visible viewers receive the ordinary bootstrap reappearance burst: `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE`
- no cross-map retained-viewer `MOVE` is invented for spawn-backed practice mobs
- no `GC WARP` / player-teleport packet is emitted for non-player return; legacy `WarpSet` / `TPacketGCWarp` is PC-only (`!IsPC()` fails closed) and carries `x/y/addr/port` for player map-server teleport, not mob leash recovery
- legacy mob map/sectree relocation evidence aligns with `CHARACTER::Show` remove/view-cleanup + insert/`EncodeInsertPacket` rather than WARP or cross-map MOVE
- a successful cross-map return restores exactly one runtime entity for the authored `spawn_group_ref` onto the authored home map and must leave no dual-map occupancy or duplicate membership behind
- engagement / selected-target / chase / return / homeward ownership fail closed across that map change exactly as the owned delete/readd recovery already does
- same-map retained-viewer `MOVE` choreography stays owned and unchanged

Current implementation status:
- operator `POST .../return-home` and automatic pending-frame return-step flush after a cross-map `UpdateStaticActor` displace already prove foreign-map `CHARACTER_DEL`, home-map add/info/update, cleared pending schedule, persisted authored home, one-ref/one-actor, empty foreign-map occupancy, and no invented cross-map `MOVE` (`TestGameRuntimeFlushServerFramesAppliesDueCrossMapSpawnGroupReturnStepLeavesNoDualMapOccupancy` plus the operator dual-map proof)
- this freeze closes the earlier “deferred until a packet boundary is owned” placeholder: the packet boundary is the already-owned delete/readd families above
- speculative RED that asserts cross-map `MOVE` or `WARP` frames for practice-mob return is cancelled for Track A bootstrap scope

Explicit non-goals for this freeze:
- inventing cross-map mob `MOVE` or `GC WARP` choreography for spawn-backed return
- converting same-map return / chase / homeward / operator position `MOVE` back to delete/readd
- pathfinding, patrol, or continuous interpolation
- live damaged-HP daemon-restart durability (owned separately by `content-spawn-groups-bootstrap.md`)

## First owned within-radius homeward-step after engagement release

Question frozen here:

**Once chase can leave a still-live practice mob classified `within_radius` (not `at_home`, not `return_required`) and engagement release already clears the chase deadline, what is the smallest server-owned recovery that can step that unengaged actor back toward authored home without inventing pathfinding, changing the return-step no-op contract for `within_radius`, or opening cross-map warp choreography?**

Contract for `PlanStaticActorSpawnLeashHomewardStep(actor, radius, max_step)`:
- fail closed for invalid/non-spawn actors, non-positive leash radius, non-positive `max_step`, or `return_required` classification (outside-leash recovery stays with `PlanStaticActorSpawnLeashReturnStep`)
- if the actor currently classifies `at_home`, return a complete no-op at the current/home position
- if the actor currently classifies `within_radius` on the same map as authored home, return one deterministic x/y step toward authored home capped by `max_step`, using the same step math family as return-step / chase planning; mark `complete = true` when the planned `next` equals authored home
- by itself the planner never mutates actor position, never updates the static-actor store, never changes HP/death/engagement state, and never queues visibility frames

Live pending-frame executor rules:
- arm one pending homeward deadline (`1s`, fixed `max_step = 100`) only for a live spawn-backed practice mob that currently lacks `engaged_by` ownership and classifies `within_radius`
- profile-authored per-mob homeward-step arming delay beyond the bootstrap `1s` default is owned as optional `combat_profiles.homeward_delay_ms` in `content-spawn-groups-bootstrap.md`; live homeward arming / re-arm consume `EffectiveStaticActorSpawnHomewardDelay` for the actor's combat profile (omit/zero keeps bootstrap `1s`)
- arm from engagement-release paths that also clear chase (client `TARGET(0)`, proximity leave-radius walk-away, leave/logout/close, phase-select, transfer, EnterGame reclaim, owner death floor, and other owned engagement releases) after the actor is already displaced `within_radius`
- arm on runtime startup / daemon rematerialization when a persisted live spawn-backed actor already classifies unengaged `within_radius`, mirroring how `return_required` rematerialization arms return-step
- never arm while still engaged, `at_home`, `return_required`, dead/waiting on respawn, or non-spawn
- each `FlushServerFrames()` pass keeps the order of due respawns, then due `return_required` return-steps, then due homeward steps, then due chase steps, then proximity acquisition / delayed retaliation
- a due homeward step plans, persists the stepped materialized position, fans retained-viewer same-map `MOVE` (remove/add still delete/bootstrap), keeps the actor unengaged, and re-arms while still eligible `within_radius`; landing on authored home clears the deadline
- re-engage / chase eligibility clears any pending homeward deadline so chase and homeward never both own the same actor
- EnterGame / transfer / restart preflights flush due homeward before encoding static-actor visibility, matching return-step and chase preflight; focused shared-world coverage now freezes the already-due EnterGame, `/restart_here`, and `/restart_town` catch-up paths for homeward the same way return/chase already own them

Current implementation status:
- pure planner `PlanStaticActorSpawnLeashHomewardStep` is owned in `internal/worldruntime`
- pending-frame homeward executor, engagement-release arming, chase mutual exclusion, and same-map retained-viewer `MOVE` are live in `internal/minimal`
- focused coverage now also freezes owner death-floor engagement release after chase leaves the mob `within_radius`: immediate and delayed floor paths keep the homeward schedule armed by `clearActiveCombatTarget` instead of clearing it, the still-connected dead owner is skipped for retained homeward `MOVE`, and a living retained watcher receives the due homeward `MOVE` back to authored home
- focused multi-step homeward cadence coverage now freezes chase displace beyond one `max_step=100` beat: engagement release arms homeward, the first due homeward `MOVE` re-arms while still `within_radius`, and the second due lands on authored home / `at_home` and clears the deadline (`TestGameRuntimeFlushServerFramesAppliesMultiStepSpawnGroupHomewardCadenceAfterChaseDisplaceBeyondMaxStep`)
- focused EnterGame / MOVE-transfer / `/restart_here` / `/restart_town` due-homeward preflight coverage now mirrors the owned chase/return preflight proofs
- daemon-restart rematerialization of live unengaged `within_radius` spawn-backed actors now arms pending homeward through `loadPersistedStaticActors`
- operator `POST .../return-step` still no-ops `within_radius`; exact-home snap remains the controlled `return-home` trigger
- the read-only pending homeward inspection endpoints below are now live over that already-owned schedule
- operator/runtime same-map position `UpdateStaticActor` that leaves an unengaged spawn-backed actor `within_radius` now re-arms pending homeward through the shared eligibility sync (mirroring `return_required` return-step re-arm) instead of only clearing the deadline

## First owned pending homeward-step inspection seam

Question frozen here:

**Once the pending-frame homeward executor is live, what is the smallest read-only loopback inspection surface that can expose currently armed homeward-step deadlines without inventing a homeward POST trigger, homeward packets, or a second scheduler?**

The paired read-only pending-homeward schedule surfaces mirror chase/return-step inspection:
- `GET /local/spawn-group-homeward-steps`
- `GET /local/spawn-group-homeward-steps/{entity_id}`
- `GET /local/maps/{map_index}/spawn-group-homeward-steps`

Rows are local/operator snapshots, not gameplay packets. Each row exposes:
- `entity_id`
- `ready_at`
- `remaining_ms`
- the current still-eligible spawn-group `actor`
- the planned homeward `step` that the fixed-`max_step = 100` / default-leash executor would apply on the next due flush (`home` / `current` / `radius` / `status` / `return_required` plus `next` and `complete`)

Row rules:
- rows are sorted by `entity_id`
- already-due but unflushed schedules remain visible with `remaining_ms = 0`
- stale schedules whose actor is gone, dead, no longer spawn-backed, `return_required`, no longer `within_radius`, re-engaged by a live owner, or no longer safely plannable are omitted and return `404` for exact lookup
- the map-local endpoint returns the same row shape filtered by the pending actor's current effective `map_index`, returns an empty JSON array for a known map with no pending homeward-step timers, rejects malformed or zero map indexes with `400`, and returns `404` when the runtime cannot resolve that map-scoped snapshot
- `GET /local/spawn-group-homeward-steps/{entity_id}` returns `400` for malformed entity IDs and `404` for absent/stale/ineligible pending homeward-step schedules
- these endpoints never mutate actor position, engagement, selected-target ownership, HP, death/respawn timers, return-step schedules, chase deadlines, homeward deadlines, or visible-world membership
- no `POST` homeward-step operator surface is owned by this inspection freeze

Current implementation status:
- the pending-frame homeward executor is now live in `internal/minimal`
- engagement-release paths that clear chase after a `within_radius` displace arm the `1s` homeward deadline
- due homeward steps persist position, queue retained-viewer `MOVE` replication (with remove/add visibility still using delete/bootstrap), keep the actor unengaged, and re-arm while the actor remains eligible `within_radius`
- focused multi-step homeward cadence coverage now freezes that re-arm path after chase displace beyond one `max_step` (`TestGameRuntimeFlushServerFramesAppliesMultiStepSpawnGroupHomewardCadenceAfterChaseDisplaceBeyondMaxStep`)
- re-engage / chase eligibility, return-step, respawn, remove, return-home, operator/runtime `UpdateStaticActor`, and content-bundle prune/restore paths clear or restore homeward deadlines alongside the chase/return schedules
- `ImportContentBundle` now mirrors return/chase schedule ownership for homeward: identical no-op reimports prune stale ineligible homeward deadlines while preserving still-eligible due times, successful non-identical replacements prune removed/stale homeward deadlines before import fanout, and failed replacement rollback restores the pre-import homeward due-at snapshot for still-eligible actors
- the read-only pending homeward inspection endpoints above are now live over that already-owned schedule
- focused EnterGame, `/restart_here`, and `/restart_town` due-homeward catch-up coverage now mirrors the owned return/chase preflight proofs so skipped zero-HP lifecycle frames cannot leave stale within_radius visuals behind
- daemon-restart rematerialization of live unengaged `within_radius` spawn-backed actors now arms pending homeward
- operator/runtime same-map position `UpdateStaticActor` that leaves a live unengaged spawn-backed actor `within_radius` now re-arms pending homeward through `syncSpawnGroupHomewardStepScheduleForEntity` (and still clears for `at_home` / `return_required` / dead / engaged / non-spawn)

Explicit non-goals for this homeward freeze alone:
- auto exact-home correction for every `within_radius` actor without a prior engagement/chase displacement boundary beyond the owned arming rules
- changing `PlanStaticActorSpawnLeashReturnStep` so `within_radius` starts moving
- inventing cross-map homeward MOVE / `GC WARP` choreography (cross-map return stays on frozen delete/readd)
- pathfinding, patrol, or a second scheduler/goroutine
- operator POST homeward trigger
- inventing selected-target ownership or preserving engagement across homeward

## Done: operator/runtime UpdateStaticActor re-arms within-radius homeward

Question frozen here:

**Once engagement-release and daemon-restore already arm pending homeward for live unengaged `within_radius` spawn-backed actors, and operator/runtime same-map position `UpdateStaticActor` already re-arms return-step when the actor classifies `return_required`, what is the smallest honest follow-on so that same update path re-arms homeward when it leaves the actor unengaged `within_radius` instead of only clearing the deadline?**

Contract (now GREEN):
- after a successful same-map position-only operator/runtime `UpdateStaticActor` on a live spawn-backed practice mob, call the same homeward eligibility sync used by engagement-release / restore
- if the post-update actor is live, unengaged, spawn-backed, and classifies `within_radius`, arm one pending homeward deadline (`1s`, fixed `max_step = 100`)
- if the post-update actor is `at_home`, `return_required`, dead, engaged, or non-spawn, clear any pending homeward deadline (return-step ownership still wins for `return_required`)
- keep the existing engagement / selected-target / chase clear behavior on operator/runtime update
- do not invent a homeward POST trigger, pathfinding, or cross-map homeward choreography

Current implementation status:
- `UpdateStaticActor` now calls `syncSpawnGroupHomewardStepScheduleForEntity` after syncing return-step and clearing chase
- focused coverage proves arming for unengaged `within_radius` plus due homeward flush that restores `at_home` without arming return-step
- the older "within_radius never auto-moves" isolation proof is replaced by that homeward-after-update / return-step-idle proof

Automatic pending-frame cross-map return-step dual-map anti-leak after `UpdateStaticActor` is now owned beside operator return-home (`TestGameRuntimeFlushServerFramesAppliesDueCrossMapSpawnGroupReturnStepLeavesNoDualMapOccupancy`). Content-bundle homeward schedule prune/restore is now owned beside the return-step proofs (`TestGameRuntimeFailedContentBundleImportRestoresSpawnGroupHomewardStepSchedule`, `TestGameRuntimeNoOpContentBundleImportPrunesStaleSpawnGroupHomewardStepSchedule`, `TestGameRuntimeSuccessfulContentBundleReplacementClearsStaleSpawnGroupHomewardStepSchedule`). Proximity-armed owner death-floor → same-socket `/restart_here` leave/re-enter suppress is now owned beside `TARGET(0)` / death/respawn suppress seeding (`TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere`). That same suppress now also survives `/phase_select` and abrupt reconnect Leave → Join identity changes via VID park/claim before `/restart_here` (`TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorPhaseSelectRestartHere`, `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorReconnectRestartHere`). Cross-map return client choreography is now frozen as the already-owned delete/readd / direct-home rebuild path above; speculative cross-map `MOVE` / `GC WARP` RED for practice-mob return is cancelled for Track A bootstrap scope.
