# Entity Runtime Bootstrap

This document freezes the first explicit entity/world-runtime ownership model that follows the initial shared-world pre-alpha slices.

It sits on top of:
- `world-topology-bootstrap.md`
- `visibility-rebuild.md`
- `map-transfer-bootstrap.md`

Those documents already freeze bootstrap topology, visibility policy boundaries, and the current transfer contract.
What this document adds is the next owned architecture boundary underneath those flows.

## Scope

This slice documents the runtime concepts that the repository now owns or is intentionally extracting next for M2:
- live player runtime
- generic player-first entity identity
- player directory lookup ownership
- map-occupancy index ownership
- session-directory ownership for transport hooks
- visibility/AOI policy ownership

It does **not** claim that all of those pieces are fully implemented yet.
The point of this slice is to stop treating those boundaries as implicit future refactors.

## Current owned runtime concepts

### Live player runtime

The selected in-world player is no longer treated as the same conceptual object as the persisted bootstrap character snapshot.

The current live runtime boundary is:
- `internal/player/runtime.go`

The current owned responsibilities are:
- hold the persisted bootstrap `loginticket.Character` snapshot
- hold live selected-session world position separately from that persisted snapshot
- keep the selected-session link (`Login`, `CharacterIndex`) explicit
- expose a live character view for gameplay/session flows
- allow explicit re-alignment with a newly persisted snapshot after save/update flows

This means runtime mutation and persistence mutation are no longer the same thing by accident.

### Generic entity identity

The first reusable world actor identity boundary is now owned by:
- `internal/worldruntime/entity.go`
- `internal/worldruntime/entity_registry.go`

The current bootstrap implementation is intentionally narrow:
- player entities only
- in-memory registration only
- one process-local runtime only

The owned abstraction boundary is:
- runtime callers should refer to reusable entity identity instead of raw session-local bookkeeping where possible
- future non-player actors must fit this identity model instead of forcing another rewrite of visibility ownership later

### Map-occupancy index

The runtime now owns a dedicated effective-map membership boundary in:
- `internal/worldruntime/map_index.go`

The current owned responsibilities are:
- track player entity membership by effective `MapIndex`
- normalize bootstrap `MapIndex = 0` through topology-aware effective-map semantics
- expose deterministic per-map character snapshots for runtime callers
- keep register, move, and remove bookkeeping explicit instead of rebuilding occupancy from whole-world scans by default

This keeps map occupancy as an owned runtime primitive even though some callers still need to be rewired to consume it directly.

### Session directory

The runtime now owns a dedicated transport-hook directory in:
- `internal/worldruntime/session_directory.go`

The current owned responsibilities are:
- register / replace / remove queued frame sinks and relocate callbacks by entity ID
- let `internal/minimal/shared_world.go` route join/leave/transfer fanout and exact-session relocate lookups through a `worldruntime`-owned directory boundary
- keep transport cleanup explicit on leave/close/reconnect without a bootstrap-local hook map

This means the shared-world runtime no longer needs a separate session-hook table just to find queued frame sinks or relocate callbacks.

### Visibility and AOI policy

Visible-world decisions are now owned by:
- `internal/worldruntime/topology.go`
- `internal/worldruntime/visibility.go`

The current owned policy boundary is:
- topology defines effective local channel and effective map identity
- visibility policy defines whether two live actors can see each other
- `WholeMapVisibilityPolicy` is the current default bootstrap implementation

This means AOI exists as an architecture seam even though the default behavior is still whole-map visibility.

## Remaining extraction boundaries now explicitly owned by the roadmap

The repository still treats these as the next project-owned runtime boundaries after the player directory, map-index, and session-directory extractions:

## Current composition model

The current bootstrap composition is intentionally transitional:

- `internal/minimal/factory.go` still owns session-flow wiring
- `internal/minimal/shared_world.go` still orchestrates the current shared-world bootstrap runtime
- `internal/player` now owns selected live player state
- `internal/worldruntime` now owns topology, visibility, entity identity, effective-map membership, and transport-hook directory seams

This is an explicit intermediate state.
The project is not claiming that `internal/minimal/shared_world.go` is already the final world runtime.

## Why this slice exists

Tasks 1-10 already changed the architecture materially:
- topology is no longer scattered helper logic
- visibility diffs are explicit
- transfer is routed through `internal/warp`
- live player state is separate from persisted snapshots
- a first entity registry exists

Without this document, the next M2 work would still be guided mostly by commit history and chat context.

This slice makes the next owned runtime frontier explicit:

1. own directories and indexes inside `internal/worldruntime`
2. close the remaining self-session transfer rebootstrap gap
3. add a real AOI policy beyond the whole-map default
4. only then open inventory/equipment state on top of the live player runtime

## Explicit non-goals

This slice does not yet add or freeze:
- NPC, mob, spawn, or item-ground entity runtime
- combat state, damage, death, or respawn
- inventory, equipment, or item-use packet/state contracts
- DB-backed persistence or migrations
- real shard/channel routing or remote world ownership handoff
- final public admin/auth surfaces beyond the current local-only bootstrap tooling

## Success definition for the next M2 window

The next world-runtime checkpoint should look like this:
- player lookup is not a whole-world scan
- map occupancy is an owned runtime primitive
- session transport hooks are routed through an owned directory instead of a bootstrap-local hook map
- transfer can reuse an owned self-session rebootstrap burst
- AOI policy can evolve without rewriting every caller

At that point, M2 stops being a vague goal and becomes a reusable runtime foundation for M3 character-state work.