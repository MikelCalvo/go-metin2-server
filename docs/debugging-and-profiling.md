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
The ops/pprof HTTP listener now defaults to `127.0.0.1` (`127.0.0.1:6061` for `authd`, `127.0.0.1:6060` for `gamed`) and daemon/service startup rejects wildcard or non-loopback pprof binds such as `:6060`, `0.0.0.0:6060`, `[::]:6060`, or a remote hostname. Keep remote access behind an explicit local transport such as SSH tunneling rather than exposing this mux directly.

### `POST /local/account-store/validate`

Validates the durable bootstrap account snapshot store through the same strict loader used by runtime backup/restore primitives, without mutating any account files. This endpoint is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if any committed account snapshot is corrupt, is not valid UTF-8, has an invalid filename/login pairing, uses a non-canonical case-variant filename, duplicates a login by case variant, uses an empty, whitespace-padded, or embedded-NUL account login, or violates the deterministic account snapshot invariants. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely. Multiple all-zero character records are accepted as empty select-screen slots, but any zero-ID slot with leftover name, VID, location, stat, guild, gold, item, equipment, or quickslot state fails validation; non-zero character records must carry a non-empty, unpadded, NUL-free name and any persisted guild name must also be NUL-free.

Successful responses are JSON summaries with:

- `account_count`
- `character_count` — persisted select-screen character records, including all-zero empty slot placeholders
- optional `empty_character_slot_count` when all-zero empty slot placeholders are present
- `logins` sorted in deterministic account-list order
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.account-*.json` temp files are present

Crash leftovers such as hidden `.account-*.json` temp files are not treated as committed snapshots, matching the committed-snapshot list/backup contract, but validation reports them as deterministic residue so an operator can see interrupted account writes before cleanup or recovery work.

### `POST /local/account-store/crash-temps/cleanup`

Removes same-directory `.account-*.json` crash-temp residue from the durable bootstrap account snapshot store after first validating the committed snapshot set through the same strict loader used by `/local/account-store/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if committed account state is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup account-store JSON summary (`account_count`, `character_count`, optional `empty_character_slot_count`, and deterministic `logins`). Because cleanup validates before removing anything, corrupt committed account snapshots leave crash-temp files in place for manual recovery. Only hidden `.account-*.json` temp files are removed; committed account snapshots, backup manifests, and unrelated hidden files are preserved. Use `/local/account-store/validate` first when you want a read-only residue report, then this endpoint when the operator has decided the interrupted temp writes are disposable.

### `POST /local/login-tickets/validate`

Validates the one-shot authd-to-gamed login-ticket handoff store without consuming or deleting any tickets. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if any committed ticket is corrupt, is not valid UTF-8, has unknown/trailing JSON, has an invalid or mismatched filename/login-key pairing, has an empty, whitespace-padded, or embedded-NUL login, has a zero login key, has a missing/zero `issued_at`, or violates the character/item/equipment/quickslot invariants shared with ticket load/consume. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely. Multiple all-zero character records are accepted as empty select-screen slots, but any zero-ID slot with leftover persisted character state fails validation before the handoff can be consumed; non-zero character records must carry a non-empty, unpadded, NUL-free name and any persisted guild name must also be NUL-free.

Successful responses are JSON summaries with:

- `ticket_count`
- `character_count` — select-screen character records embedded in pending tickets, including all-zero empty slot placeholders
- optional `empty_character_slot_count` when all-zero empty slot placeholders are present in pending ticket payloads
- `logins` sorted in deterministic ticket-list order
- `login_keys` in the same order as `logins`
- optional `oldest_issued_at` / `newest_issued_at` bounds when at least one committed ticket is present
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.ticket-*.json` temp files are present

Crash leftovers such as hidden `.ticket-*.json` temp files are not treated as pending handoff tickets, but validation reports them as deterministic residue. The issued-at bounds are calculated from committed tickets only, so operators can quickly see the age span of pending one-shot handoffs before choosing a cutoff for stale-ticket preview or cleanup. Use this endpoint to inspect both pending committed handoff state and interrupted ticket writes before debugging authd/gamed login-key issues; it is not a replay, consume, restore, or remote admin API.

### `POST /local/login-tickets/crash-temps/cleanup`

Removes same-directory `.ticket-*.json` crash-temp residue from the one-shot login-ticket handoff store after first validating the committed ticket set through the same strict loader used by `/local/login-tickets/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if committed ticket state is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup login-ticket JSON summary (`ticket_count`, `character_count`, optional `empty_character_slot_count`, deterministic `logins`, matching `login_keys`, and issued-at bounds when committed tickets remain). Because cleanup validates before removing anything, corrupt committed tickets leave crash-temp files in place for manual recovery. Only hidden `.ticket-*.json` temp files are removed; committed handoff tickets and unrelated hidden files are preserved. Use `/local/login-tickets/validate` first when you want a read-only residue report, then this endpoint when the operator has decided interrupted temp ticket writes are disposable.

### `POST /local/login-tickets/issued-before/preview`

Dry-runs the stale login-ticket cutoff logic without consuming or deleting any one-shot handoff tickets. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed ticket set fails validation.

Request body JSON fields:

- `issued_before` — RFC3339/RFC3339Nano timestamp cutoff; tickets with `issued_at < issued_before` are reported as stale

The preview path validates the whole committed ticket set through the same strict listing boundary used by `/local/login-tickets/validate`, including the requirement that every committed ticket has a non-zero `issued_at`, reports `stale_count`, deterministic `stale_logins` / `stale_login_keys`, and embeds the unchanged `current` login-ticket summary with issued-at bounds plus embedded character and empty-slot counts when tickets are present. Hidden `.ticket-*.json` crash-temp files are visible in the `current` summary but are not stale cleanup candidates. Use this endpoint before `/local/login-tickets/issued-before/cleanup` when an operator wants a no-mutation audit of abandoned authd-to-gamed handoff keys.

### `POST /local/login-tickets/issued-before/cleanup`

Removes committed one-shot login-ticket files whose `issued_at` timestamp is strictly older than an operator-supplied cutoff. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed ticket set fails validation or if deletion/directory sync fails.

Request body JSON fields:

- `issued_before` — RFC3339/RFC3339Nano timestamp cutoff; only tickets with `issued_at < issued_before` are removed

The cleanup path validates the whole committed ticket set through the same strict listing boundary used by `/local/login-tickets/validate` before deleting anything, so corrupt committed tickets or tickets with missing/zero `issued_at` fail closed and leave all pending handoff files available for inspection. Hidden `.ticket-*.json` crash-temp files are reported in the returned `remaining` summary but are not removed by this endpoint; use `/local/login-tickets/crash-temps/cleanup` for interrupted temp writes and `/local/login-tickets/issued-before/preview` for a no-mutation stale-ticket audit. Successful responses include the cutoff, `removed_count`, deterministic `removed_logins` / `removed_login_keys`, and a `remaining` login-ticket summary including issued-at bounds plus embedded character and empty-slot counts for the surviving handoff set, so operators can verify pending ticket count, select-screen payload size, and age span after pruning stale tickets. This is a bounded local recovery primitive for abandoned authd-to-gamed handoff keys, not a remote admin API or a normal ticket-consume path.

### `POST /local/item-templates/validate`

Validates the authored bootstrap item-template snapshot store without mutating item-template state. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed item-template snapshot is malformed, has unknown/trailing JSON, duplicates a vnum, or violates template policy such as invalid max counts, equipment slots, display metadata, use effects, or equip effects. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `template_count`
- `vnums` sorted in deterministic template order
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.item-templates-*.json` temp files are present

A missing committed `item-templates.json` is reported as an empty authored-template store, matching the runtime fallback to built-in bootstrap item templates. Crash leftovers are reported for operator visibility but are not treated as committed templates. Use this endpoint before importing content bundles, debugging merchant catalog/template mismatches, or planning item-template migration work; it is not a gameplay API or a remote admin API.

### `POST /local/item-templates/crash-temps/cleanup`

Removes same-directory `.item-templates-*.json` crash-temp residue from the authored bootstrap item-template snapshot store after first validating the committed `item-templates.json` snapshot through the same strict loader used by `/local/item-templates/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup item-template JSON summary (`template_count` and deterministic `vnums`). Because cleanup validates before removing anything, corrupt committed item-template snapshots leave crash-temp files in place for manual recovery. Only hidden `.item-templates-*.json` temp files are removed; committed snapshots and unrelated hidden files are preserved. Use `/local/item-templates/validate` first when you want a read-only residue report, then this endpoint when the operator has decided interrupted item-template temp writes are disposable.

### `POST /local/static-actor-store/validate`

Validates the authored bootstrap static-actor snapshot store without mutating actor content. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed `static-actors.json` snapshot is malformed, has unknown/trailing JSON, duplicates actor IDs or spawn-group refs, or violates actor policy such as missing names, zero map indexes, invalid race numbers, invalid interaction refs, invalid combat profiles, or invalid reward descriptors. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `actor_count`
- optional `interactable_actor_count`
- optional `spawn_group_count`
- deterministic `actor_ids`
- deterministic `actor_names`
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.static-actors-*.json` temp files are present

A missing committed `static-actors.json` is reported as an empty authored actor store. Crash leftovers are reported for operator visibility but are not treated as committed static actors. Use this endpoint before content import/restore work or before deleting static-actor crash-temp residue; it is not a gameplay API or a remote admin API.

### `POST /local/static-actor-store/crash-temps/cleanup`

Removes same-directory `.static-actors-*.json` crash-temp residue from the authored static-actor snapshot store after first validating the committed `static-actors.json` snapshot through the same strict loader used by `/local/static-actor-store/validate` and `/local/persistence/status`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup static-actor JSON summary (`actor_count`, deterministic `actor_ids` / `actor_names`, and authored-content counters). Because cleanup validates before removing anything, corrupt committed static-actor snapshots leave crash-temp files in place for manual recovery. Only hidden `.static-actors-*.json` temp files are removed; committed snapshots and unrelated hidden files are preserved.

### `POST /local/interaction-store/validate`

Validates the authored bootstrap interaction-definition snapshot store without mutating interaction content. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed `interaction-definitions.json` snapshot is malformed, has unknown/trailing JSON, duplicates a `kind:ref`, or violates definition policy for supported `info`, `talk`, `shop_preview`, or `warp` definitions. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `definition_count`
- deterministic `definition_keys` (`kind:ref`)
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.interaction-definitions-*.json` temp files are present

A missing committed `interaction-definitions.json` is reported as an empty authored interaction store. Crash leftovers are reported for operator visibility but are not treated as committed definitions. Use this endpoint before content import/restore work or before deleting interaction crash-temp residue; it is not a gameplay API or a remote admin API.

### `POST /local/interaction-store/crash-temps/cleanup`

Removes same-directory `.interaction-definitions-*.json` crash-temp residue from the authored interaction-definition snapshot store after first validating the committed `interaction-definitions.json` snapshot through the same strict loader used by `/local/interaction-store/validate` and `/local/persistence/status`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup interaction JSON summary (`definition_count` and deterministic `definition_keys`). Because cleanup validates before removing anything, corrupt committed interaction-definition snapshots leave crash-temp files in place for manual recovery. Only hidden `.interaction-definitions-*.json` temp files are removed; committed snapshots and unrelated hidden files are preserved.

### `POST /local/item-templates/backup`

Copies the authored bootstrap item-template snapshot into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source snapshot is invalid, the destination is non-empty, the destination is equal to or nested under the active item-template store directory, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

A successful backup writes `item-template-backup-manifest.json` with the backup format marker, deterministic item-template summary, copied snapshot filename, byte size, and SHA-256 checksum. Missing committed `item-templates.json` snapshots are backed up as an empty authored-template store with no synthetic snapshot file, preserving the runtime fallback to built-in bootstrap templates. Hidden `.item-templates-*.json` crash leftovers are ignored as backup payload. Before creating the requested destination, backup also validates any active restored item-template backup manifest against the current committed snapshot bytes; stale or malformed active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active item-template store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest against the current committed snapshot bytes and fail closed on malformed manifests, summary drift, size drift, checksum drift, filename drift, or a manifest that omits an existing committed snapshot. A later successful item-template save removes the restored manifest, so changed authored-template state stops claiming to be the exact restored backup. If snapshot copying, manifest writing, or final directory sync fails after files were committed, backup removes the snapshot file and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/item-templates/backup/validate`

Dry-runs an operator-supplied item-template backup source through the same strict manifest and snapshot checks used by backup, but does not create or mutate the active item-template store. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `item-template-backup-manifest.json` plus the committed item-template snapshot when the manifest lists one

Successful responses are the deterministic item-template backup summary (`template_count`, sorted `vnums`, plus optional `crash_temp_count` / `crash_temp_files`) that was validated. Backup directories are required to be manifest-closed and symlink-free: every non-temp entry must be either `item-template-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.item-templates-*.json` crash leftovers remain visible in the preflight summary but are not backup payload. The backup manifest itself must also be an ordinary file with valid UTF-8 before JSON decoding, matching the committed snapshot loader's fail-closed text boundary instead of accepting replacement-character drift in operator-managed metadata. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. This is a local recovery/audit primitive, not an online restore or live template reload API.

### `POST /local/item-templates/restore`

Restores the authored bootstrap item-template snapshot store from an operator-supplied source backup directory into the active item-template store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active item-template store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `item-template-backup-manifest.json` plus `item-templates.json` when the manifest lists a committed snapshot

The restore path is intentionally a replacement primitive, not an online merge or live gameplay reload API. It refuses to write into a non-empty active item-template directory, refuses destinations that are equal to or nested under the backup source (including symlink-resolved paths), refuses to run while connected selected-character sessions are live, validates the backup manifest and symlink-free backup entries before creating target files, preserves committed zero-template snapshots as real `item-templates.json` files, reloads the in-memory template index for future sessions after a successful restore, and writes a fresh `item-template-backup-manifest.json` alongside the restored snapshot so the replacement store can be preflighted or backed up again. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. The next successful item-template save removes that restored manifest so a later mutation cannot leave the active store looking like the exact validated backup source. Missing committed snapshots restore as an empty authored-template store, preserving the built-in runtime fallback.

### `POST /local/account-store/backup`

Copies the durable bootstrap account snapshot store into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only destination paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source store is invalid, the destination is non-empty, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

The backup path uses the same committed-snapshot list/validate contract as `/local/account-store/validate`: hidden crash-temp files are ignored, corrupt or non-UTF-8 committed snapshots fail closed, and successful responses contain `account_count`, `character_count`, optional `empty_character_slot_count`, and deterministic `logins` for the backup that was just written. A successful backup also writes `account-backup-manifest.json` with the backup format marker, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums. Before creating the destination, backup also validates any active restored backup manifest against the current committed account files; a stale or malformed active manifest fails closed with `409` and leaves the requested destination uncreated. If an active account store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest against the current committed account files and fail closed on malformed manifests, summary drift, size drift, checksum drift, or login/filename drift. A later successful account save removes the restored manifest, so changed live account state stops claiming to be the exact restored backup. The destination must be empty and must not be equal to or nested under the active account-store directory, including through destination symlinks, so this endpoint cannot silently merge unrelated operator files with a runtime backup or recursively copy its own in-progress output. If account copying, manifest writing, or the final destination-directory sync fails after files were committed, backup removes the account files and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/account-store/backup/validate`

Dry-runs an operator-supplied account backup source through the same strict restore-source loader and backup-manifest checks used by `/local/account-store/restore`, but does not create or mutate the active account-store directory. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only source paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, contains non-UTF-8 committed snapshots, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `account-backup-manifest.json` plus committed account snapshots that pass the strict account-store loader

Successful responses are the deterministic backup summary (`account_count`, `character_count`, optional `empty_character_slot_count`, sorted `logins`, plus optional `crash_temp_count` / `crash_temp_files`) that would be restored. Use this endpoint as the preflight check before pointing a fresh replacement account-store path at `/local/account-store/restore`; manually assembled snapshot directories without the manifest are rejected instead of being treated as restorable backups. Backup directories are also required to be manifest-closed and symlink-free: every non-temp entry must be either `account-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.account-*.json` crash-temp regular files remain visible in the preflight summary but are not restorable payload. The backup manifest itself must be an ordinary file with valid UTF-8 before JSON decoding, matching the committed account snapshot loader's fail-closed text boundary instead of accepting replacement-character drift in operator-managed metadata. Symlinked manifests, manifested account snapshots, or crash-temp-shaped entries fail closed with `409`; crash-temp-shaped directories are rejected as untracked entries instead of being silently ignored.

### `POST /local/account-store/restore`

Restores the durable bootstrap account snapshot store from an operator-supplied source backup directory into the active store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only source paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active account-store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `account-backup-manifest.json` plus committed account snapshots that pass the strict account-store loader

The restore path uses the same committed-snapshot list/validate contract as backup: `account-backup-manifest.json` is required as the backup integrity marker and is ignored by the account loader, hidden crash-temp files in the backup are ignored, corrupt or non-UTF-8 committed snapshots fail closed, and restore refuses to merge into a non-empty active store directory. In the shipped `gamed` runtime, restore also refuses to run while connected selected-character sessions are live, so an operator cannot replace durable account snapshots underneath in-memory player state. The destination restore directory must also be outside the source backup directory, including through destination symlinks, so restore cannot write a replacement store into the backup tree it is reading. Restore validates the manifest format marker, deterministic summary, exact-cased per-account logins, per-account filenames, byte sizes, SHA-256 checksums, symlink-free backup entries, and closed-directory coverage before creating or copying into the destination store; missing manifests, malformed manifests, symlinked manifests or account snapshots, checksum drift, manifest login case drift, untracked non-temp files/directories, crash-temp-shaped directories or symlinks, nested restore destinations, or live-session restore attempts fail closed with `409` and leave the destination untouched. A successful restore writes a fresh `account-backup-manifest.json` for the restored account set, so the replacement store can immediately be validated or copied again through the backup preflight path. The next successful account save removes that restored manifest so mutated live account state cannot be mistaken for the exact restored backup. If copying starts but a later account save, manifest write, or final directory sync fails, restore removes the account files and manifest it already committed and syncs the restore directory again so operators are not left with a partially restored account set. Operators should use this only as a bootstrap recovery primitive for an empty replacement account-store directory, not as an online merge, live reload, or database migration mechanism.

### `GET /local/runtime-config`

Returns JSON describing the active bootstrap runtime selection. This endpoint is read-only, rejects non-`GET` methods with `405`, and exposes only the local runtime facts needed for AOI/debugging:

- `local_channel_id`
- `visibility_mode` (`whole_map`, `radius`, or `custom` for future non-standard policies)
- `visibility_radius`
- `visibility_sector_size`
- `persistence.login_ticket_store_dir`
- `persistence.account_store_dir`
- `persistence.static_actor_store_path`
- `persistence.interaction_store_path`
- `persistence.item_template_store_path`

The default bootstrap runtime reports local channel `1` and whole-map visibility. When `gamed` is configured for radius AOI, this snapshot reports the active radius and sector-size values selected from the `METIN2_VISIBILITY_*` / `METIN2_GAMED_VISIBILITY_*` environment overrides.

The `persistence` block reports the active bootstrap JSON store locations selected from `METIN2_*_STORE_*` / service-specific environment overrides. Use it before running local backup, restore, validation, or stale-ticket cleanup endpoints so the operator confirms the daemon is pointing at the intended account, login-ticket, static-actor, interaction, and item-template stores. A running `gamed` has already rejected empty or overlapping persistence paths at startup: account and login-ticket directories must be separate trees, file-backed content stores must be separate files, and no file-backed store may resolve into either persistence directory.

### `GET /local/persistence/status`

Returns a loopback-only JSON health snapshot for the bootstrap persistence stores that already have strict runtime validation primitives. This endpoint is read-only, is registered only on `gamed`, rejects non-`GET` methods with `405`, and returns `200` even when one store is invalid so operators can inspect all store statuses in one response.

Current response fields:

- `ok` — `true` only when every included store validates successfully
- `account_store`
  - `path`
  - `valid`
  - `summary` with the same `account_count`, `character_count`, `empty_character_slot_count` when select-screen holes exist, `logins`, and optional crash-temp fields returned by `/local/account-store/validate`
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present in the active store directory
  - optional `error` when validation fails
- `login_ticket_store`
  - `path`
  - `valid`
  - `summary` with the same `ticket_count`, `character_count`, optional `empty_character_slot_count`, `logins`, `login_keys`, optional issued-at bounds, and optional crash-temp fields returned by `/local/login-tickets/validate`
  - optional `error` when validation fails
- `item_template_store`
  - `path`
  - `valid`
  - `summary` with the same `template_count`, `vnums`, and optional crash-temp fields returned by `/local/item-templates/validate`
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present next to the active item-template snapshot
  - optional `error` when validation fails
- `static_actor_store`
  - `path`
  - `valid`
  - `summary` with `actor_count`, deterministic `actor_ids` / `actor_names`, optional `interactable_actor_count`, optional `spawn_group_count`, and optional static-actor crash-temp fields
  - optional `error` when validation fails
- `interaction_store`
  - `path`
  - `valid`
  - `summary` with `definition_count`, deterministic `definition_keys` (`kind:ref`), and optional interaction crash-temp fields
  - optional `error` when validation fails

Use this endpoint as the first read-only persistence triage check before choosing a narrower validate, crash-temp cleanup, stale-ticket cleanup, backup, or restore endpoint. It deliberately keeps checking the remaining stores after one store fails, so a corrupt account snapshot does not hide healthy login-ticket, item-template, static-actor, or interaction-definition stores. Authored content stores that have no committed snapshot are reported as valid empty stores, while corrupt committed snapshots fail closed and still let operators inspect the other persistence surfaces in the same response. Static-actor and interaction-definition store files now also reject a JSON `null` document root or `null` root collection (`static_actors` / `definitions`) instead of treating those lossy snapshots as empty authored content. Login-ticket summaries now expose the embedded select-screen character count and empty-slot count just like account summaries, which helps operators compare pending one-shot authd handoffs against durable account snapshots before consuming or cleaning stale keys. Account and item-template stores that still carry a restored backup manifest now report that manifest explicitly under `backup_manifest`, and validation still verifies that active manifest against the current committed snapshot bytes; malformed manifests, stale checksum/summary data, or item-template manifests that omit an existing committed snapshot make the affected store invalid so operators can detect post-restore drift before treating a replacement store as an exact backup copy. It is an operator/debugging surface, not a gameplay API and not a remote admin API.

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

On success `POST` returns the canonicalized profile defaults, including any derived `damage_per_normal_attack` and sorted reward drop vnums. Request bodies are bounded to 4 KiB, oversized bodies return `413`, and invalid UTF-8 is rejected before JSON decoding or profile registration. Empty bodies, invalid JSON, unknown fields, trailing JSON, invalid formulas, invalid reward descriptors, non-loopback callers, and methods other than `GET` / `POST` fail closed.

### `GET` / `POST /local/content-bundle`

Exports or imports the deterministic authored bootstrap content bundle used by static actors, interaction definitions, spawn groups, and their authored combat-profile snapshots.

`GET` canonicalizes and validates the exported bundle before writing pretty-printed canonical JSON, so local operators always receive the same deterministic byte shape used by bundle import/example tests. Export keeps item templates needed only by expanded combat-profile reward-default drop lists as well as templates referenced directly by merchant catalogs and spawn groups, so reward-bearing custom-profile bundles remain self-contained and immediately re-importable. If the runtime exporter ever returns an invalid bundle, the endpoint fails closed with `500` instead of leaking a partial or non-canonical snapshot.

`POST` canonicalizes and validates the whole bundle before applying it, then returns the imported bundle with the same pretty-printed canonical JSON encoder used by export/validate. The request body is bounded to 1 MiB and oversized bodies are rejected before import; invalid UTF-8 request bodies and a JSON `null` root are also rejected before the import callback runs instead of being treated as an empty or lossy replacement bundle. In addition to the per-row validation, non-built-in `combat_profiles` entries must be referenced by at least one static actor or spawn group in the same bundle; unreferenced snapshots are rejected so this endpoint cannot mutate process-local combat profiles without importing authored content that uses them. If a portable combat-profile snapshot names a profile that is already registered in the running process, its canonical defaults must match exactly; conflicting snapshots are rejected before import. Structured merchant `shop_preview` definitions and item-shaped reward drops (`reward_drop_vnums` on spawn groups or bundled combat-profile defaults) must also carry the referenced `item_templates` in the same portable bundle; bundles that omit those templates are rejected before import. Duplicate reward drop vnums in either spawn groups or bundled combat-profile defaults are rejected rather than silently deduplicated during canonicalization.

### `GET` / `POST /local/content-bundle/summary`

Returns a loopback-only JSON summary of authored content bundles without returning the full authoring payload.

`GET` summarizes the currently exported canonical bundle. It is read-only and uses the same export + canonicalization rules as `GET /local/content-bundle`; if the live authored content cannot be exported as a valid bundle, the endpoint fails closed with `500` rather than summarizing a partial snapshot.

`POST` is a dry-run helper for candidate bundles. It uses the same 1 MiB request bound, strict JSON decoding, invalid UTF-8 rejection, JSON `null` root rejection, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle` / `POST /local/content-bundle/validate`, then returns only the compact summary. It does not call the live runtime exporter and does not import or mutate runtime state. Use it when an operator wants quick counts/kind/map impact for a candidate bundle before requesting the full canonical payload from `/local/content-bundle/validate` or committing it with `/local/content-bundle`.

The summary includes deterministic counts for static actors, interactable static actors, spawn groups, portable combat profiles, item templates, structured shop catalog entries, interaction definitions, per-kind referenced/unreferenced interaction definitions, exact referenced/unreferenced interaction definition identities, compact `interaction_definition_previews` for every authored definition, exact portable `static_actors` identities (`name`, `map_index`, `x`, `y`, `race_num`, optional `combat_profile`, optional `interaction_kind`, optional `interaction_ref`) for both plain and interactable actors, exact `interactable_static_actors` identities with the same compact `preview` strings used by `/local/interaction-visibility`, exact spawn-group identities (`ref`, `name`, `map_index`, `combat_profile`, and reward descriptor) plus resolved `reward_drop_items` metadata for every authored drop vnum, aggregate reward totals (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) plus deterministic `reward_drops` grouped by item vnum with `source_count` and resolved item metadata, exact portable `combat_profiles` snapshots, exact item-template identities (`vnum`, `name`, `stackable`, `max_count`, optional `shop_buy_price`), exact structured `shop_catalogs` with per-entry slot / item vnum / resolved item name / count / price / stack metadata, exact `warp_destinations`, and per-map static actor / interactable static actor / spawn-group occupancy with per-map `info_actor_count`, `talk_actor_count`, `shop_preview_actor_count`, `shop_catalog_entry_count`, `warp_actor_count`, spawn reward totals, and drop item counts. Invalid candidate bundles return `400`, non-loopback callers return `403`, and methods other than `GET` / `POST` return `405`.

### `POST /local/content-bundle/import-preview`

Previews the impact of importing a candidate authored bootstrap content bundle without mutating runtime state. This loopback-only endpoint uses the same 1 MiB request bound, strict JSON decoding, invalid UTF-8 rejection, JSON `null` root rejection, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle` / `POST /local/content-bundle/validate`, then compares the candidate against the currently exported canonical live bundle.

Successful responses are JSON with:

- `current` — the current live content-bundle summary
- `candidate` — the candidate bundle summary after canonicalization
- `deltas` — compact count deltas where each count has `current`, `candidate`, and signed `delta`

The current delta set covers static actors, exact added/removed portable static-actor rows, interactable actors, spawn groups, exact added/removed/changed spawn-group records, portable combat profiles, exact added/removed/changed combat-profile records, item templates, exact added/removed/changed item-template records, shop catalog entries, exact added/removed/changed shop catalog records, shop routes, exact added/removed/changed shop route records, warp destinations, exact added/removed/changed warp destination records, warp routes, exact added/removed/changed warp route records, reward drop items, interaction definitions, referenced interaction definitions, unreferenced interaction definitions, per-interaction-kind count/reference deltas, and a deterministic `maps` array for only map indexes whose tracked authored counts change. Each static-actor delta carries `change` (`added` or `removed`) plus the canonical `current` and/or `candidate` portable static-actor row that caused it; each item-template delta is keyed by `vnum`, carries `change` (`added`, `removed`, or `changed`), and includes the canonical `current` and/or `candidate` template snapshot that caused the change; each combat-profile delta is keyed by `profile`, carries `change` (`added`, `removed`, or `changed`), and includes the canonical current and/or candidate profile snapshot with HP, damage/formula, presentation, respawn delay, and death-reward defaults; each spawn-group delta is keyed by `ref`, carries the same `change` values, and includes the summarized `current` and/or `candidate` spawn group with resolved reward-drop item metadata when applicable; each reward-drop delta is keyed by item `vnum`, carries `change`, and includes the grouped current and/or candidate source counts plus template metadata; each shop-catalog delta is keyed by interaction `kind` + `ref`, carries `change`, and includes the canonical current and/or candidate catalog with title, entry count, and resolved per-entry item/template metadata; each warp-destination delta is keyed by interaction `kind` + `ref`, carries `change`, and includes the canonical current and/or candidate destination with optional text plus target `map_index`/`x`/`y`; each shop-route delta is keyed by actor name, source `map_index`/`x`/`y`, and interaction `ref`, carries `change`, and includes the compact current and/or candidate route with title and entry count; each warp-route delta uses the same actor/source/ref key and includes the compact current and/or candidate route with text plus target `map_index`/`x`/`y`; each interaction-kind delta carries before/after/signed values for total definitions, referenced definitions, and unreferenced definitions for the changed kind; each map delta carries before/after/signed count values for static actors, interactables, info/talk/shop/warp service actors, shop catalog entries, spawn groups, and reward-drop item counts. Invalid JSON, unknown fields, dangling refs, invalid static actors/spawn groups/combat profiles, missing merchant or reward-drop item templates, and other candidate bundle validation failures return `400`; non-loopback callers return `403`; methods other than `POST` return `405`. Use this endpoint when an operator wants a no-mutation before/after audit before applying a replacement bundle through `POST /local/content-bundle`.

### `POST /local/content-bundle/validate`

Validates and canonicalizes an authored bootstrap content bundle without importing or mutating runtime state. This loopback-only endpoint uses the same 1 MiB request bound, strict JSON decoding, invalid UTF-8 rejection, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle`. The underlying bundle decoder also rejects invalid UTF-8 before `encoding/json` can replace malformed bytes, so checked-in fixtures and non-HTTP tooling share the same fail-closed rule.

Successful responses return the canonical bundle JSON that would be accepted by import. Invalid JSON, invalid UTF-8, unknown fields, JSON `null` roots, dangling refs, invalid static actors/spawn groups/combat profiles, missing merchant or reward-drop item templates, and other bundle validation failures return `400`; non-loopback callers return `403`; methods other than `POST` return `405`.

Use this as an on-box dry-run check before applying a larger content bundle or before committing updates to deterministic example bundles. The repository-owned `docs/examples/bootstrap-npc-service-bundle.json` fixture is required to stay byte-for-byte canonical under this same validation path, so operators can paste it directly into `/local/content-bundle/validate` or `/local/content-bundle` without hidden normalization drift.

### `POST /local/notice`

Queues a server-originated `CHAT_TYPE_NOTICE` system message through the running `gamed` shared-world runtime. This endpoint is registered only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects empty or whitespace-only messages with `400`, rejects invalid UTF-8 request bodies with `400`, rejects bodies over 4 KiB with `413`, and trims leading/trailing whitespace before broadcasting. It is a local operator/debugging surface, not a gameplay chat path and not a remote admin API.

- request body: raw UTF-8 plain-text notice message
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
  - `current_visible_spawn_groups`
  - `target_visible_spawn_groups`
  - `removed_visible_spawn_groups`
  - `added_visible_spawn_groups`
  - `before_map_occupancy`
  - `after_map_occupancy`
  - `map_occupancy_changes`

Visible static-actor entries in this preview now also expose `dead: true` while a runtime-owned practice mob remains in its owned dead interval before respawn.
The visible spawn-group arrays are deterministic subsets of the matching static-actor arrays whose `spawn_group_ref` is non-empty, preserving the same `dead` and reward-descriptor fields as `/local/spawn-groups`.
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
- `visible_spawn_groups`
- `visible_ground_items`

Connected-character and visible-peer player entries now also expose `dead: true` while a still-connected owner remains at the retaliation-owned `0`-HP floor.
Visible static-actor entries now also expose `dead: true` while a runtime-owned practice mob is still in its server-owned dead interval.
`visible_spawn_groups` is the deterministic subset of `visible_static_actors` whose `spawn_group_ref` is non-empty, using the same spawn-backed static-actor snapshot shape as `/local/spawn-groups`; dead practice mobs keep the same `dead: true` flag in both arrays.
`visible_ground_items` reports the item-shaped and gold-shaped ground rewards currently visible to that specific connected character, sorted by visible ground `vid`, using the same fields exposed by `/local/ground-items`.

### `GET /local/maps`

Returns a JSON snapshot of current effective `MapIndex` occupancy in the bootstrap runtime, sorted by `map_index`.

Each entry includes:

- `map_index`
- `character_count`
- `characters`
- `static_actor_count`
- `static_actors`
- `spawn_group_count`
- `spawn_groups`
- `ground_item_count`
- `ground_items`

The `characters` array is sorted by name and each character uses the same effective runtime location fields exposed by `/local/players`, including the current `dead` flag.
Static actors are surfaced in the owned map snapshots as the current runtime expands beyond player-only visibility.
Those static-actor entries now also expose `dead: true` while a runtime-owned practice mob is still dead before respawn.
`spawn_group_count` and `spawn_groups` provide the deterministic per-map subset of those static actors whose `spawn_group_ref` is non-empty, using the same spawn-backed static-actor snapshot shape as `/local/spawn-groups`. This keeps attackable authored spawn presence inspectable from map occupancy without removing the actor from the full `static_actors` array.
Temporary pending ground items are surfaced with their visible `vid`, `vnum`, optional `count`, optional display `owner_name`, owner identity (`owner_login`, `owner_character_id`, `owner_vid`), optional `gold_amount`, effective `map_index`, and `x/y/z` position so operator map snapshots show both connected actors and transient ground occupancy without losing the owned ground-entry identity used by stale-pickup guards.

### `GET /local/maps/{map_index}`

Returns one current map-occupancy snapshot by effective `map_index`, using the same JSON shape as a single row from `/local/maps`.
This endpoint is loopback-only and read-only. Decimal and `0x`-prefixed hexadecimal map indexes are accepted for consistency with other local runtime lookups. Invalid, zero, or missing map indexes return `400`; well-formed but currently unoccupied maps return `404`.

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

Each visible interactable entry reuses the static-actor snapshot shape from `/local/static-actors`, including:

- `interaction_kind`
- `interaction_ref`
- `dead: true` while the target actor is currently at the bootstrap combat `0`-HP floor
- a compact preview, or
- `resolution_failure`

Current previews cover self-only `info` / `talk`, structured merchant `shop_preview` catalog summaries, and compact `warp` destination summaries.
The per-character subject snapshot in this endpoint also reuses the same player `dead: true` flag exposed by `/local/players` while a still-connected owner remains at the retaliation-owned `0`-HP floor.

### `GET /local/inventory/{name}`, `GET /local/equipment/{name}`, `GET /local/quickslots/{name}`, `GET /local/currency/{name}`

Returns the exact-name live M3 runtime state for the selected character.
These endpoints are intended for loopback-only debugging and QA while the gameplay-facing surfaces are still bootstrap.

`/local/inventory/{name}` returns carried items with `id`, `vnum`, `count`, `slot`, and optional `locked: true` for runtime-locked carried items.
`/local/equipment/{name}` returns equipped items with `id`, `vnum`, `count`, `equip_slot`, and optional `locked: true` for runtime-locked equipped items.
False lock state is omitted so existing unlocked snapshots keep their compact shape.

`/local/quickslots/{name}` returns the selected character's live quickslot bindings as sorted byte-sized tuples:

- `position`
- `type`
- `slot`

This lets operator QA compare visible quickslot edits, automatic item quickslot cleanup, and reconnect persistence without treating the endpoint as a gameplay API.

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

### `GET /local/static-actor-respawns` and `GET /local/static-actor-respawns/{entity_id}`

Returns the deterministic list of pending server-driven static-actor respawn timers for runtime-owned dead practice mobs.
These endpoints are loopback-only, read-only, reject non-`GET` methods with `405`, and are intended for QA/debugging of the dead interval frozen in `spec/protocol/non-player-death-respawn-bootstrap.md`.

Each row exposes:

- `entity_id` — runtime static-actor entity / client-visible static-actor `VID`
- `ready_at` — the server-owned timestamp when the next flush can rebuild the actor
- `remaining_ms` — milliseconds until `ready_at`, clamped to `0` when the timer is already due but has not yet been flushed through the pending server-frame path
- `actor` — the same static-actor snapshot shape used by `/local/static-actors`, with `dead: true` while the respawn remains pending

Rows are sorted by `entity_id`.
Once `FlushServerFrames()` runs after a due timer and the respawn rebuild is emitted, that actor disappears from this snapshot.
The exact-entity endpoint returns the same row shape for one pending respawn; invalid or missing entity IDs return `400`, and well-formed but absent/already-flushed respawn IDs return `404`.

### `GET /local/spawn-groups`

Returns the deterministic list of currently materialized spawn-backed runtime actors: static-actor snapshots whose `spawn_group_ref` is non-empty.
This endpoint is loopback-only, read-only, rejects non-`GET` methods with `405`, and is intended for QA/debugging of the authored `spawn_groups` contract frozen in `spec/protocol/content-spawn-groups-bootstrap.md`.

`GET /local/spawn-groups/{entity_id}` returns one exact spawn-backed actor row by runtime entity ID / client-visible static-actor `VID`.
Invalid path IDs return `400`; missing entities or ordinary non-spawn static actors return `404`.

Each row reuses the same static-actor snapshot shape exposed by `/local/static-actors`, including:

- `entity_id`
- `name`
- `map_index`
- `x`
- `y`
- `race_num`
- `combat_profile`
- `combat_level`
- `combat_rank`
- optional `retaliation_point_delta` when the resolved combat profile uses a non-default owner-retaliation amount; omitted means the bootstrap default `-1` point loss applies
- `spawn_group_ref`
- reward descriptor fields (`reward_experience`, `reward_gold`, `reward_drop_vnums`)
- optional `dead: true` while the materialized actor is in its server-owned dead interval before respawn

Plain `static_actors` without `spawn_group_ref` are intentionally omitted so operators can inspect authored attackable spawn presence separately from ordinary visible/service actors.
Rows are sorted by actor name with `entity_id` as the tie-breaker, matching the runtime static-actor snapshot ordering.

### `GET` / `POST /local/static-actors`, `GET /local/static-actors/{entity_id}`, and `PATCH` / `PUT` / `DELETE /local/static-actors/{entity_id}`

Use these endpoints to inspect and author bootstrap static actors. The collection `GET` returns every current static actor snapshot; the entity lookup `GET /local/static-actors/{entity_id}` returns one current snapshot by runtime entity / client-visible static-actor `VID`, returns `404` when that actor no longer exists, and uses the same loopback-only `403`, invalid-ID `400`, and wrong-method `405` guard shape as the other local static-actor endpoints.

Create/update bodies currently use:

- `name`
- `map_index`
- `x`
- `y`
- `race_num`
- optional paired `interaction_kind` and `interaction_ref`
- optional `combat_profile`

If one interaction field is present, the other must also be present.
`name` is trimmed before use and must remain non-empty, valid UTF-8, and embedded-NUL-free; raw create/update bodies containing invalid UTF-8 are rejected before the runtime mutation callback is invoked.
`combat_profile` follows the same bootstrap profile identifiers accepted by content bundles and spawn groups, letting local operator create/update calls seed practice-mob/training-dummy descriptors without importing a full bundle.
Returned static-actor snapshots expose the resolved `combat_level`, `combat_rank`, and any non-default `retaliation_point_delta` from that combat profile, so operator/debug consumers can inspect custom presentation and hostility metadata without re-resolving the profile registry.
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

`POST`, `PATCH`, and `PUT` bodies are bounded to 4 KiB; oversized authored interaction requests fail closed with `413` before reaching runtime mutation callbacks. Raw create/update bodies containing invalid UTF-8 are rejected before JSON decoding and before runtime mutation callbacks, matching the file-backed interaction store and content-bundle import boundary. `PATCH` and `PUT` are full-identity upserts, so body `kind` + `ref` must match the path exactly.
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
- `GET /local/content-bundle/summary` and dry-run `POST /local/content-bundle/summary` include compact `shop_routes` entries for each placed merchant actor, so local QA can inspect exact actor-to-catalog placement without fetching or applying the full bundle
- `POST /local/content-bundle/import-preview` compares a candidate replacement against the live exported bundle and returns no-mutation `current` / `candidate` summaries plus count/amount `deltas`, including per-interaction-kind reference deltas, per-definition `added` / `removed` / `changed` deltas with compact current/candidate previews, per-map before/after/signed deltas for changed authored map counts, and spawn reward EXP/gold totals

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

List currently materialized authored spawn-group actors:

```bash
curl http://127.0.0.1:6060/local/spawn-groups
```

This snapshot filters to attackable content materialized from `spawn_groups`; use `/local/static-actors` when you need the full visible static-actor set.
`/local/maps` also embeds the same spawn-backed subset per occupied map as `spawn_group_count` and `spawn_groups`, so map-local QA does not need to cross-filter `/local/static-actors` manually.

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
