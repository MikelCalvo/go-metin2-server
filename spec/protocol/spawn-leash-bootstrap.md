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

On success, the trigger plans one return step with the default bootstrap leash radius, persists the materialized static-actor position to `step.next`, then mutates runtime state through the same static-actor visibility transition helpers used by operator actor updates. Retained viewers receive the normal delete-plus-readd refresh at the stepped position; old-position-only viewers receive `CHARACTER_DEL`; newly visible viewers receive the normal add/info/update burst. A moving step preserves HP, death timers, reward descriptors, and combat profile metadata, but now releases current practice-mob engagement, clears selected combat targets bound to that actor's visible `VID`, and advances the runtime combat snapshot version so stale selected `ATTACK` and stale delayed retaliation beats fail closed until fresh target acquisition. If that manual/operator step leaves the actor still `return_required`, the runtime refreshes the pending automatic return-step deadline from the manual step time and replaces any older pre-manual deadline; the old deadline must not fire immediately after the operator has already moved the actor. If the step completes or moves the actor back inside leash, the pending deadline is cleared. If the actor is already `at_home` or merely `within_radius`, the trigger returns a no-op `complete = true` step and does not persist, mutate runtime position, clear targets, release engagement, advance snapshot ownership, queue frames, or schedule a new automatic step. Dead actors waiting on respawn fail closed until the server-owned respawn boundary rebuilds them.

The first server-owned executor deliberately does not introduce a general mob scheduler or goroutine loop. When an owned runtime/operator update leaves a live spawn-backed actor in `return_required`, or when runtime startup restores a persisted live spawn-backed actor that already classifies `return_required`, the runtime arms one pending return-step deadline. Each `FlushServerFrames()` pass first flushes due respawns, then applies any due spawn return steps with the same fixed `max_step = 100` path used by the loopback trigger, then flushes session-local delayed retaliation. A due automatic step persists the stepped materialized position before mutating runtime, queues the same retained/removed/added visibility frames as the operator step, and then arms the next one-second return step only if the post-step actor snapshot is still `return_required`. If a due step moves the actor back inside the leash radius without landing exactly at authored home, the executor clears the pending deadline and does not schedule a later no-op flush. Actors that classify `within_radius` are not auto-stepped to exact home; exact-home correction remains the controlled `/return-home` trigger until later chase/return choreography is owned. Failed planning, dead actors, or missing actors fail closed and clear the pending return-step deadline rather than fabricating movement. If static snapshot persistence fails before runtime mutation, the executor emits no visibility frames, leaves both runtime and persisted actor positions unchanged, and re-arms another one-second retry while the actor still classifies `return_required`.

The paired read-only pending-return schedule surface is:
- `GET /local/spawn-group-return-steps`
- `GET /local/spawn-group-return-steps/{entity_id}`

Rows are local/operator snapshots, not gameplay packets. Each row exposes `entity_id`, `ready_at`, `remaining_ms`, the current return-required spawn-group `actor`, and the planned `step` that the fixed-`max_step = 100` executor would apply on the next due flush. Rows are sorted by `entity_id`; already-due but unflushed schedules remain visible with `remaining_ms = 0`; stale schedules whose actor is gone, dead, no longer spawn-backed, no longer `return_required`, or no longer safely plannable are omitted and return `404` for exact lookup.

A spawn-backed actor whose default leash classification is already `return_required` is now deliberately outside the owned stationary combat loop until a later return/chase slice moves or rebuilds it back into leash. Fresh `TARGET` selection for that actor fails closed with no self frame and the shared-world attempt seam reports `target_return_required` instead of the generic non-targetable reason; a stale already-selected `ATTACK` against an actor that became return-required also fails closed with the same explicit reason before HP mutation, engagement, immediate retaliation, delayed retaliation, damage-info, reward, or respawn side effects. Actors that still classify `at_home` or `within_radius` keep using the existing target -> attack -> death -> respawn contract.

The first live consumer of that preserved home was respawn, not chase AI. When a spawn-backed combatant respawns after its server-owned dead timer, the runtime restores the materialized actor position to the preserved authored home before rebuilding visibility. Viewers that only saw the old displaced runtime position receive the ordinary `CHARACTER_DEL`, viewers that only see the authored home receive the normal add/info/update burst, and viewers that can see both receive the usual delete-plus-readd refresh. The respawned actor reports `status = at_home`, full bootstrap HP, and no active target binding.

The next controlled consumer is the loopback-only return-home trigger:
- endpoint: `POST /local/spawn-groups/{entity_id}/return-home`
- scope: `gamed` local/operator tooling only
- request body: none
- success response: the same spawn-group leash snapshot shape, now reporting the actor at authored `home` / `current` with `status = at_home`

On success, the trigger returns one live spawn-backed actor's materialized position to its preserved authored home. That applies to any non-home live displacement, including actors still classified as `within_radius`; the trigger is not limited to `return_required` recovery. For displaced actors, the runtime persists the static-actor snapshot update before mutating runtime state; if that save fails, the trigger fails closed before position, engagement, selected-target ownership, or queued visibility frames change. It does not change HP, death timers, reward descriptors, or combat profile metadata. It releases any current practice-mob engagement, clears selected combat targets bound to that actor's visible `VID`, and advances the actor's runtime combat snapshot version even when the actor was already at home and no coordinate mutation is needed. That at-home snapshot reset is intentional: stale session-local pending delayed retaliation beats must fail closed until the player freshly reselects/reengages the mob. This matches the existing explicit reset boundaries for actor update/removal. When the return changes visible-world membership, it reuses the owned static-actor visibility transition path: old-position-only viewers receive `CHARACTER_DEL`, home-position viewers receive the normal add/info/update burst, and retained viewers receive the normal delete-plus-readd refresh. If the actor was still within the same visible set, retained viewers still receive the same delete-plus-readd refresh at the authored home followed by target clears for sessions that had that actor selected.

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

The read-only loopback endpoint remains `GET /local/spawn-groups/{entity_id}/leash?radius=<positive-int>`. It is operator/debug tooling only; it is not a gameplay packet and it does not mutate actor position, target ownership, HP, death state, respawn timers, or visible-world membership. Its result is still meaningful after a runtime actor-position update: `home` remains the authored spawn position while `current` reflects the materialized actor position at lookup time.

The mutating loopback return endpoints are deliberately separate:
- `POST /local/spawn-groups/{entity_id}/return-step?max_step=<positive-int>` applies one capped planned step only when the actor is currently `return_required`; invalid `max_step`, malformed entity IDs, missing/non-spawn actors, dead actors waiting on respawn, or actors whose static snapshot cannot be stepped safely fail closed.
- `POST /local/spawn-groups/{entity_id}/return-home` performs the controlled exact-home return described above and returns `404` for missing/non-spawn actors, dead actors waiting on respawn, or actors whose static snapshot cannot be returned safely.
- `GET /local/spawn-group-return-steps/{entity_id}` returns `400` for malformed entity IDs and `404` for absent, stale, non-spawn, dead, in-radius, or unsafe pending return-step schedules.

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
- chase packets or server-driven `MOVE` fanout for mobs
- path-aware return cadence beyond the first due return-step applied from the existing pending-frame flush loop
- client-visible return-home packet choreography beyond the existing static-actor delete/readd visibility path
- pathfinding, patrol routes, sectors, or navmesh logic
- aggro radius acquisition or target switching
- persistence of live mob position distinct from authored spawn position

Until a later slice wires this classifier/return-step planner into chase or path-aware live mob movement behavior, the existing content-loaded practice mobs remain stationary except for the current capped return-step recovery path and use the already-owned target -> attack -> death -> respawn lifecycle only while they classify `at_home` or `within_radius`. A materialized spawn-backed actor that already classifies `return_required` is kept visible/debuggable but is not accepted as a combat target again until an owned respawn, operator return-home, operator return-step, update, or server-owned return-step executor places it back inside leash; runtime attempt callers can now distinguish this specific gate as `target_return_required`. The `GET` leash endpoint is only a read-only inspection bridge over that classifier, while the `POST` return-step and return-home endpoints are controlled local triggers for QA and lifecycle recovery, not final mob AI. The exact return-home trigger can also be used on a live `within_radius` mob to restore exact authored placement and reset selected-target/engagement ownership without changing HP or reward metadata; the one-step trigger leaves already-`within_radius` mobs untouched and selected, and the server-owned return-step executor stops re-arming as soon as a step brings the actor back inside that radius.
