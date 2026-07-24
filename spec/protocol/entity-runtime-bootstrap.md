# Entity Runtime Bootstrap

This document freezes the first explicit entity/world-runtime ownership model that follows the initial shared-world pre-alpha slices.

It sits on top of:
- `world-topology-bootstrap.md`
- `visibility-rebuild.md`
- `transfer-rebootstrap-burst.md`

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
- static-actor visibility-`VID` lookup ownership for non-player actors

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

### Player directory

The runtime now owns a dedicated player lookup directory in:
- `internal/worldruntime/player_directory.go`

The current owned responsibilities are:
- index connected player entities by runtime entity ID
- index connected player entities by client-visible `VID`
- index connected player entities by exact character name
- keep deterministic player snapshot output for higher-level scope helpers
- prune stale secondary `VID` / name index entries when the primary entity entry is already gone, including lookup and removal paths, so partial teardown does not leave ghost lookup ownership that can block reconnect, replacement registration, or later cleanup
- prune stale secondary `VID` / name aliases that still point at a surviving player entity but no longer match that entity's current canonical `VID` / exact name; lookup fails closed for the stale alias while leaving the current canonical indexes intact, and register/update paths remove every old alias for the entity before writing the current keys
- reclaim stale non-canonical secondary `VID` / name aliases during register/update conflict checks before treating a key as live ownership; a secondary key that points at a surviving different player only blocks registration/update when that key still matches the indexed player's current canonical `VID` or exact name
- repair player-directory presence from surviving map-index presence during runtime lookups by entity ID, client-visible `VID`, or exact character name, so a partial teardown or repair that loses the directory entry does not strand whisper/scope/introspection lookups until the next update path
- reject player registration or update when a requested client-visible `VID` or exact character name is still owned by surviving map-index player presence for a different entity ID, even if the player-directory entry has already been lost during partial teardown

The stale-index pruning rule is deliberately narrow: a secondary lookup whose primary entity still exists remains authoritative only when the key matches that entity's current canonical identity. Orphaned secondary pointers and non-canonical aliases for either the same surviving entity or a different surviving entity are reclaimed before registration/update treats the key as available. Conflicting secondary keys that still point at a different live entity and match that entity's canonical identity continue to block ownership changes. Surviving map-index player presence follows the same fail-closed identity rule for `VID` and exact-name ownership until an explicit cleanup/reclaim path removes it.

### Static-actor visibility-VID directory

The runtime now owns non-player static-actor lookup by the temporary client-visible visibility `VID` in:
- `internal/worldruntime/non_player_directory.go`

The current owned responsibilities are:
- index static actors by their canonical encodable visibility `VID`, currently the runtime entity ID when it fits `uint32` and the actor has a non-zero `race_num` that fits the current `CHARACTER_ADD` projection
- reject live cross-actor conflicts on that visibility `VID`
- prune orphaned visibility-`VID` entries when their primary static-actor entry is already gone
- prune non-canonical visibility-`VID` aliases that point at a surviving actor whose current canonical visibility `VID` is different
- allow later register/update repair paths to reclaim orphaned or non-canonical aliases while preserving real live conflicts
- repair non-player-directory presence from surviving map-index presence during runtime lookups by entity ID, client-visible static-actor `VID`, or full static-actor snapshots, so partial teardown or repair that loses the directory entry does not hide visible actors from interaction/targeting/scope readers until the next update path
- reject unsupported static-actor `interaction_kind` values in the runtime directory itself, keeping runtime create/update validation aligned with the interaction-definition store and content-bundle boundaries

This keeps interaction/targeting and visibility bootstrap lookups from resolving through ghost non-player aliases after partial teardown or in-place repair.

### Map-occupancy index

The runtime now owns a dedicated effective-map membership boundary in:
- `internal/worldruntime/map_index.go`

The current owned responsibilities are:
- track player entity membership by effective `MapIndex`
- track static-actor membership by effective `MapIndex`
- normalize bootstrap `MapIndex = 0` through topology-aware effective-map semantics
- expose deterministic per-map character and static-actor snapshots for runtime callers
- keep register, move, update, and remove bookkeeping explicit instead of rebuilding occupancy from whole-world scans by default
- tolerate partial teardown when either the player/static entity index or map bucket has already been cleared first, so cleanup can still remove the remaining index state
- prune stale same-kind map-bucket ownership for the same entity ID during player/static registration before inserting the new effective-map presence, so reconnect/reclaim repair paths cannot leave ghost occupancy on older maps
- prune duplicate player map-bucket ownership for the same entity ID during player movement/update before inserting the new effective-map presence, so a surviving stale bucket cannot leave one player occupying two maps after a repair update
- repair duplicate player map-bucket ownership during player lookup when the primary entity index survives, mirroring static-actor lookup repair so runtime readers do not preserve ghost occupancy on older maps
- reject cross-kind player/static collisions even when the opposite-kind primary entity index is missing but its map-bucket ownership survives, including player movement/update repair paths that would otherwise overwrite a surviving static-actor bucket with the same entity ID
- reject cross-kind player/static collisions when the opposite-kind primary entity index survives but the map bucket is missing, so partial map-index teardown cannot let player registration/movement or static-actor registration/update claim the same entity ID
- rebuild player map-index presence from the player directory during player updates when the map index was partially cleared first, mirroring static-actor update repair and keeping movement/transfer refresh paths tolerant of map-index-only loss
- rebuild player-directory presence from surviving player map-index presence during player updates when the directory entry was partially cleared first, so movement/transfer refresh paths can repair the exact-name and `VID` lookups instead of requiring disconnect/reconnect cleanup
- repair duplicate player map-bucket ownership when lookup by `VID` discovers a stale bucket for an older player `VID` while the primary entity index already points at the player's newer canonical `VID`; the stale lookup fails closed, prunes the old map bucket, and preserves the current canonical `VID` lookup
- expose static-actor lookup by client-visible `VID` directly from surviving map-index presence, and expose all static actors from map-index state, so entity-registry repair and scope visibility readers can recover non-player directory presence after partial teardown
- repair duplicate same-kind player/static map-bucket ownership from surviving primary map-index entries before returning map-occupancy snapshots, so `/local/maps` and scope readers do not surface one actor on stale older maps after a partial repair
- repair that same duplicate same-kind map-bucket ownership before direct per-map player/static readers return, so lower-level runtime callers of `MapCharacters(...)` and `StaticActors(...)` share the same ghost-pruning behavior as full occupancy snapshots

This keeps map occupancy as an owned runtime primitive, and the current connected-player / visibility / map-occupancy / static-actor introspection snapshots can now be composed through `internal/worldruntime/scopes.go` instead of bootstrap-local shared-world conversion code.
The tolerant cleanup rules are deliberately narrow: they make reconnect/close/register repair idempotent across owned runtime indexes without treating stale map/index remnants as a live session. Cross-kind primary-index and bucket remnants are still authoritative enough to block conflicting registration because player and static-actor entity IDs share one runtime identity space.

### Session directory

The runtime now owns a dedicated transport-hook directory in:
- `internal/worldruntime/session_directory.go`

The current owned responsibilities are:
- register / replace / remove queued frame sinks and relocate callbacks by entity ID
- let `internal/minimal/shared_world.go` route join/leave/transfer fanout and exact-session relocate lookups through a `worldruntime`-owned directory boundary
- keep transport cleanup explicit on leave/close/reconnect without a bootstrap-local hook map
- tolerate partial teardown by letting leave/close cleanup remove stale transport hooks even when another runtime index already lost the entity first

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