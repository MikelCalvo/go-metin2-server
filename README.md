# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

## Status

This repository is the public bootstrap for a fresh, protocol-first rewrite.

Current scope of the project:
- Go 1.26 baseline.
- Clean-room process: no legacy server/client code is vendored into this repository.
- Separate `authd` and `gamed` binaries from day zero.
- A dedicated pprof/ops HTTP server for profiling goroutines, heap, allocs, mutex contention and blocking.
- Minimal legacy TCP listeners wired into both `authd` and `gamed`.
- A real secure legacy handshake compatible with the current local client reference: X25519+BLAKE2b session keys, HMAC-SHA512/256 challenge verification, XChaCha20-Poly1305 session token delivery, and XChaCha20-encrypted post-handshake traffic.
- A stub-compatible binary smoke path that reaches `GAME` with the current public bootstrap flows.
- A deterministic single-character `MOVE` round-trip wired through the current bootstrap runtime.
- A deterministic selected-character `SYNC_POSITION` reconciliation path wired through the current bootstrap runtime.
- A tolerant `CLIENT_VERSION` metadata path accepted during `LOADING` before `ENTERGAME`.
- A tolerant `PONG` control path accepted in `GAME` for server-driven ping probes.
- An explicit bootstrap world-topology model that centralizes local channel, map, and chat scope decisions for the current single-process runtime.
- Character deletion on the selection surface with deterministic success/failure responses.
- A first exact loading-to-game bootstrap burst after `ENTERGAME`: `PHASE(GAME)`, selected-character `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`, `PLAYER_POINT_CHANGE`, then any trailing peer visibility frames.
- Minimal shared-world peer visibility for players that are already connected to the bootstrap runtime and share the same bootstrap `MapIndex`.
- Minimal MOVE fanout so visible peers on the same bootstrap `MapIndex` receive queued movement replication from other connected players, with AOI-aware visibility rebuild on `MOVE` when an opt-in runtime radius policy causes peers to newly enter or leave visible range.
- Minimal `SYNC_POSITION` fanout so visible peers on the same bootstrap `MapIndex` receive queued reconciliation updates from other connected players, with AOI-aware visibility rebuild on `SYNC_POSITION` when an opt-in runtime radius policy causes peers to newly enter or leave visible range.
- A dedicated `internal/worldruntime` visibility helper and first whole-map visibility-policy boundary for same-topology visible-peer computation, deterministic add/remove peer-set diffs, and explicit self-facing visibility transitions for enter/leave/relocate callers.
- A first opt-in bootstrap AOI helper in `internal/worldruntime`: deterministic floor-stable sector keys and a radius-based visibility policy now exist beside the default whole-map policy, and `gamed` can now select whole-map vs radius AOI through service config/env wiring without changing the default wire behavior when AOI is left unset.
- A first non-player runtime scaffolding in `internal/worldruntime`: static actors can now be registered with owned entity identity and map presence, can now also be updated in-place across runtime directories/map buckets, `gamed` now exposes the first local-only operator seed/snapshot/update/remove path for those runtime actors, entering players now receive the first client-visible bootstrap burst for already-visible static actors, newly seeded static actors now also fan out that same bootstrap burst to already-visible online players, operator-driven static-actor deletes now also immediately enqueue `CHARACTER_DEL` to already-visible online players, operator-driven in-place static-actor updates now refresh already-visible online players with delete-plus-rebootstrap when the actor stays in the same visible set and now also emit leave/add visibility deltas when that update crosses map/AOI boundaries, gameplay-triggered transfer rebootstrap now also appends the moved player's static-actor delete/add visibility deltas, `MOVE` / `SYNC_POSITION` now rebuild self-facing static-actor visibility with add/delete frames when configured AOI makes those actors enter or leave visible range, and a deterministic file-backed static-actor snapshot store is now wired into `gamed` boot plus successful operator create/update/delete mutations.
- A first owned client-originated bootstrap static-actor interaction ingress: `GAME` sessions can now send `INTERACT (0x0501)` targeting a visible static actor by `vid`, with deterministic codec coverage and dedicated `internal/game` dispatch hooks ready for the next `info` / `talk` runtime slices without yet claiming the response behavior.
- An internal server-side map-relocation visibility rebuild primitive that removes peers from the old bootstrap `MapIndex` and bootstraps peers on the destination `MapIndex`.
- A loopback-only `gamed` relocation ops trigger that exercises bootstrap `MapIndex` relocation by exact character name without freezing a final client warp contract.
- A loopback-only `gamed` relocation dry-run endpoint that previews visibility and map-occupancy effects before applying a bootstrap `MapIndex` relocation, now including full before/after map-occupancy snapshots and explicit static-actor visibility diffs alongside the delta counts, and composing that structured preview through `internal/worldruntime/scopes.go`.
- A loopback-only `gamed` structured transfer endpoint that commits the minimal bootstrap map-transfer contract and returns the applied transfer result, including full before/after map-occupancy snapshots and explicit static-actor visibility diffs alongside the delta counts, with the structured result now also composed through `internal/worldruntime/scopes.go`.
- A first owned self-session transfer rebootstrap burst: transfer-triggered `MOVE` / `SYNC_POSITION` suppress immediate self ack packets, rebuild the moved player on the same game socket with the relocated self bootstrap burst, and then append trailing peer visibility deltas.
- Persist-before-commit bootstrap transfer orchestration via `internal/warp`, with best-effort rollback to the previous persisted account snapshot if the late runtime commit step fails.
- A first `internal/player` runtime model that keeps selected-session live world position separate from the persisted bootstrap character snapshot.
- A first generic `internal/worldruntime` entity registry that registers player entities through a reusable identity boundary instead of direct shared-world character bookkeeping.
- A topology-aware `internal/worldruntime` map index that tracks effective-map player membership as an owned runtime primitive instead of recomputing occupancy from ad hoc session scans.
- A transport-only `internal/worldruntime` session directory boundary for queued frame sinks and relocate callbacks, now used as the shared-world transport-hook lookup path for join/leave/transfer and fanout delivery instead of bootstrap-local hook maps.
- A first documented bootstrap reconnect/teardown runtime contract: close/disconnect tears down session-directory hooks, entity ownership, and map occupancy idempotently, reconnect rebuilds fresh live runtime state from persisted snapshots instead of stale in-memory ownership, duplicate fresh `ENTERGAME` for the same still-live selected character is rejected instead of creating a ghost `GAME` session with no shared-world registration, that rejected duplicate session now stays retryable on the same encrypted game socket while it remains in `LOADING`, transfer-then-close retries now also refresh the waiting selected runtime from the persisted destination snapshot before re-entering `GAME`, close/leave now still emits the final peer-facing delete when the entity registry entry was already lost but the shared-world runtime still has the last known player snapshot, a surviving session-directory hook now still blocks duplicate `ENTERGAME` even if the entity index entry was already lost, stale reclaimed sockets no longer keep fanning out gameplay replication to live peers after they lose shared-world ownership, later stale `MOVE` / `SYNC_POSITION` traffic now stays self-local-only instead of overwriting the persisted character snapshot after reclaim, and stale `WHISPER` traffic now also stays self-local/non-delivering instead of fabricating a missing-target reply after reclaim, a fresh `ENTERGAME` can reclaim stale runtime ownership if the old session hook is already missing, and visible peers now see the stale actor deleted before the reclaimed session replays its normal entry burst.
- A loopback-only `gamed` runtime config endpoint that reports the active bootstrap local-channel and visibility-policy selection (`whole_map` vs configured radius AOI) for the current daemon process.
- A loopback-only `gamed` runtime snapshot endpoint that lists currently connected bootstrap characters and their effective map/position state, now routed through `internal/worldruntime` scope queries instead of open-coded shared-world scans.
- A loopback-only `gamed` runtime visibility endpoint that shows which currently connected bootstrap characters can see each other under the shared-world bootstrap rules, now also built from `internal/worldruntime` visibility scope snapshots.
- A loopback-only `gamed` runtime map-occupancy endpoint and static-actor snapshot surface now consume owned `internal/worldruntime` scope snapshots over the map index and non-player directories instead of bootstrap-local snapshot conversion code, including bootstrap static actors alongside connected players and on static-only maps.
- Minimal local talking chat fanout so same-empire visible peers on the same bootstrap `MapIndex` receive queued `GC_CHAT` deliveries from other connected players.
- Minimal whisper routing by exact character name across currently connected bootstrap sessions.
- Topology-aware social scope queries in `internal/worldruntime` now own local talking, bootstrap-global party, shout, guild, and exact-name whisper target selection instead of scattering those routing conditions across bootstrap shared-world fanout code.
- Minimal bootstrap `CHAT_TYPE_PARTY` fanout across the currently connected `GAME` sessions.
- Minimal bootstrap `CHAT_TYPE_GUILD` fanout across connected `GAME` sessions that share the same non-zero `GuildID`.
- Minimal bootstrap `CHAT_TYPE_SHOUT` fanout across connected `GAME` sessions in the same empire.
- Minimal bootstrap system `CHAT_TYPE_INFO` self-delivery in `GAME`, plus a server-originated `CHAT_TYPE_NOTICE` broadcast path exposed through a local-only `gamed` ops endpoint; client-originated `CHAT_TYPE_NOTICE` remains rejected.
- A first owned in-game slash-command bootstrap path: `/quit` now returns a self-facing `CHAT_TYPE_COMMAND quit`, `/logout` now leaves the bootstrap shared world and emits `PHASE(CLOSE)`, and `/phase_select` now leaves the bootstrap shared world and emits `PHASE(SELECT)` so the same session can choose another character again instead of echoing those strings as normal talking chat.
- A first self-only `CHARACTER_UPDATE` refresh emitted immediately after the visible-world insert.
- A first self-only `PLAYER_POINT_CHANGE` refresh emitted immediately after the selected-character update.
- Multi-stage Docker build with a lightweight runtime image that keeps Go debug information intact by avoiding stripped builds.

## Near-term goal

The first real milestone is not “full gameplay”.
It is a minimal but complete TMP4-compatible boot path:
- handshake,
- login/auth,
- character list,
- create character,
- select character,
- enter game,
- spawn in a minimal world,
- basic movement.

## Roadmap

Legend:
- `[x]` done
- `[~]` in progress
- `[ ]` not started

### 1. Project foundation
- [x] Go 1.26 baseline
- [x] clean-room policy and repository bootstrap
- [x] separate `authd` and `gamed` entrypoints
- [x] profiling/ops endpoints
- [x] CI and container baseline

### 2. Protocol foundation
- [x] frame envelope contract
- [x] session phase model
- [x] initial boot-path packet matrix
- [x] protocol docs owned by this repository

### 3. Control-plane handshake
- [x] control packet primitives (`PHASE`, `PING`, `PONG`)
- [x] key exchange packet layouts (`KEY_CHALLENGE`, `KEY_RESPONSE`, `KEY_COMPLETE`)
- [x] server-side handshake flow
- [x] socket-level server-side handshake validation

### 4. Authentication and selection surface
- [x] login request handling
- [x] login success/failure path
- [x] character list surface
- [x] minimal `authd` and `gamed` runtime listeners
- [x] empire selection support for empty-account bootstrap flow
- [x] character creation, deletion and selection

### 5. World entry
- [x] loading/bootstrap packets
- [x] main character bootstrap
- [x] player points/stat bootstrap
- [x] enter-game flow

### 6. First in-world behavior
- [~] minimal world state
- [x] basic movement handling
- [x] movement replication/ack path

### 7. Hardening and expansion
- [~] persistence layer that matches the compatibility target
- [ ] observability and operator tooling beyond pprof/health
- [ ] additional gameplay systems after the boot path is stable
- [ ] production packaging and deployment guidance

## Global system status

Legend:
- `[x]` implemented and exercised in the repository
- `[~]` bootstrap, partial, or intentionally simplified
- `[ ]` not started yet

### Protocol and boot path

| Area | Status | Notes |
| --- | --- | --- |
| Frame envelope and packet codecs | [x] | Core control/login/world/move/chat packet families are owned by this repository. |
| Session phases and handshake | [x] | `HANDSHAKE -> AUTH/LOGIN -> SELECT -> LOADING -> GAME` is documented and tested. |
| Auth/login surface | [x] | `authd` and `gamed` split, login-key path working. |
| Character list / selection surface | [x] | login success, empire selection, create/delete/select all exist in bootstrap form. |
| Enter-game bootstrap | [x] | main character, visible insert, point refresh, and phase transition are in place. |
| Real capture/golden fixture library | [~] | Protocol docs and tests exist, but a richer project-owned fixture corpus is still ahead. |

### World runtime

| Area | Status | Notes |
| --- | --- | --- |
| Shared player visibility | [~] | Peer enter/leave visibility exists for players on the same bootstrap `MapIndex`, now computed through a dedicated `internal/worldruntime` helper that exposes explicit visibility diffs and the first whole-map policy boundary for runtime callers. |
| Movement replication | [~] | `MOVE` and `SYNC_POSITION` fan out to already-visible peers on the same bootstrap `MapIndex`; when opt-in radius AOI is configured, both now also rebuild peer visibility with add/delete transitions if the update crosses the visible-range boundary. |
| Map boundaries | [~] | An explicit bootstrap topology now owns local channel/map chat scoping, but there is still no final warp flow yet. |
| Channel topology | [ ] | No real multi-channel topology, shard routing, or inter-channel ownership yet. |
| Interest management / culling | [~] | The first AOI boundary now exists as a whole-map visibility policy, an opt-in bootstrap radius/sector helper now exists beside it, topology now carries explicit helpers for selecting whole-map vs radius visibility policy, and `gamed` can now wire that choice from service config/env into the live bootstrap runtime, but there is still no production range-, sector-, or distance-based culling policy wired through the runtime by default. |
| Warp / map transfer | [~] | A server-side visibility-rebuild primitive, structured transfer commit path, and first self-session transfer rebootstrap burst exist, but there is still no final end-to-end client/server warp/loading flow yet. |
| Entity runtime beyond players | [~] | A first generic entity registry, dedicated player directory, topology-aware map index, and transport-only session directory now exist in `internal/worldruntime`; shared-world transport routing now consumes that session directory, reconnect/teardown and the first non-player runtime contract are now frozen in docs, duplicate-live `ENTERGAME` rejection can now be retried on the same encrypted game socket once the live owner disappears, the first static non-player actor scaffolding now owns entity identity plus map presence, `gamed` now exposes loopback-only operator seed/snapshot/remove paths for those static actors, a deterministic file-backed static-actor snapshot store now exists for the full bootstrap actor set and is now wired into boot plus successful operator create/update/delete mutations, static-actor visibility introspection now also reports per-player `visible_static_actors`, interaction-ready `interaction_kind` / `interaction_ref` metadata can now be stored and persisted on those bootstrap actors, and runtime map-occupancy snapshots now surface static actors on their effective maps, but broader reconnect hardening, richer AOI, and real non-player gameplay behavior are still ahead. |

### Social, chat, and operator surfaces

| Area | Status | Notes |
| --- | --- | --- |
| Local talking chat | [~] | Same-map + same-empire fanout exists; still bootstrap scope. |
| Whisper | [x] | Exact-name whisper routing works across connected bootstrap sessions through the `internal/worldruntime` player directory. |
| Party chat | [~] | Bootstrap-only fanout; no real party membership system yet. |
| Guild chat | [~] | Scoped by non-zero `GuildID`, but no guild lifecycle/roster system exists yet. |
| Shout | [~] | Same-empire bootstrap fanout exists; no real world/channel topology behind it yet. |
| System info / notice | [~] | `INFO` self-delivery, server-originated `NOTICE`, and local-only notice trigger exist; bootstrap-global notice target selection now routes through `internal/worldruntime` instead of ad hoc shared-world scans. |
| Operator/admin surface | [~] | Loopback-only `POST /local/notice`, `POST /local/relocate`, `POST /local/relocate-preview`, `POST /local/transfer`, `GET /local/runtime-config`, `GET /local/players`, `GET /local/visibility`, `GET /local/maps`, plus `GET`/`POST /local/static-actors`, `PATCH`/`PUT /local/static-actors/{entity_id}`, and `DELETE /local/static-actors/{entity_id}` for bootstrap non-player runtime seeding/introspection/edit/removal, exist on `gamed`; `/local/runtime-config` now reports the active local channel and visibility-policy selection, `/local/visibility` now also reports `visible_static_actors` per connected player under the active topology/AOI policy, `/local/static-actors` create/update payloads can now also carry optional paired `interaction_kind` / `interaction_ref` metadata, `/local/maps` now reports static actors alongside connected players in each effective-map snapshot, newly seeded static actors now immediately enqueue their visibility burst to already-visible online players, operator-driven static-actor deletes now immediately enqueue `CHARACTER_DEL` to those same already-visible sessions, PATCH/PUT updates now refresh retained viewers with delete-plus-rebootstrap and also emit delete/add visibility deltas when the actor crosses map/AOI boundaries, and the structured relocate-preview/transfer responses now include full before/after map-occupancy snapshots plus explicit static-actor visibility diffs beside the delta counts; broader admin/auth tooling does not. |

### Character systems and gameplay

| Area | Status | Notes |
| --- | --- | --- |
| Character snapshots / bootstrap stats | [~] | Enough for selection, spawn, movement, chat, and bootstrap transfer-trigger slices; a first live `internal/player` runtime model now exists separately from persisted snapshots, the default bootstrap `mkmk` account now seeds `MkmkWar` at the legacy Shinsoo Yongan start while auto-migrating the untouched old fake `(1000,2000)` snapshot on load, and fresh character creation now uses legacy empire-specific create positions instead of fake slot-relative coordinates. |
| Gameplay transfer triggers | [~] | First exact-position trigger can commit bootstrap map transfer from `MOVE` / `SYNC_POSITION`; persist-before-commit ordering and the current self-session rebootstrap burst are documented, but the final warp/loading packet is still not frozen. |
| Inventory | [ ] | Not started. |
| Equipment | [ ] | Not started. |
| Item use / consumables | [ ] | Not started. |
| Derived stats / combat formulas | [ ] | Not started. |
| Combat loop | [ ] | No targeting, attacks, damage, or death loop yet. |
| Respawn / death handling | [ ] | Not started. |
| NPCs / shops | [ ] | Not started. |
| Mobs / AI / spawn groups | [ ] | Not started. |
| Quest / script runtime | [ ] | Not started. |

### Persistence, packaging, and public-project maturity

| Area | Status | Notes |
| --- | --- | --- |
| Login tickets | [x] | Working file-backed ticket flow between `authd` and `gamed`. |
| Bootstrap account snapshots | [~] | File-backed account/character persistence exists, but it is not compatibility-grade yet. |
| Bootstrap static-actor snapshots | [x] | A deterministic file-backed snapshot store now exists under `internal/staticstore`, and `gamed` now loads it at boot and rewrites it after successful static-actor create/update/delete mutations. |
| Database schema / migrations | [ ] | No real DB-backed persistence layer or live migrations yet. |
| Observability | [~] | Health, pprof, and small local-only notice/relocation/runtime-introspection/map-occupancy/dry-run/transfer/static-actor seed-remove endpoints exist; metrics/logging/admin depth still needs work. |
| CI / public validation | [x] | GitHub Actions baseline checks formatting, tests, vet, daemon builds, and runtime/debug image builds. |
| Release/deploy guidance | [ ] | No production-grade release/deployment story yet. |

## Milestone ladder

| Milestone | Status | Exit criteria |
| --- | --- | --- |
| M0 — Protocol-owned boot path | [x] | Handshake, auth/login, selection, create/delete/select, enter-game, and first movement loop are stable. |
| M1 — Shared-world pre-alpha | [~] | Players can see each other, move, chat, and receive notices with real map/channel boundaries. |
| M2 — Entity/world runtime foundation | [~] | Live player runtime, the first generic entity registry, the first owned map index, and the first transport-only session directory now exist; shared-world transport routing and the first transfer rebootstrap burst now go through owned runtime boundaries, the reconnect/teardown contract is now frozen in docs, duplicate-live `ENTERGAME` rejection can now stay retryable on the same encrypted game socket while the waiting session remains in `LOADING`, `gamed` now exposes loopback-only static-actor seed/snapshot/update/remove paths, owned map-occupancy snapshots now include static actors on their effective maps, entering players now receive the first client-visible bootstrap burst for already-visible static actors, and `MOVE` / `SYNC_POSITION` now rebuild self-facing static-actor visibility under configured AOI, while broader reconnect hardening, richer AOI, and fuller non-player systems still need to replace the remaining bootstrap shortcuts. |
| M3 — Character systems | [ ] | Inventory, equipment, item use, and character-state persistence exist in usable form. |
| M4 — Combat vertical slice | [ ] | Targeting, attacks, damage, death, and respawn work for at least one minimal content path. |
| M5 — Content runtime | [ ] | NPCs, mobs, spawns, shops, and the first quest/script runtime are available. |
| M6 — Compatibility-grade persistence and operations | [ ] | DB-backed persistence, richer observability/admin tooling, CI maturity, and deploy guidance are in place. |

## Project layout

- `cmd/authd` — auth daemon entrypoint
- `cmd/gamed` — game daemon entrypoint
- `internal/boot` — minimal connect -> handshake -> login boot flow composition
- `internal/config` — runtime config loading
- `internal/handshake` — server-side control-plane handshake flow
- `internal/login` — login-by-key flow and selection-surface transition
- `internal/minimal` — stub session factories used by the current authd/gamed bootstrap runtime
- `internal/player` — live player-runtime objects that separate selected-session world state from persisted bootstrap character snapshots
- `internal/worldruntime` — bootstrap topology, visibility helpers, and the first generic entity/runtime seams (entity registry + player directory + map index + transport-only session directory, now used by the bootstrap shared-world transport fanout/routing path and topology-aware social scope queries)
- `internal/warp` — minimal bootstrap transfer/warp boundary used to route gameplay-triggered map moves through a dedicated package
- `internal/accountstore` — file-backed bootstrap account/character snapshots used across fresh sessions
- `internal/staticstore` — deterministic file-backed bootstrap static-actor snapshots, ready for later boot/runtime wiring
- `internal/ops` — health and pprof endpoints
- `internal/service` — shared service bootstrap / shutdown helpers and legacy session runtime hooks
- `docs/` — engineering and clean-room documentation
- `spec/protocol` — protocol notes and packet inventory

## Key documents

- `docs/workflow.md`
- `docs/testing-strategy.md`
- `docs/qa/manual-client-checklist.md`
- `docs/clean-room-policy.md`
- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/roadmaps/2026-04-18-global-project-assessment.md`
- `docs/plans/2026-04-18-open-mt2-style-next-steps-roadmap.md`
- `docs/plans/2026-04-21-world-runtime-and-character-state-next-twenty-five-slices.md`
- `docs/plans/2026-04-24-runtime-aoi-and-static-actor-next-ten-slices.md`
- `spec/protocol/README.md`
- `spec/protocol/session-phases.md`
- `spec/protocol/frame-layout.md`
- `spec/protocol/control-handshake.md`
- `spec/protocol/auth-login.md`
- `spec/protocol/login-selection.md`
- `spec/protocol/select-world-entry.md`
- `spec/protocol/character-delete-selection.md`
- `spec/protocol/client-version-loading.md`
- `spec/protocol/game-ping-pong.md`
- `spec/protocol/shared-world-peer-visibility.md`
- `spec/protocol/move-peer-fanout.md`
- `spec/protocol/sync-position-peer-fanout.md`
- `spec/protocol/local-chat-peer-fanout.md`
- `spec/protocol/whisper-name-routing.md`
- `spec/protocol/party-chat-bootstrap.md`
- `spec/protocol/guild-chat-bootstrap.md`
- `spec/protocol/shout-chat-bootstrap.md`
- `spec/protocol/info-notice-bootstrap.md`
- `spec/protocol/server-notice-broadcast.md`
- `spec/protocol/chat-scope-first-hardening.md`
- `spec/protocol/map-index-world-scope-hardening.md`
- `spec/protocol/world-topology-bootstrap.md`
- `spec/protocol/visibility-rebuild.md`
- `spec/protocol/entity-runtime-bootstrap.md`
- `spec/protocol/map-relocation-visibility-rebuild.md`
- `spec/protocol/bootstrap-map-transfer-contract.md`
- `spec/protocol/transfer-rebootstrap-burst.md`
- `spec/protocol/runtime-reconnect-cleanup.md`
- `spec/protocol/non-player-entity-bootstrap.md`
- `spec/protocol/static-actor-interaction-bootstrap.md`
- `spec/protocol/static-actor-interaction-request.md`
- `spec/protocol/visible-world-bootstrap.md`
- `spec/protocol/character-update-bootstrap.md`
- `spec/protocol/player-point-change-bootstrap.md`
- `spec/protocol/sync-position-bootstrap.md`
- `spec/protocol/boot-path.md`
- `spec/protocol/packet-matrix.md`

## pprof

Both binaries expose an ops server with:
- `/healthz`
- `/debug/pprof/`
- `/debug/pprof/goroutine`
- `/debug/pprof/heap`
- `/debug/pprof/profile`
- `/debug/pprof/allocs`
- `/debug/pprof/block`
- `/debug/pprof/mutex`
- `/debug/pprof/trace`

`gamed` also exposes:
- `POST /local/notice`
  - loopback clients only
  - raw request body = notice text
  - queues a bootstrap `CHAT_TYPE_NOTICE` broadcast to connected `GAME` sessions
- `POST /local/relocate`
  - loopback clients only
  - JSON body: `{\"name\":\"CharacterName\",\"map_index\":42,\"x\":1700,\"y\":2800}`
  - compatibility/operator shim that applies the same bootstrap map transfer but only returns plain-text success/failure
- `POST /local/relocate-preview`
  - loopback clients only
  - JSON body: `{\"name\":\"CharacterName\",\"map_index\":42,\"x\":1700,\"y\":2800}`
  - previews the visibility and map-occupancy effects of that relocation without mutating runtime state
  - returns JSON with the current snapshot, target snapshot, removed/added visible peers, and map occupancy deltas
- `POST /local/transfer`
  - loopback clients only
  - JSON body: `{\"name\":\"CharacterName\",\"map_index\":42,\"x\":1700,\"y\":2800}`
  - commits the minimal structured bootstrap map-transfer contract
  - returns JSON with `applied = true` plus the committed transfer result
- `GET /local/players`
  - loopback clients only
  - returns a JSON snapshot of currently connected bootstrap characters, sorted by name
  - exposes the effective runtime location fields used by the current shared-world bootstrap (`name`, `vid`, `map_index`, `x`, `y`, `empire`, `guild_id`)
- `GET /local/visibility`
  - loopback clients only
  - returns a JSON snapshot of currently connected bootstrap characters plus the peers each one can currently see under the shared-world bootstrap rules
  - each entry exposes the same effective runtime location fields plus `visible_peers` and `visible_static_actors`
- `GET` / `POST /local/static-actors` and `PATCH` / `PUT` / `DELETE /local/static-actors/{entity_id}`
  - loopback clients only
  - create/update bodies use JSON fields `name`, `map_index`, `x`, `y`, `race_num`
  - create/update may also carry optional paired `interaction_kind` and `interaction_ref`
  - if one interaction field is present, the other must also be present
- `GET /local/maps`
  - loopback clients only
  - returns a JSON snapshot of effective `MapIndex` occupancy in the current bootstrap runtime
  - each entry exposes `map_index`, `character_count`, and a `characters` array sorted by name

Default addresses:
- `gamed`: `:6060`
- `authd`: `:6061`

Global override:
- `METIN2_PPROF_ADDR`

Per-service overrides:
- `METIN2_GAMED_PPROF_ADDR`
- `METIN2_AUTHD_PPROF_ADDR`

Examples:

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
curl http://127.0.0.1:6060/debug/pprof/goroutine?debug=1
curl -X POST http://127.0.0.1:6060/local/notice --data 'server maintenance'
curl -X POST http://127.0.0.1:6060/local/relocate \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
curl -X POST http://127.0.0.1:6060/local/relocate-preview \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
curl -X POST http://127.0.0.1:6060/local/transfer \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
curl http://127.0.0.1:6060/local/players
curl http://127.0.0.1:6060/local/visibility
curl http://127.0.0.1:6060/local/maps
```

Do not expose pprof directly to the public internet.

## Legacy listener runtime

Current default legacy listener addresses:
- `authd`: `:11002`
- `gamed`: `:13000`

Global override:
- `METIN2_LEGACY_ADDR`

Per-service overrides:
- `METIN2_AUTHD_LEGACY_ADDR`
- `METIN2_GAMED_LEGACY_ADDR`

Current advertised/public host default:
- `127.0.0.1`

Global override:
- `METIN2_PUBLIC_ADDR`

Per-service overrides:
- `METIN2_AUTHD_PUBLIC_ADDR`
- `METIN2_GAMED_PUBLIC_ADDR`

Notes:
- `gamed` currently advertises `PublicAddr + port(LegacyAddr)` in `LOGIN_SUCCESS4`.
- For local testing, `127.0.0.1` is fine.
- For a remote Windows client, set `METIN2_GAMED_PUBLIC_ADDR` to the host/IP the client should connect to.

Current stub bootstrap credentials:
- login: `mkmk`
- password: `hunter2`
- auth login key: `0x01020304`

Current minimal runtime path exposed by the shipped binaries:
- `authd`: `HANDSHAKE -> AUTH -> LOGIN3 -> AUTH_SUCCESS`
- `gamed`: `HANDSHAKE -> LOGIN -> SELECT -> EMPIRE_SELECT? -> CHARACTER_CREATE? -> CHARACTER_DELETE? -> CHARACTER_SELECT -> LOADING -> CLIENT_VERSION? -> ENTERGAME -> GAME -> CHARACTER_ADD -> CHAR_ADDITIONAL_INFO -> CHARACTER_UPDATE -> PLAYER_POINT_CHANGE -> peer CHARACTER_ADD/CHAR_ADDITIONAL_INFO/CHARACTER_UPDATE/CHARACTER_DEL -> peer MOVE/SYNC_POSITION/CHAT -> MOVE/SYNC_POSITION/CHAT/WHISPER/PARTY_CHAT/GUILD_CHAT/SHOUT_CHAT/INFO_CHAT`

This is still a bootstrap runtime, not full gameplay.
What exists today:
- shared authd -> gamed login tickets
- file-backed bootstrap account snapshots for the stub login
- character creation that survives fresh auth/game sessions
- character deletion that persists an empty slot across fresh auth/game sessions
- bootstrap character snapshots now persist `MapIndex`, with new characters defaulting to bootstrap map `1`
- deterministic single-character `MOVE` replication/ack using the selected character VID
- deterministic selected-character `SYNC_POSITION` reconciliation in `GAME`
- deterministic selected-character local talking `GC_CHAT` echo in `GAME`
- deterministic selected-character whisper `WHISPER_TYPE_NOT_EXIST` sender feedback for unknown targets in `GAME`
- bootstrap movement updates character coordinates and persists them across fresh auth/game sessions
- empty-account bootstrap flow can select empire before first character creation, and that choice persists across fresh auth/game sessions
- tolerant `CLIENT_VERSION` acceptance in `LOADING` with no phase transition and no server response
- tolerant `PONG` acceptance in `GAME` with no phase transition and no server response
- the selected character is inserted into the visible world after `ENTERGAME` via minimal `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` + `PLAYER_POINT_CHANGE`
- a later entering player receives already-connected peers as `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` bootstrap frames when they share the same bootstrap `MapIndex`
- already-connected peers receive queued `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` when a new player enters and `CHARACTER_DEL` when that peer disconnects, again within the same bootstrap `MapIndex`
- already-connected peers on the same bootstrap `MapIndex` receive queued `MOVE` replication when a visible peer moves
- already-connected peers on the same bootstrap `MapIndex` receive queued `SYNC_POSITION` replication when a visible peer reconciles position
- same-empire already-connected peers on the same bootstrap `MapIndex` receive queued local talking `GC_CHAT` deliveries when a visible peer chats
- named connected peers receive direct `GC_WHISPER` delivery when another player whispers them, while unknown targets return `WHISPER_TYPE_NOT_EXIST` to the sender
- currently connected `GAME` sessions act as one temporary bootstrap party for `CHAT_TYPE_PARTY` fanout
- connected `GAME` sessions with the same non-zero `GuildID` receive bootstrap `CHAT_TYPE_GUILD` fanout
- connected `GAME` sessions in the same empire receive bootstrap `CHAT_TYPE_SHOUT` fanout
- `CHAT_TYPE_INFO` currently acts as a bootstrap system/self channel with `vid = 0` and raw message text
- a server-originated `CHAT_TYPE_NOTICE` path now queues raw system notices with `vid = 0` to connected `GAME` sessions, and `gamed` exposes that path through loopback-only `POST /local/notice`; client-originated `CHAT_TYPE_NOTICE` remains rejected
- `gamed` also exposes loopback-only `POST /local/relocate` so an already-connected bootstrap character can still be moved to another `MapIndex` by exact name through the older operator shim
- `gamed` also exposes loopback-only `POST /local/relocate-preview` so operators can inspect visible-peer and map-occupancy changes before applying a bootstrap relocation
- `gamed` also exposes loopback-only `POST /local/transfer` as the minimal structured bootstrap map-transfer contract, returning the applied transfer result when the commit succeeds
- the first gameplay-side transfer trigger now exists too: an internal exact-position trigger can commit that same bootstrap transfer contract from `MOVE` / `SYNC_POSITION`, while deliberately leaving the final self-facing warp/loading reply for a later slice
- `gamed` also exposes loopback-only `GET /local/players` so the current connected bootstrap-character snapshot can be inspected before and after operator-driven runtime changes
- `gamed` also exposes loopback-only `GET /local/visibility` so the current shared-world visibility graph can be inspected before and after operator-driven runtime changes
- `gamed` also exposes loopback-only `GET /local/maps` so effective `MapIndex` occupancy can be inspected before and after operator-driven runtime changes

What still does not exist yet:
- compatibility-grade persistence matching the legacy target
- full gameplay/world state beyond the boot path

## Development

### Local

```bash
make test
```

Run the daemons locally:

```bash
go run ./cmd/authd
go run ./cmd/gamed
```

### Docker

Build the default lightweight runtime image:

```bash
docker build --target runtime -t go-metin2-server:latest .
```

Build the debug-flavoured runtime image:

```bash
docker build --target runtime-debug -t go-metin2-server:debug .
```

Why this Dockerfile keeps debug information:
- it uses a multi-stage build,
- it keeps the final image small with Distroless,
- it deliberately does not pass `-ldflags="-s -w"`, so DWARF/debug symbols remain available.

## Clean-room rule

This repository must only contain code, documentation and fixtures produced for this project.
Do not copy legacy Metin2 server/client source into this repository.
