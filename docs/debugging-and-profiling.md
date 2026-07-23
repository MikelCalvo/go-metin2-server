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

### `POST /local/account-store/validate`

Validates the durable bootstrap account snapshot store through the same strict loader used by runtime backup/restore primitives, without mutating any account files. This endpoint is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if any committed account snapshot is corrupt, has an invalid filename/login pairing, or violates the deterministic account snapshot invariants.

Successful responses are JSON summaries with:

- `account_count`
- `character_count`
- `logins` sorted in deterministic account-list order

Crash leftovers such as hidden `.account-*.json` temp files are ignored, matching the committed-snapshot list/backup contract.

### `POST /local/account-store/backup`

Copies the durable bootstrap account snapshot store into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, and returns `409` if the source store is invalid, the destination is non-empty, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

The backup path uses the same committed-snapshot list/validate contract as `/local/account-store/validate`: hidden crash-temp files are ignored, corrupt committed snapshots fail closed, and successful responses contain `account_count`, `character_count`, and deterministic `logins` for the backup that was just written. A successful backup also writes `account-backup-manifest.json` with the backup format marker, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums. The destination must be empty and must not be equal to or nested under the active account-store directory, including through destination symlinks, so this endpoint cannot silently merge unrelated operator files with a runtime backup or recursively copy its own in-progress output. If account copying, manifest writing, or the final destination-directory sync fails after files were committed, backup removes the account files and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/account-store/backup/validate`

Dry-runs an operator-supplied account backup source through the same strict restore-source loader and backup-manifest checks used by `/local/account-store/restore`, but does not create or mutate the active account-store directory. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `account-backup-manifest.json` plus committed account snapshots that pass the strict account-store loader

Successful responses are the deterministic backup summary (`account_count`, `character_count`, sorted `logins`) that would be restored. Use this endpoint as the preflight check before pointing a fresh replacement account-store path at `/local/account-store/restore`; manually assembled snapshot directories without the manifest are rejected instead of being treated as restorable backups.

### `POST /local/account-store/restore`

Restores the durable bootstrap account snapshot store from an operator-supplied source backup directory into the active store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, and returns `409` if the source backup is missing or invalid, the active account-store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `account-backup-manifest.json` plus committed account snapshots that pass the strict account-store loader

The restore path uses the same committed-snapshot list/validate contract as backup: `account-backup-manifest.json` is required as the backup integrity marker and is ignored by the account loader, hidden crash-temp files in the backup are ignored, corrupt committed snapshots fail closed, and restore refuses to merge into a non-empty active store directory. Restore validates the manifest format marker, deterministic summary, per-account filenames, byte sizes, and SHA-256 checksums before creating or copying into the destination store; missing manifests, malformed manifests, or checksum drift fail closed with `409` and leave the empty destination untouched. A successful restore writes a fresh `account-backup-manifest.json` for the restored account set, so the replacement store can immediately be validated or copied again through the backup preflight path. If copying starts but a later account save, manifest write, or final directory sync fails, restore removes the account files and manifest it already committed and syncs the restore directory again so operators are not left with a partially restored account set. Operators should use this only as a bootstrap recovery primitive for an empty replacement account-store path, not as an online merge/import API.

### `GET /local/runtime-config`

Returns JSON describing the active bootstrap runtime selection. This endpoint is read-only, rejects non-`GET` methods with `405`, and exposes only the local runtime facts needed for AOI/debugging:

- `local_channel_id`
- `visibility_mode` (`whole_map`, `radius`, or `custom` for future non-standard policies)
- `visibility_radius`
- `visibility_sector_size`

The default bootstrap runtime reports local channel `1` and whole-map visibility. When `gamed` is configured for radius AOI, this snapshot reports the active radius and sector-size values selected from the `METIN2_VISIBILITY_*` / `METIN2_GAMED_VISIBILITY_*` environment overrides.

### `GET` / `POST /local/static-actor-combat-profiles`

Lists and registers process-local bootstrap static-actor combat profiles for later static-actor or spawn-group authoring. This is loopback-only operator tooling, not gameplay protocol and not durable content storage.

`GET` returns a deterministic JSON list under `profiles`, including the built-in `practice_mob` and `training_dummy` profiles plus any registered process-local profiles. Each entry exposes the same canonical defaults returned by registration, including derived `damage_per_normal_attack`, formula fields, presentation fields, respawn delay, and cloned reward descriptors. Nested `death_reward` fields use the same stable snake-case JSON keys accepted by `POST`: `experience`, `gold`, and `drop_vnums`.

`POST` request body JSON fields:

- `profile` — lowercase snake-case profile name; built-in names and duplicates are rejected
- `max_hp`
- optional `damage_per_normal_attack`
- optional formula fields `attack_value` / `defense_value`
- optional presentation fields `level` / `rank`
- `respawn_delay_ms`
- optional `death_reward` with `experience`, `gold`, and `drop_vnums`

On success `POST` returns the canonicalized profile defaults, including any derived `damage_per_normal_attack` and sorted reward drop vnums. Invalid JSON, unknown fields, invalid formulas, invalid reward descriptors, non-loopback callers, and methods other than `GET` / `POST` fail closed.

### `GET` / `POST /local/content-bundle`

Exports or imports the deterministic authored bootstrap content bundle used by static actors, interaction definitions, spawn groups, and their authored combat-profile snapshots.

`GET` canonicalizes and validates the exported bundle before writing JSON, so local operators always receive the same deterministic shape used by bundle import/example tests. If the runtime exporter ever returns an invalid bundle, the endpoint fails closed with `500` instead of leaking a partial or non-canonical snapshot.

`POST` canonicalizes and validates the whole bundle before applying it. The request body is bounded to 1 MiB and oversized bodies are rejected before import. In addition to the per-row validation, non-built-in `combat_profiles` entries must be referenced by at least one static actor or spawn group in the same bundle; unreferenced snapshots are rejected so this endpoint cannot mutate process-local combat profiles without importing authored content that uses them. Structured merchant `shop_preview` definitions and item-shaped reward drops (`reward_drop_vnums` on spawn groups or bundled combat-profile defaults) must also carry the referenced `item_templates` in the same portable bundle; bundles that omit those templates are rejected before import.

### `GET /local/content-bundle/summary`

Returns a loopback-only JSON summary of the currently exported canonical content bundle without returning the full authoring payload.

The summary includes deterministic counts for static actors, spawn groups, portable combat profiles, item templates, interaction definitions, referenced/unreferenced interaction definitions, interaction kinds, and per-map static actor / spawn-group occupancy. It is read-only and uses the same export + canonicalization rules as `GET /local/content-bundle`; if the live authored content cannot be exported as a valid bundle, the endpoint fails closed with `500` rather than summarizing a partial snapshot. Non-loopback callers return `403`, and methods other than `GET` return `405`.

### `POST /local/content-bundle/validate`

Validates and canonicalizes an authored bootstrap content bundle without importing or mutating runtime state. This loopback-only endpoint uses the same 1 MiB request bound, strict JSON decoding, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle`.

Successful responses return the canonical bundle JSON that would be accepted by import. Invalid JSON, unknown fields, dangling refs, invalid static actors/spawn groups/combat profiles, missing merchant or reward-drop item templates, and other bundle validation failures return `400`; non-loopback callers return `403`; methods other than `POST` return `405`.

Use this as an on-box dry-run check before applying a larger content bundle or before committing updates to deterministic example bundles. The repository-owned `docs/examples/bootstrap-npc-service-bundle.json` fixture is required to stay byte-for-byte canonical under this same validation path, so operators can paste it directly into `/local/content-bundle/validate` or `/local/content-bundle` without hidden normalization drift.

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
  - `current_visible_static_actors`
  - `target_visible_static_actors`
  - `removed_visible_static_actors`
  - `added_visible_static_actors`
  - `before_map_occupancy`
  - `after_map_occupancy`
  - `map_occupancy_changes`

Visible static-actor entries in this preview now also expose `dead: true` while a runtime-owned practice mob remains in its owned dead interval before respawn.
`before_map_occupancy` and `after_map_occupancy` also include currently pending bootstrap ground items, preserving transient ground occupancy across dry-run map snapshots without redefining `map_occupancy_changes`, which remains character-count oriented.
Player snapshots in the same preview now also expose `dead: true` while a still-connected engaged owner remains at the current retaliation-owned `0`-HP floor, whether that owner appears as `character`, `target`, or a visible peer.

### `POST /local/transfer`

- request body: JSON
- example:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

- commits the minimal structured bootstrap map-transfer contract
- returns the same JSON shape as preview, but with `applied = true`
- the same static-actor `dead: true` flag is preserved in transfer results while a runtime-owned practice mob remains dead before respawn
- the same player `dead: true` flag is preserved in transfer results while a still-connected owner remains at that retaliation-owned `0`-HP floor
- if that same dead owner is moved into another live peer's visible world or into visibility of another static actor through this loopback path, live peers still receive the ordinary queued peer-entry burst plus trailing `GC DEAD(owner_vid)` for that owner, while the dead owner itself now skips both the queued destination peer-entry burst and any queued destination static-actor bootstrap burst and keeps only any old-world cleanup frames still needed locally

### `GET /local/runtime-config`

Returns a loopback-only JSON snapshot of the active `gamed` bootstrap runtime policy, so operators can verify the visibility/AOI mode the daemon actually booted with instead of inferring it from environment variables.

Current fields:

- `local_channel_id`
- `visibility_mode` (`whole_map`, `radius`, or `custom` for future non-standard policies)
- `visibility_radius`
- `visibility_sector_size`

`whole_map` remains the default bootstrap behavior and reports zero radius/sector values. `radius` reports the configured runtime AOI radius and sector size from the active topology policy.

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
- `dead`

The `map_index` field reflects the effective runtime map boundary currently used by the shared-world bootstrap.

### `GET /local/visibility`

Returns a JSON snapshot of the current shared-world visibility graph, sorted by character name.

Each entry includes the same effective runtime location fields exposed by `/local/players`, plus:

- `visible_peers`
- `visible_static_actors`
- `visible_ground_items`

Connected-character and visible-peer player entries now also expose `dead: true` while a still-connected owner remains at the retaliation-owned `0`-HP floor.
Visible static-actor entries now also expose `dead: true` while a runtime-owned practice mob is still in its server-owned dead interval.
`visible_ground_items` reports the item-shaped and gold-shaped ground rewards currently visible to that specific connected character, sorted by visible ground `vid`, using the same fields exposed by `/local/ground-items`.

### `GET /local/maps`

Returns a JSON snapshot of current effective `MapIndex` occupancy in the bootstrap runtime, sorted by `map_index`.

Each entry includes:

- `map_index`
- `character_count`
- `characters`
- `static_actor_count`
- `static_actors`
- `ground_item_count`
- `ground_items`

The `characters` array is sorted by name and each character uses the same effective runtime location fields exposed by `/local/players`, including the current `dead` flag.
Static actors are surfaced in the owned map snapshots as the current runtime expands beyond player-only visibility.
Those static-actor entries now also expose `dead: true` while a runtime-owned practice mob is still dead before respawn.
Temporary pending ground items are surfaced with their visible `vid`, `vnum`, optional `count`, optional display `owner_name`, owner identity (`owner_login`, `owner_character_id`, `owner_vid`), optional `gold_amount`, effective `map_index`, and `x/y/z` position so operator map snapshots show both connected actors and transient ground occupancy without losing the owned ground-entry identity used by stale-pickup guards.

### `GET /local/ground-items`

Returns a flat JSON snapshot of all currently pending bootstrap ground entries, sorted by visible ground `vid`.
This is a loopback-only debug view of the same transient item-shaped and gold-shaped rewards already included in `/local/maps`; it does not expose a gameplay pickup API and does not mutate ground state.
Successful gameplay pickup removes the entry from this flat list, the by-VID lookup below, and `/local/maps` occupancy together.

Each entry includes:

- `vid`
- `vnum` for item-shaped ground rewards
- `count` for item-shaped ground rewards
- `owner_name`
- `owner_login`
- `owner_character_id`
- `owner_vid`
- `gold_amount` for gold-shaped ground rewards
- `map_index`
- `x`
- `y`
- `z`

### `GET /local/ground-items/{vid}`

Returns one pending bootstrap ground entry by its visible ground `vid` using the same JSON fields as `/local/ground-items`.
This endpoint is also loopback-only and read-only. Decimal and `0x`-prefixed hexadecimal `vid` path values are accepted to match the way runtime/debug logs commonly show VIDs. Invalid or missing `vid` path values return `400`; well-formed but absent `vid` values return `404`.

### `GET /local/interaction-visibility`

Returns a JSON snapshot of each connected bootstrap character plus the currently visible interactable static actors that would resolve for them.

Each visible interactable entry includes:

- `interaction_kind`
- `interaction_ref`
- a compact preview, or
- `resolution_failure`

Current previews cover self-only `info` / `talk`, structured merchant `shop_preview` catalog summaries, and compact `warp` destination summaries.
The per-character subject snapshot in this endpoint also reuses the same player `dead: true` flag exposed by `/local/players` while a still-connected owner remains at the retaliation-owned `0`-HP floor.

### `GET /local/inventory/{name}`, `GET /local/equipment/{name}`, `GET /local/currency/{name}`

Returns the exact-name live M3 runtime state for the selected character.
These endpoints are intended for loopback-only debugging and QA while the gameplay-facing surfaces are still bootstrap.

### `GET /local/combat-target/{name}` and `GET /local/combat-targets`

`GET /local/combat-target/{name}` returns the exact-name selected combat-target snapshot for a connected bootstrap character when that session currently owns a visible runtime combat target.
`GET /local/combat-targets` returns the deterministic list of all currently resolved active combat-target snapshots, sorted by runtime subject entity ID, so loopback QA can inspect target ownership without already knowing the selected character name.
Both responses reuse the runtime debug snapshot shape documented in `spec/protocol/combat-normal-attack-bootstrap.md`:

- `subject_entity_id`
- `subject`
- `target_vid`
- `snapshot_version`
- `hp_percent`
- `actor`

The embedded `subject` field uses the same effective connected-character snapshot shape exposed by `/local/players`, so combat-target debugging can verify the current owner location/dead-state without a second lookup.
Both endpoints are loopback-only and read-only. The exact-name endpoint returns `404` when the character is not connected, no longer has a live session hook, has no active target, or the target no longer resolves through the current visibility/runtime combat rules; the list endpoint omits unresolved/stale selections instead of leaking hidden or invalid target data.

### `GET` / `POST /local/static-actors` and `PATCH` / `PUT` / `DELETE /local/static-actors/{entity_id}`

Use these endpoints to inspect and author bootstrap static actors.

Create/update bodies currently use:

- `name`
- `map_index`
- `x`
- `y`
- `race_num`
- optional paired `interaction_kind` and `interaction_ref`
- optional `combat_profile`

If one interaction field is present, the other must also be present.
`combat_profile` follows the same bootstrap profile identifiers accepted by content bundles and spawn groups, letting local operator create/update calls seed practice-mob/training-dummy descriptors without importing a full bundle.
Returned static-actor snapshots now also expose `dead: true` while a runtime-owned practice mob is still in its server-owned dead interval, including `DELETE /local/static-actors/{entity_id}` responses when a dead dummy is removed before respawn.

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

`POST`, `PATCH`, and `PUT` bodies are bounded to 4 KiB; oversized authored interaction requests fail closed with `413` before reaching runtime mutation callbacks. `PATCH` and `PUT` are full-identity upserts, so body `kind` + `ref` must match the path exactly.
Interaction `ref` values must use the canonical path-safe `<namespace>:<name>` form (for example `npc:qa_merchant` or `lore:qa_square`); slashes, whitespace, dots, hyphens, uppercase letters, missing namespaces, blank segments, and extra `:` separators are rejected before persistence/import.
Deletes fail closed while a bootstrap static actor still references the definition.

### Combat ownership troubleshooting workflow

Use the current local-only runtime endpoints together when combat target ownership looks wrong:

1. `GET /local/players`
   - confirm the authoritative live owner is the expected selected character instance after reconnect/reclaim, and check `dead: true` before assuming later silent owner-side rejection is a targeting bug
2. `GET /local/visibility`
   - confirm whether the dummy is still visible to that live owner before assuming a combat bug, and check `dead: true` on both visible practice mobs and still-connected player owners before treating a no-target/no-attack result as unexpected
3. `POST /local/relocate-preview`
   - simulate range/visibility-loss moves before mutating runtime state, then compare with the real `MOVE` / `SYNC_POSITION` path; dead practice mobs now stay marked `dead: true` in the previewed static-actor arrays, and dead player subjects / peers now keep the same flag there too
4. `POST /local/transfer`
   - reproduce transfer rebootstrap cleanup explicitly when checking whether stale target ownership survives across a fresh bootstrap; dead practice mobs now stay marked `dead: true` in the applied structured result, and dead player subjects / peers do too
5. `GET /local/combat-targets`
   - list all currently resolved active target selections when debugging multi-session target ownership without knowing every character name first
6. `GET` / `PATCH` / `PUT /local/static-actors/{entity_id}`
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

Inspect one currently resolved combat-target selection by exact character name:

```bash
curl http://127.0.0.1:6060/local/combat-target/MkmkWar
```

List all currently resolved combat-target selections:

```bash
curl http://127.0.0.1:6060/local/combat-targets
```

Both combat-target endpoints are read-only local runtime snapshots. They do not introduce new client packets and still fail closed when a selected target is stale, invisible, or no longer combat-targetable.

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
