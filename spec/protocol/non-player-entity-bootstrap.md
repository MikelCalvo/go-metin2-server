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
- a deterministic file-backed static-actor snapshot schema that can represent the full bootstrap actor set on disk before boot/runtime wiring lands; the store trims authored string fields (`name`, `combat_profile`, `interaction_kind`, `interaction_ref`, and `spawn_group_ref`) before validation/persistence and rejects values that become invalid after trimming
- runtime-owned boot-time restore plus successful create/update/delete persistence for that full static-actor snapshot on `gamed`
- runtime-owned in-place edits of those static actors across non-player directories and map indexes while preserving entity identity
- optional interaction-ready metadata (`interaction_kind` / `interaction_ref`) persisted and exposed through those same bootstrap actor seams without claiming interaction behavior yet
- operator/runtime map-occupancy and static-actor snapshots that can now surface those actors through `internal/worldruntime/scopes.go`, including resolved combat profile rank metadata for combat-profile actors
- the first loopback-only operator seed/snapshot/update/remove surface used to create, inspect, edit, and delete those runtime actors on `gamed`, including optional `combat_profile` metadata for create/update paths that need to author combat-capable bootstrap actors without a content-bundle import
- the first client-visible enter-game bootstrap burst for static actors that already share the entering player's visible world under the current bootstrap topology/AOI policy
- the first live operator-seed visibility burst for newly created static actors that already share some connected player's visible world under those same bootstrap topology/AOI rules
- the first live operator-delete visibility teardown for removing static actors that are currently visible to connected players under those same bootstrap topology/AOI rules
- the first live operator-update refresh for in-place edits that keep the actor in the same visible world set for already-connected players under those same bootstrap topology/AOI rules
- the first live operator-update visibility rebuild for in-place edits that move the actor across map/AOI boundaries and therefore change which already-connected players can see it

This slice now claims only seven narrow client-visible behaviors for non-player actors:
- when a player enters `GAME`, already-visible bootstrap static actors can be appended to that enter-game result with deterministic `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE` frames; combat-profile actors copy their resolved profile `level` into `CHAR_ADDITIONAL_INFO.level`, while runtime/operator snapshots expose the resolved profile `rank` as descriptor metadata only
- when an operator seeds a new static actor while players are already online, sessions that already share visible world with that actor can immediately receive the same deterministic `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE` burst
- when an operator removes a static actor while players are already online, sessions that currently share visible world with that actor can immediately receive a deterministic `CHARACTER_DEL`
- when an operator updates a static actor while players are already online and the actor remains in the same visible world set for those sessions, those already-visible sessions can immediately receive a deterministic `CHARACTER_DEL` followed by the actor's refreshed `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE` burst
- when an operator updates a static actor while players are already online and that update changes which sessions share visible world with the actor, sessions leaving visibility can immediately receive `CHARACTER_DEL` while sessions entering visibility can immediately receive the actor's deterministic `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE` burst
- when that same player later crosses the configured AOI boundary through `MOVE` or `SYNC_POSITION`, the runtime can queue the corresponding self-facing static-actor add/delete rebuild frames
- when gameplay-triggered transfer rebootstrap moves that player into another visible-world scope, the runtime can append the corresponding self-facing static-actor delete/add rebuild frames after the relocated self burst

It still does **not** yet claim generic live non-player movement/update packets beyond those explicit operator-triggered refresh/rebuild cases, dynamic actor behavior, or gameplay systems behind those actors.

## Current owned non-player contract

The first non-player contract is intentionally narrow.
A bootstrap non-player actor is only required to own:
- reusable entity identity
- non-player entity kind
- effective map presence
- map coordinates
- one minimal class/template identifier suitable for future packet/content work
- optional display/name identifier for deterministic lookup, debugging, or tooling
- a non-zero class/template/race identifier that fits the current bootstrap `CHARACTER_ADD` wire projection (`uint16`); runtime registration/update, file-backed restore, and content-bundle import all fail closed instead of accepting unencodable static actors that later visibility paths would have to skip
- optional interaction-ready metadata (`interaction_kind` / `interaction_ref`) for later self-only interaction slices

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
- client-visible spawn/update/delete packet choreography beyond the first enter-game bootstrap burst, the first live operator-seed add burst, the first live operator-delete teardown for already-visible sessions, the first same-visible-set operator update refresh, the first operator-triggered map/AOI visibility rebuild for static-actor edits, and the first self-facing AOI add/delete rebuild for already-seeded static actors
- damage, targeting, or death state beyond the currently documented training-dummy/death/reward bootstrap slices; in-place static-actor updates preserve the actor's authored/bootstrap death reward unless a later explicit reward-editing slice changes it
  - runtime directories, map indexes, visibility diffs, and snapshot/read paths must clone reward drop-vnum slices on register, update, removal, and read/diff composition so callers cannot mutate authored/bootstrap reward descriptors by holding an old slice alias
  - if a partially torn-down static actor is still present in the non-player directory but missing from the map index, a later in-place update must rebuild map presence while preserving the authored/bootstrap death reward instead of requiring delete-and-recreate
  - if the reverse partial-teardown shape leaves only map-index presence behind, in-place update must repair the entity index and move the actor to the requested effective map instead of failing closed; this keeps operator/static-spawn edit paths tolerant of index cleanup order without treating an unknown actor ID as creatable
  - if a bad partial-teardown or repair path leaves duplicate map-bucket presence for the same static actor entity ID, later register, lookup, in-place update, or remove must prune all stale map buckets before inserting or returning the new effective-map presence; map occupancy must never keep ghost static actors for the same entity ID on older maps
  - if a stale visibility-VID index entry survives for the same static actor entity ID, later registration or in-place update must prune the stale VID before inserting the actor's current encodable visibility VID; removal must prune every visibility VID alias for the removed entity ID; visible-VID lookup must never keep an old alias for the same static actor after registration, update repair, or removal
  - if a stale visibility-VID index entry points at a missing static actor entity ID, a later `ByVID` lookup must prune that orphaned index entry before failing closed; visible-VID lookup must not keep returning through stale non-player directory aliases after partial teardown
  - if that orphaned visibility-VID entry would otherwise conflict with later registration or in-place update of the real actor for that VID, the non-player directory must prune the orphan and allow the current actor to claim the VID; only live conflicting actor ownership should fail closed
  - file-backed static actor snapshots must also fail closed when `race_num` is zero or does not fit the current bootstrap `CHARACTER_ADD` projection (`uint16`), so boot-time/operator restore cannot persist actors that the visible static-actor path would later have to skip as unencodable
  - explicit static-actor entity-ID restore paths must fail closed when the requested ID is already owned by a live player entity; player and non-player runtime identities share one registry-owned entity namespace and must not overlap even if a static snapshot or operator repair path supplies an explicit ID
  - player `VID` values and encodable static-actor visibility `VID` values must share the same live visibility namespace; registering or updating either side must fail closed when the other side already owns that visible ID so later target/interaction lookups cannot resolve one on top of the other
  - rejected player/static-actor registrations must not consume the shared entity-ID allocator; only successful registrations advance `NextEntityID()` so failed collision, duplicate, validation, or index-repair attempts do not create avoidable gaps before later snapshot persistence/restore work
  - that cross-kind player-registration/update guard must also consult static-actor map presence, so a partially torn-down static actor that has lost its non-player directory entry but still occupies a map cannot be shadowed by a player using the same client-visible `VID`
  - tolerant static-actor removal through surviving map-index presence must also prune any stale non-player-directory visibility-VID aliases for the removed entity ID; the non-player directory's own remove path also prunes aliases even when the primary actor entry is already gone, so removal must not leave an interaction/targeting lookup alias behind after a directory/map-index partial teardown
  - in-place static-actor updates that make a previously non-encodable actor encodable for visibility must apply the same cross-kind visible-ID collision guard before mutating the non-player directory or map index; that guard must also consult surviving player map presence after partial player-directory teardown, and the pre-update actor plus player entry/map presence must remain unchanged on rejection
- inter-channel ownership

## Success definition for the next slice

The next runtime checkpoint after this document should be able to say:
- `internal/worldruntime` can register a non-player actor with its own entity kind
- the actor can be looked up deterministically
- the actor participates in owned map presence/index bookkeeping
- runtime-owned directories and map indexes can now also update that static actor in place without delete-and-recreate when its name/class/position changes
- a deterministic file-backed static-actor snapshot store can now save/load the full bootstrap actor set by stable entity identity, and `gamed` now restores that snapshot at boot and rewrites it after successful static-actor create/update/delete mutations
- those bootstrap static actors can now also carry optional paired `interaction_kind` / `interaction_ref` metadata through runtime state, snapshots, and operator create/update surfaces without claiming the interaction behavior itself yet
- runtime/operator map-occupancy snapshots can now surface those static actors on their effective maps
- relocate-preview and transfer results can now also expose explicit added/removed visible static actors beside the before/after occupancy snapshots
- runtime/operator static-actor, visibility, map-occupancy, relocate-preview, and transfer snapshots can now expose resolved `combat_rank` for combat-profile actors without changing current packet output or combat math
- `gamed` can seed, inspect, update, and remove those bootstrap static actors through loopback-only operator paths
- entering players can now receive a first deterministic bootstrap burst for static actors that already share their visible world, reusing `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE`
- newly seeded static actors can now also enqueue that same deterministic visibility burst to already-visible online players without requiring those players to relog or move first
- operator-driven static-actor deletes can now also enqueue a deterministic `CHARACTER_DEL` to already-visible online players without requiring those players to relog or move first
- operator-driven same-visible-set static-actor updates can now also enqueue a deterministic `CHARACTER_DEL` plus refreshed actor bootstrap burst to already-visible online players without requiring those players to relog or move first
- operator-driven static-actor relocations across map/AOI visibility boundaries can now also enqueue `CHARACTER_DEL` to leaving sessions and the normal actor bootstrap burst to entering sessions without requiring those players to relog or move first
- moving/syncing players can now receive the first deterministic self-facing add/delete rebuild for static actors when configured AOI changes whether those actors are visible
- player-only runtime behavior remains unchanged while this scaffolding lands

At that point, the repository will own the first real non-player runtime seam without pretending that NPCs or mobs are implemented.