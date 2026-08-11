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

For spawn-backed static actors, the authored/home position is the actor's `spawn_group_ref` placement. In the current stationary practice-mob runtime, the current position is still the materialized actor position, so imported mobs normally classify as `at_home` until a later slice owns live mob movement.

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

Until a later slice wires this helper into live mob behavior, the existing content-loaded practice mobs remain stationary and use the already-owned target -> attack -> death -> respawn lifecycle.
