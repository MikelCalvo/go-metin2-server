# Non-Player Entity Bootstrap

This document freezes the first owned non-player runtime contract for `go-metin2-server`.

It sits on top of:
- `entity-runtime-bootstrap.md`
- `world-topology-bootstrap.md`
- `visibility-rebuild.md`

Those documents already freeze the current player-first world-runtime seams and the current topology/visibility rules.
What this document adds is the next narrower boundary:

**What is the smallest non-player entity model the runtime will own before combat, AI, spawn systems, or client-visible packet choreography exist?**

## Scope

This contract applies only to:
- the current single-process bootstrap runtime
- runtime-owned non-player identity and map presence
- static or operator-seeded actors that exist as world-runtime data
- deterministic lookup and map membership inside `internal/worldruntime`
- runtime-owned in-place edits of those static actors across non-player directories and map indexes while preserving entity identity
- operator/runtime map-occupancy and static-actor snapshots that can now surface those actors through `internal/worldruntime/scopes.go`
- the first loopback-only operator seed/snapshot/update/remove surface used to create, inspect, edit, and delete those runtime actors on `gamed`
- the first client-visible enter-game bootstrap burst for static actors that already share the entering player's visible world under the current bootstrap topology/AOI policy

This slice now claims only two narrow client-visible behaviors for non-player actors:
- when a player enters `GAME`, already-visible bootstrap static actors can be appended to that enter-game result with deterministic `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE` frames
- when that same player later crosses the configured AOI boundary through `MOVE` or `SYNC_POSITION`, the runtime can queue the corresponding self-facing static-actor add/delete rebuild frames

It still does **not** yet claim general spawn/update/delete replication to other sessions, dynamic actor behavior, or gameplay systems behind those actors.

## Current owned non-player contract

The first non-player contract is intentionally narrow.
A bootstrap non-player actor is only required to own:
- reusable entity identity
- non-player entity kind
- effective map presence
- map coordinates
- one minimal class/template identifier suitable for future packet/content work
- optional display/name identifier for deterministic lookup, debugging, or tooling

In other words, the runtime is about to own the fact that a non-player actor exists in the world, where it is, and what kind of actor it is.
It is **not** yet owning what that actor does.

## Minimum runtime fields

The first runtime-owned non-player actor should be representable with fields equivalent to:
- `entity ID`
- `entity kind`
- `position { map, x, y }`
- `class/template/race identifier`
- optional `name` or stable label

The exact Go type names can evolve, but the ownership boundary should stay this small.

## Why this slice exists

The project already owns:
- live player runtime
- entity identity for players
- player directory lookup
- map occupancy index
- session-directory transport hooks
- AOI policy seams

Without a non-player contract, the next step beyond player-only runtime would either:
- leak ad hoc actor structs directly into `internal/minimal`, or
- jump too early into NPC/mob/combat behavior before the runtime identity boundary is ready.

This document keeps the next step honest:
- own non-player identity first
- own non-player map presence second
- defer behavior until later slices

## Explicit non-goals

This slice does not yet freeze:
- NPC dialog or quest behavior
- mob AI, aggro, pathing, or combat
- spawn groups or respawn rules
- shops, drops, loot, or item generation
- client-visible spawn/update/delete packet choreography beyond the first enter-game bootstrap burst and the first self-facing AOI add/delete rebuild for already-seeded static actors
- damage, targeting, or death state
- inter-channel ownership or persistence schema for non-player actors

## Success definition for the next slice

The next runtime checkpoint after this document should be able to say:
- `internal/worldruntime` can register a non-player actor with its own entity kind
- the actor can be looked up deterministically
- the actor participates in owned map presence/index bookkeeping
- runtime-owned directories and map indexes can now also update that static actor in place without delete-and-recreate when its name/class/position changes
- runtime/operator map-occupancy snapshots can now surface those static actors on their effective maps
- `gamed` can seed, inspect, update, and remove those bootstrap static actors through loopback-only operator paths
- entering players can now receive a first deterministic bootstrap burst for static actors that already share their visible world, reusing `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE`
- moving/syncing players can now receive the first deterministic self-facing add/delete rebuild for static actors when configured AOI changes whether those actors are visible
- player-only runtime behavior remains unchanged while this scaffolding lands

At that point, the repository will own the first real non-player runtime seam without pretending that NPCs or mobs are implemented.