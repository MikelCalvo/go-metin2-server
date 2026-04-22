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
- the first loopback-only operator seed/snapshot/remove surface used to create, inspect, and delete those runtime actors on `gamed`

This slice does **not** yet claim that non-player actors are visible to clients, replicated on the wire, or driven by gameplay systems.

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
- client-visible spawn/update/delete packet choreography
- damage, targeting, or death state
- inter-channel ownership or persistence schema for non-player actors

## Success definition for the next slice

The next runtime checkpoint after this document should be able to say:
- `internal/worldruntime` can register a non-player actor with its own entity kind
- the actor can be looked up deterministically
- the actor participates in owned map presence/index bookkeeping
- `gamed` can seed, inspect, and remove those bootstrap static actors through loopback-only operator paths
- player-only runtime behavior remains unchanged while this scaffolding lands

At that point, the repository will own the first real non-player runtime seam without pretending that NPCs or mobs are implemented.