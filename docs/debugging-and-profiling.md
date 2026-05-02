# Debugging and profiling

The project ships with a dedicated ops HTTP server that exposes standard Go pprof handlers.

Do not expose pprof directly to the public internet.

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

When a session flow exposes the secure legacy transport hooks, the runtime also decrypts incoming post-handshake bytes and encrypts outgoing post-handshake bytes transparently after the plaintext `KEY_COMPLETE` boundary.

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
Unless noted otherwise, non-loopback callers are rejected with `403`.

### `GET /local/runtime-config`

Returns JSON describing the active bootstrap runtime selection, including the current local-channel wiring and visibility policy (`whole_map` or configured radius AOI).

### `POST /local/notice`

- request body: raw plain-text notice message
- success response: `queued N`

### `POST /local/relocate`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- compatibility/operator shim for the older plain-text relocation trigger
- still applies the bootstrap map transfer
- success response: `relocated 1`

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

### `POST /local/transfer`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- commits the minimal structured bootstrap map-transfer contract
- returns the same JSON shape as preview, but with `applied = true`

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
- `visible_static_actors`

### `GET /local/maps`

Returns a JSON snapshot of current effective `MapIndex` occupancy in the bootstrap runtime, sorted by `map_index`.

Each entry includes:

- `map_index`
- `character_count`
- `characters`

The `characters` array is sorted by name and each character uses the same effective runtime location fields exposed by `/local/players`.
Static actors are surfaced in the owned map snapshots as the current runtime expands beyond player-only visibility.

### `GET /local/interaction-visibility`

Returns a JSON snapshot of each connected bootstrap character plus the currently visible interactable static actors that would resolve for them.

Each visible interactable entry includes:

- `interaction_kind`
- `interaction_ref`
- a compact preview, or
- `resolution_failure`

Current previews cover self-only `info` / `talk`, structured merchant `shop_preview` catalog summaries, and compact `warp` destination summaries.

### `GET /local/inventory/{name}`, `GET /local/equipment/{name}`, `GET /local/currency/{name}`

Returns the exact-name live M3 runtime state for the selected character.
These endpoints are intended for loopback-only debugging and QA while the gameplay-facing surfaces are still bootstrap.

### `GET` / `POST /local/static-actors` and `PATCH` / `PUT` / `DELETE /local/static-actors/{entity_id}`

Use these endpoints to inspect and author bootstrap static actors.

Create/update bodies currently use:

- `name`
- `map_index`
- `x`
- `y`
- `race_num`
- optional paired `interaction_kind` and `interaction_ref`

If one interaction field is present, the other must also be present.

### `GET` / `POST /local/interactions` and `PATCH` / `PUT` / `DELETE /local/interactions/{kind}/{ref}`

Use these endpoints to inspect and author the deterministic interaction catalog.

Bodies always use identity fields:

- `kind`
- `ref`

Current authored shapes:

- `info` / `talk`
  - `text`
- `shop_preview`
  - `title`
  - `catalog[]` entries with `slot`, `item_vnum`, `price`, `count`
- `warp`
  - `map_index`, `x`, `y`
  - optional `text`

`PATCH` and `PUT` are full-identity upserts, so body `kind` + `ref` must match the path exactly.
Deletes fail closed while a bootstrap static actor still references the definition.

### Combat ownership troubleshooting workflow

Use the current local-only runtime endpoints together when combat target ownership looks wrong:

1. `GET /local/players`
   - confirm the authoritative live owner is the expected selected character instance after reconnect/reclaim
2. `GET /local/visibility`
   - confirm whether the dummy is still visible to that live owner before assuming a combat bug
3. `POST /local/relocate-preview`
   - simulate range/visibility-loss moves before mutating runtime state, then compare with the real `MOVE` / `SYNC_POSITION` path
4. `POST /local/transfer`
   - reproduce transfer rebootstrap cleanup explicitly when checking whether stale target ownership survives across a fresh bootstrap
5. `GET` / `PATCH` / `PUT /local/static-actors/{entity_id}`
   - inspect or replace the current dummy snapshot in place when reproducing replaced-target fail-closed behavior

Current combat ownership debugging expectations:

- range or visibility loss should eventually collapse the live session's selected target to one self-only `GC TARGET(0, 0)`
- transfer, `/phase_select` re-entry, and reconnect should all require a fresh accepted `TARGET` before the next live `ATTACK`
- stale post-reclaim sockets may still produce self-local noise, but they must not mutate runtime dummy HP or the replacement live owner's selected target state
- dummy HP is runtime-owned only; after a harness/operator-injected `0` HP or in-place actor replacement, the old selected snapshot must fail closed until the session reselects target intent

### `GET` / `POST /local/content-bundle`

Exports or imports one deterministic authored-content artifact spanning both bootstrap static actors and interaction definitions.

- `GET` exports the current bundle
- `POST` imports a full replacement bundle
- imports reject dangling interaction references before mutating runtime state

A small reference artifact lives at `docs/examples/bootstrap-npc-service-bundle.json`.

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

Inspect runtime wiring:

```bash
curl http://127.0.0.1:6060/local/runtime-config
```

Send a local-only notice:

```bash
curl -X POST http://127.0.0.1:6060/local/notice --data 'server maintenance'
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

Inspect visible interactable actors:

```bash
curl http://127.0.0.1:6060/local/interaction-visibility
```

Inspect live inventory, equipment, and currency for a character:

```bash
curl http://127.0.0.1:6060/local/inventory/MkmkWar
curl http://127.0.0.1:6060/local/equipment/MkmkWar
curl http://127.0.0.1:6060/local/currency/MkmkWar
```

List the authored interaction catalog:

```bash
curl http://127.0.0.1:6060/local/interactions
```

Create a talk interaction:

```bash
curl -X POST http://127.0.0.1:6060/local/interactions \
  -H 'Content-Type: application/json' \
  --data '{"kind":"talk","ref":"npc:village_guard","text":"Keep your blade sharp."}'
```

Create a bootstrap static actor bound to that interaction:

```bash
curl -X POST http://127.0.0.1:6060/local/static-actors \
  -H 'Content-Type: application/json' \
  --data '{"name":"Village Guard","map_index":1,"x":1234,"y":5678,"race_num":20355,"interaction_kind":"talk","interaction_ref":"npc:village_guard"}'
```

Export the current authored content bundle:

```bash
curl http://127.0.0.1:6060/local/content-bundle
```

## Docker note

The runtime image keeps debug information because builds are not stripped with `-ldflags="-s -w"`.
That preserves DWARF/symbol data for better profiling and stack analysis while still using a lightweight final image.
