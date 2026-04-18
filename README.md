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
- A stub-compatible binary smoke path that reaches `GAME` with the current public bootstrap flows.
- A deterministic single-character `MOVE` round-trip wired through the current bootstrap runtime.
- A deterministic selected-character `SYNC_POSITION` reconciliation path wired through the current bootstrap runtime.
- A tolerant `CLIENT_VERSION` metadata path accepted during `LOADING` before `ENTERGAME`.
- A tolerant `PONG` control path accepted in `GAME` for server-driven ping probes.
- Character deletion on the selection surface with deterministic success/failure responses.
- A first visible-world bootstrap that inserts the selected character into the game world after `ENTERGAME`.
- Minimal shared-world peer visibility for players that are already connected to the bootstrap runtime and share the same bootstrap `MapIndex`.
- Minimal MOVE fanout so visible peers on the same bootstrap `MapIndex` receive queued movement replication from other connected players.
- Minimal `SYNC_POSITION` fanout so visible peers on the same bootstrap `MapIndex` receive queued reconciliation updates from other connected players.
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

## Project layout

- `cmd/authd` — auth daemon entrypoint
- `cmd/gamed` — game daemon entrypoint
- `internal/boot` — minimal connect -> handshake -> login boot flow composition
- `internal/config` — runtime config loading
- `internal/handshake` — server-side control-plane handshake flow
- `internal/login` — login-by-key flow and selection-surface transition
- `internal/minimal` — stub session factories used by the current authd/gamed bootstrap runtime
- `internal/accountstore` — file-backed bootstrap account/character snapshots used across fresh sessions
- `internal/ops` — health and pprof endpoints
- `internal/service` — shared service bootstrap / shutdown helpers and legacy session runtime hooks
- `docs/` — engineering and clean-room documentation
- `spec/protocol` — protocol notes and packet inventory

## Key documents

- `docs/workflow.md`
- `docs/testing-strategy.md`
- `docs/clean-room-policy.md`
- `docs/development.md`
- `docs/debugging-and-profiling.md`
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
