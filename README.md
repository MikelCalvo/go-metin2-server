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
- Minimal MOVE fanout so visible peers on the same bootstrap `MapIndex` receive queued movement replication from other connected players.
- Minimal `SYNC_POSITION` fanout so visible peers on the same bootstrap `MapIndex` receive queued reconciliation updates from other connected players.
- An internal server-side map-relocation visibility rebuild primitive that removes peers from the old bootstrap `MapIndex` and bootstraps peers on the destination `MapIndex`.
- A loopback-only `gamed` relocation ops trigger that exercises bootstrap `MapIndex` relocation by exact character name without freezing a final client warp contract.
- A loopback-only `gamed` relocation dry-run endpoint that previews visibility and map-occupancy effects before applying a bootstrap `MapIndex` relocation.
- A loopback-only `gamed` structured transfer endpoint that commits the minimal bootstrap map-transfer contract and returns the applied transfer result.
- A first owned self-session bootstrap transfer contract: transfer-triggered `MOVE` / `SYNC_POSITION` suppress immediate self ack packets and currently reuse queued self visibility-delta frames instead of a final warp/loading packet.
- A loopback-only `gamed` runtime snapshot endpoint that lists currently connected bootstrap characters and their effective map/position state.
- A loopback-only `gamed` runtime visibility endpoint that shows which currently connected bootstrap characters can see each other under the shared-world bootstrap rules.
- A loopback-only `gamed` runtime map-occupancy endpoint that groups currently connected bootstrap characters by effective `MapIndex`.
- Minimal local talking chat fanout so same-empire visible peers on the same bootstrap `MapIndex` receive queued `GC_CHAT` deliveries from other connected players.
- Minimal whisper routing by exact character name across currently connected bootstrap sessions.
- Minimal bootstrap `CHAT_TYPE_PARTY` fanout across the currently connected `GAME` sessions.
- Minimal bootstrap `CHAT_TYPE_GUILD` fanout across connected `GAME` sessions that share the same non-zero `GuildID`.
- Minimal bootstrap `CHAT_TYPE_SHOUT` fanout across connected `GAME` sessions in the same empire.
- Minimal bootstrap system `CHAT_TYPE_INFO` self-delivery in `GAME`, plus a server-originated `CHAT_TYPE_NOTICE` broadcast path exposed through a local-only `gamed` ops endpoint; client-originated `CHAT_TYPE_NOTICE` remains rejected.
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
| Shared player visibility | [~] | Peer enter/leave visibility exists for players on the same bootstrap `MapIndex`. |
| Movement replication | [~] | `MOVE` and `SYNC_POSITION` fan out to already-visible peers on the same bootstrap `MapIndex`. |
| Map boundaries | [~] | An explicit bootstrap topology now owns local channel/map chat scoping, but there is still no final warp flow yet. |
| Channel topology | [ ] | No real multi-channel topology, shard routing, or inter-channel ownership yet. |
| Interest management / culling | [ ] | No range-, sector-, or AOI-based visibility yet. |
| Warp / map transfer | [~] | A server-side visibility-rebuild primitive, structured transfer commit path, and first self-session transfer reply contract exist, but there is still no final end-to-end client/server warp flow yet. |
| Entity runtime beyond players | [ ] | No NPC, mob, item-ground, or generic entity layer yet. |

### Social, chat, and operator surfaces

| Area | Status | Notes |
| --- | --- | --- |
| Local talking chat | [~] | Same-map + same-empire fanout exists; still bootstrap scope. |
| Whisper | [x] | Exact-name whisper routing works across connected bootstrap sessions. |
| Party chat | [~] | Bootstrap-only fanout; no real party membership system yet. |
| Guild chat | [~] | Scoped by non-zero `GuildID`, but no guild lifecycle/roster system exists yet. |
| Shout | [~] | Same-empire bootstrap fanout exists; no real world/channel topology behind it yet. |
| System info / notice | [~] | `INFO` self-delivery, server-originated `NOTICE`, and local-only notice trigger exist. |
| Operator/admin surface | [~] | Loopback-only `POST /local/notice`, `POST /local/relocate`, `POST /local/relocate-preview`, `POST /local/transfer`, `GET /local/players`, `GET /local/visibility`, and `GET /local/maps` exist on `gamed`; broader admin/auth tooling does not. |

### Character systems and gameplay

| Area | Status | Notes |
| --- | --- | --- |
| Character snapshots / bootstrap stats | [~] | Enough for selection, spawn, movement, chat, and bootstrap transfer-trigger slices. |
| Gameplay transfer triggers | [~] | First exact-position trigger can commit bootstrap map transfer from `MOVE` / `SYNC_POSITION`; the current self-facing contract is documented, but the final warp/loading packet is still not frozen. |
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
| Database schema / migrations | [ ] | No real DB-backed persistence layer or live migrations yet. |
| Observability | [~] | Health, pprof, and small local-only notice/relocation/runtime-introspection/map-occupancy/dry-run/transfer endpoints exist; metrics/logging/admin depth still needs work. |
| CI / public validation | [x] | GitHub Actions baseline checks formatting, tests, vet, daemon builds, and runtime/debug image builds. |
| Release/deploy guidance | [ ] | No production-grade release/deployment story yet. |

## Milestone ladder

| Milestone | Status | Exit criteria |
| --- | --- | --- |
| M0 — Protocol-owned boot path | [x] | Handshake, auth/login, selection, create/delete/select, enter-game, and first movement loop are stable. |
| M1 — Shared-world pre-alpha | [~] | Players can see each other, move, chat, and receive notices with real map/channel boundaries. |
| M2 — Entity/world runtime foundation | [ ] | Real world/map/channel ownership, warp flow, and a reusable entity runtime replace the current bootstrap shortcuts. |
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
- `internal/warp` — minimal bootstrap transfer/warp boundary used to route gameplay-triggered map moves through a dedicated package
- `internal/accountstore` — file-backed bootstrap account/character snapshots used across fresh sessions
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
- `spec/protocol/map-relocation-visibility-rebuild.md`
- `spec/protocol/bootstrap-map-transfer-contract.md`
- `spec/protocol/map-transfer-bootstrap.md`
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
  - each entry exposes the same effective runtime location fields plus a `visible_peers` array
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
