# Spawn Leash Bootstrap

This document freezes the first tiny runtime seam for future mob chase/leash/return work in `go-metin2-server`.

It sits on top of:
- `non-player-entity-bootstrap.md`
- `content-spawn-groups-bootstrap.md`
- `world-topology-bootstrap.md`

Those documents already freeze the current player/non-player identity, map placement, authored `spawn_groups`, target/attack/death/respawn loop, and topology visibility rules.

What this document adds is deliberately narrower:

**How does the runtime classify a spawn-backed actor's current position against its authored spawn position before any real movement/pathing AI exists?**

## Current owned contract

The first bootstrap leash seam is a pure runtime classification helper in `internal/worldruntime`, exposed through the bootstrap runtime for read-only operator inspection.

Inputs:
- authored/home `Position { map_index, x, y }`
- current `Position { map_index, x, y }`
- positive leash `radius`

For spawn-backed static actors, the authored/home position is the actor's `spawn_group_ref` placement. The runtime preserves that authored home separately from the materialized actor's current `Position`, including generic runtime/operator position edits that keep the same `spawn_group_ref`. Older snapshots that lack a preserved home position fall back to the current position and classify as `at_home` until moved by a later owned seam.

In the current stationary practice-mob runtime, freshly imported mobs normally classify as `at_home`. If an owned runtime/operator update changes only the materialized actor position, the same read-only leash inspection must continue to compare that current position against the preserved authored home and can report `within_radius` or `return_required` without mutating the actor.

A spawn-backed actor whose default leash classification is already `return_required` is now deliberately outside the owned stationary combat loop until a later return/chase slice moves or rebuilds it back into leash. Fresh `TARGET` selection for that actor fails closed with no self frame and the shared-world attempt seam reports `target_return_required` instead of the generic non-targetable reason; a stale already-selected `ATTACK` against an actor that became return-required also fails closed with the same explicit reason before HP mutation, engagement, immediate retaliation, delayed retaliation, damage-info, reward, or respawn side effects. Actors that still classify `at_home` or `within_radius` keep using the existing target -> attack -> death -> respawn contract.

The first live consumer of that preserved home is now respawn, not chase AI. When a spawn-backed combatant respawns after its server-owned dead timer, the runtime restores the materialized actor position to the preserved authored home before rebuilding visibility. Viewers that only saw the old displaced runtime position receive the ordinary `CHARACTER_DEL`, viewers that only see the authored home receive the normal add/info/update burst, and viewers that can see both receive the usual delete-plus-readd refresh. The respawned actor reports `status = at_home`, full bootstrap HP, and no active target binding; the leash endpoint remains read-only and does not itself trigger that return.

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

For this slice, the concrete loopback endpoint is `GET /local/spawn-groups/{entity_id}/leash?radius=<positive-int>`. It is operator/debug tooling only; it is not a gameplay packet and it does not mutate actor position, target ownership, HP, death state, respawn timers, or visible-world membership. Its result is still meaningful after a runtime actor-position update: `home` remains the authored spawn position while `current` reflects the materialized actor position at lookup time.

## Fail-closed cases

The helper refuses to classify:
- invalid zero-map home/current positions
- non-positive leash radii
- non-spawn actors without `spawn_group_ref`
- spawn actors whose `spawn_group_ref` or combat profile is invalid under the current runtime validators

## Why this seam exists now

The current practice-mob loop is still stationary, but the world lane needs a safe boundary before adding chase, leash, return, patrol, or target-switching behavior.

Freezing the classification first lets later slices add movement or server-origin AI steps without duplicating ad hoc distance/map checks in `internal/minimal`.

## Explicit non-goals

This slice does **not** yet implement:
- autonomous mob movement
- chase packets or server-driven `MOVE` fanout for mobs
- return-home packet choreography
- pathfinding, patrol routes, sectors, or navmesh logic
- aggro radius acquisition or target switching
- persistence of live mob position distinct from authored spawn position

Until a later slice wires this classifier into live mob movement behavior, the existing content-loaded practice mobs remain stationary and use the already-owned target -> attack -> death -> respawn lifecycle only while they classify `at_home` or `within_radius`. A materialized spawn-backed actor that already classifies `return_required` is kept visible/debuggable but is not accepted as a combat target again until an owned respawn, update, or later return/chase slice places it back inside leash; runtime attempt callers can now distinguish this specific gate as `target_return_required`. The loopback endpoint is only a read-only inspection bridge over that classifier so QA can verify authored home/current/radius semantics before chase/return work begins.
