# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

## Status

This repository is the public bootstrap for a fresh, protocol-first rewrite.

Current scope of the project:
- Go 1.26 baseline.
- Clean-room process: no legacy server/client code is vendored into this repository.
- Separate `authd` and `gamed` binaries from day zero.
- A dedicated pprof/ops HTTP server for profiling goroutines, heap, allocs, mutex contention and blocking.
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
- [ ] socket-level handshake proof with the real client

### 4. Authentication and selection surface
- [ ] login request handling
- [ ] login success/failure path
- [ ] character list surface
- [ ] empire selection support if required
- [ ] character creation and selection

### 5. World entry
- [ ] loading/bootstrap packets
- [ ] main character bootstrap
- [ ] player points/stat bootstrap
- [ ] enter-game flow

### 6. First in-world behavior
- [ ] minimal world state
- [ ] basic movement handling
- [ ] movement replication/ack path

### 7. Hardening and expansion
- [ ] persistence layer that matches the compatibility target
- [ ] observability and operator tooling beyond pprof/health
- [ ] additional gameplay systems after the boot path is stable
- [ ] production packaging and deployment guidance

## Project layout

- `cmd/authd` — auth daemon entrypoint
- `cmd/gamed` — game daemon entrypoint
- `internal/config` — runtime config loading
- `internal/handshake` — server-side control-plane handshake flow
- `internal/ops` — health and pprof endpoints
- `internal/service` — shared service bootstrap / shutdown helpers
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
```

Do not expose pprof directly to the public internet.

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
