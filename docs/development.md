# Development

## Baseline

- Go 1.26
- two daemons: `authd` and `gamed`
- dedicated ops/pprof server per daemon
- Docker multi-stage build with lightweight runtime image

## Commands

```bash
make test
make build
```

Run locally:

```bash
go run ./cmd/authd
go run ./cmd/gamed
```

## Docker

Build the default lightweight runtime image:

```bash
docker build --target runtime -t go-metin2-server:latest .
```

Build the debug-flavoured runtime image:

```bash
docker build --target runtime-debug -t go-metin2-server:debug .
```

The Dockerfile keeps debug information on purpose: the final image stays small, but DWARF/symbol data is preserved for better profiling and stack analysis.

## Public CI

The repository ships with a GitHub Actions baseline workflow in `.github/workflows/ci.yml`.

It currently validates:

- `gofmt` cleanliness
- `go test ./...`
- `go vet ./...`
- daemon builds for `authd` and `gamed`
- Docker runtime and debug image builds

The intent is simple: every small slice should be pushable and publicly re-checkable.

## Runtime configuration

### pprof

- `authd`: `METIN2_AUTHD_PPROF_ADDR` (default `:6061`)
- `gamed`: `METIN2_GAMED_PPROF_ADDR` (default `:6060`)
- global override: `METIN2_PPROF_ADDR`

### Legacy TCP listeners

- `authd`: `METIN2_AUTHD_LEGACY_ADDR` (default `:11002`)
- `gamed`: `METIN2_GAMED_LEGACY_ADDR` (default `:13000`)
- global override: `METIN2_LEGACY_ADDR`

### Advertised public host

- default: `127.0.0.1`
- `authd`: `METIN2_AUTHD_PUBLIC_ADDR`
- `gamed`: `METIN2_GAMED_PUBLIC_ADDR`
- global override: `METIN2_PUBLIC_ADDR`

`gamed` currently advertises `PublicAddr + port(LegacyAddr)` in `LOGIN_SUCCESS4`.

### Bootstrap visibility policy

`gamed` defaults to whole-map bootstrap visibility: connected players and static actors share visibility when they are in the same effective `MapIndex` on the local bootstrap channel.

The runtime can opt into the radius AOI policy with environment overrides:

- `METIN2_VISIBILITY_MODE` / `METIN2_GAMED_VISIBILITY_MODE`
  - default: `whole_map`
  - supported values: `whole_map`, `radius`
  - values are normalized by trimming whitespace, lowercasing, and treating `-` as `_`
- `METIN2_VISIBILITY_RADIUS` / `METIN2_GAMED_VISIBILITY_RADIUS`
  - required positive integer when `visibility_mode = radius`
- `METIN2_VISIBILITY_SECTOR_SIZE` / `METIN2_GAMED_VISIBILITY_SECTOR_SIZE`
  - required positive integer when `visibility_mode = radius`

Service-specific overrides take precedence over global overrides for each field. Invalid visibility mode or non-positive radius/sector values fail `gamed` startup instead of falling back silently.

Use the loopback-only `GET /local/runtime-config` endpoint to confirm the policy the running `gamed` process actually booted with.

### Bootstrap QA reference

For the default stub credentials and the current real-client smoke flow, see the [manual client QA checklist](qa/manual-client-checklist.md).

### Bootstrap dummy combat state

- the current `training_dummy` HP loop is shared-world runtime state, not account/character persistence
- accepted dummy hits currently mutate only the dummy's live runtime combat state and self-only target refresh feedback
- debugging a dummy-hit issue should therefore start in `internal/worldruntime` / `internal/minimal`, not in item, inventory, or character-save code
- a process restart or world rebuild may legitimately recreate dummy HP because no persistence contract exists for this bootstrap slice yet
- the next authored content seam for loading attackable combatants from bundle data is documented in [spec/protocol/content-spawn-groups-bootstrap.md](../spec/protocol/content-spawn-groups-bootstrap.md)

## Legacy session runtime hooks

The legacy TCP runtime supports two optional per-session hooks:

- `FlushServerFrames() ([][]byte, error)` — allows a session flow to emit server-initiated frames even when no new client packet has arrived yet
- `io.Closer` — allows a session flow to release shared runtime state when the TCP session ends

The runtime checks for pending server frames between client reads.
This now powers asynchronous peer visibility *and* the bootstrap training-dummy dead-timer respawn rebuild path (`DEAD` -> delayed `CHARACTER_DEL` + add/info/update).

## Initial engineering priorities

1. freeze TMP4 target client compatibility
2. define boot-path packet matrix
3. implement TCP framing tests
4. implement session state machine tests
5. implement handshake/login/select/create/enter/move incrementally
