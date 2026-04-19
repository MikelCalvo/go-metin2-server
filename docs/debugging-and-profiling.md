# Debugging and profiling

The project ships with a dedicated ops HTTP server that exposes standard Go pprof handlers.

## Standard endpoints

- `/healthz`
- `/debug/pprof/`
- `/debug/pprof/allocs`
- `/debug/pprof/block`
- `/debug/pprof/goroutine`
- `/debug/pprof/heap`
- `/debug/pprof/mutex`
- `/debug/pprof/profile`
- `/debug/pprof/threadcreate`
- `/debug/pprof/trace`

## Structured session logs

The daemons also emit structured JSON logs to stdout/stderr in normal service mode.

For phase-aware flows, the legacy TCP runtime now logs:

- `legacy session started`
  - includes `remote_addr`
  - includes the current `phase` when the session flow exposes it
- `legacy session phase changed`
  - includes `remote_addr`
  - includes `from_phase`
  - includes `to_phase`
- `legacy session closed with error`
  - includes `remote_addr`
  - includes the terminal error

These logs are intended to help real-client debugging around:
- handshake completion
- auth/login handoff
- selection-to-loading transition
- loading-to-game transition

When debugging the `LOADING -> GAME` boundary, the current expected server-owned burst after accepted `ENTERGAME` is:
- `PHASE(GAME)`
- selected-character `CHARACTER_ADD`
- selected-character `CHAR_ADDITIONAL_INFO`
- selected-character `CHARACTER_UPDATE`
- selected-character `PLAYER_POINT_CHANGE`
- then any trailing peer-visibility frames for already-visible players

## Local-only `gamed` operator endpoints

These endpoints are intentionally loopback-only and exist to help inspect or steer the bootstrap runtime safely during development.
They are not the gameplay protocol.

### `POST /local/notice`

- request body: raw plain-text notice message
- success response: `queued N`
- rejects non-loopback callers with `403`

### `POST /local/relocate`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- compatibility/operator shim for the older plain-text relocation trigger
- still applies the bootstrap map transfer
- success response: `relocated 1`
- rejects non-loopback callers with `403`

### `POST /local/relocate-preview`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- previews the visibility and map-occupancy effects of that relocation without mutating runtime state
- returns JSON with:
  - `applied`
  - `character`
  - `target`
  - `current_visible_peers`
  - `target_visible_peers`
  - `removed_visible_peers`
  - `added_visible_peers`
  - `map_occupancy_changes`
- rejects non-loopback callers with `403`

### `POST /local/transfer`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- commits the minimal structured bootstrap map-transfer contract
- returns the same JSON shape as preview, but with `applied = true`
- rejects non-loopback callers with `403`

### `GET /local/players`

Returns a JSON snapshot of the currently connected bootstrap characters, sorted by name.

Current fields:

- `name`
- `vid`
- `map_index`
- `x`
- `y`
- `empire`
- `guild_id`

The `map_index` field reflects the effective runtime map boundary currently used by the shared-world bootstrap.

### `GET /local/visibility`

Returns a JSON snapshot of the current shared-world visibility graph, sorted by character name.

Each entry includes the same effective runtime location fields exposed by `/local/players`, plus:

- `visible_peers`

The `visible_peers` array is sorted by peer name and reflects the current bootstrap shared-world rule set.
At the moment that means visibility is gated by effective `MapIndex` only.

### `GET /local/maps`

Returns a JSON snapshot of current effective `MapIndex` occupancy in the bootstrap runtime, sorted by `map_index`.

Each entry includes:

- `map_index`
- `character_count`
- `characters`

The `characters` array is sorted by name and each character uses the same effective runtime location fields exposed by `/local/players`.

## Examples

Collect a CPU profile for 30 seconds:

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
```

Inspect heap:

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

Dump goroutines in text form:

```bash
curl http://127.0.0.1:6060/debug/pprof/goroutine?debug=1
```

Open the interactive pprof UI locally:

```bash
go tool pprof -http=:0 http://127.0.0.1:6060/debug/pprof/heap
```

Send a local-only notice:

```bash
curl -X POST http://127.0.0.1:6060/local/notice --data 'server maintenance'
```

Relocate a connected bootstrap character locally:

```bash
curl -X POST http://127.0.0.1:6060/local/relocate \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
```

Preview a bootstrap relocation without mutating runtime state:

```bash
curl -X POST http://127.0.0.1:6060/local/relocate-preview \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
```

Commit a bootstrap transfer and get the structured applied result:

```bash
curl -X POST http://127.0.0.1:6060/local/transfer \
  -H 'Content-Type: application/json' \
  --data '{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}'
```

Inspect currently connected bootstrap characters:

```bash
curl http://127.0.0.1:6060/local/players
```

Inspect the current bootstrap shared-world visibility graph:

```bash
curl http://127.0.0.1:6060/local/visibility
```

Inspect current bootstrap map occupancy:

```bash
curl http://127.0.0.1:6060/local/maps
```

## Docker note

The runtime image keeps debug information because builds are not stripped with `-ldflags="-s -w"`.
That preserves DWARF/symbol data for better profiling and stack analysis while still using a lightweight final image.
