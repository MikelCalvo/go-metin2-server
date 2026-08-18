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

Validates the durable bootstrap account snapshot store through the same strict loader used by runtime backup/restore primitives, without mutating any account files. This endpoint is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if any committed account snapshot is corrupt, is a symlink, is not valid UTF-8, has an invalid filename/login pairing, uses a non-canonical case-variant filename, duplicates a login by case variant, uses an empty, whitespace-padded, or embedded-NUL account login, or violates the deterministic account snapshot invariants. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely. Multiple all-zero character records are accepted as empty select-screen slots, but any zero-ID slot with leftover name, VID, location, stat, guild, gold, item, equipment, or quickslot state fails validation; non-zero character records must carry a non-empty, unpadded, NUL-free name, must not duplicate character IDs, case-folded names, or non-zero VIDs within the account snapshot, and any persisted guild name must also be NUL-free.

Successful responses are JSON summaries with:

- `account_count`
- `character_count` — persisted select-screen character records, including all-zero empty slot placeholders
- optional `empty_character_slot_count` when all-zero empty slot placeholders are present
- `logins` sorted in deterministic account-list order
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.account-*.json` temp files are present

Crash leftovers such as hidden regular `.account-*.json` temp files are not treated as committed snapshots, matching the committed-snapshot list/backup contract, but validation reports them as deterministic residue so an operator can see interrupted account writes before cleanup or recovery work. Crash-temp-shaped symlinks fail closed instead of being reported or followed, so validation/backup cannot traverse outside the configured account store through hidden temp residue.

### `POST /local/account-store/crash-temps/cleanup`

Removes same-directory `.account-*.json` crash-temp residue from the durable bootstrap account snapshot store after first validating the committed snapshot set through the same strict loader used by `/local/account-store/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if committed account state is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup account-store JSON summary (`account_count`, `character_count`, optional `empty_character_slot_count`, and deterministic `logins`). Because cleanup validates before removing anything, corrupt committed account snapshots or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.account-*.json` temp files are removed; committed account snapshots, backup manifests, symlinks, and unrelated hidden files are preserved. Use `/local/account-store/validate` first when you want a read-only residue report, then this endpoint when the operator has decided the interrupted temp writes are disposable.

### `POST /local/login-tickets/validate`

Validates the one-shot authd-to-gamed login-ticket handoff store without consuming or deleting any tickets. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if any committed ticket is corrupt, is a symlink, is not valid UTF-8, has unknown/trailing JSON, has an invalid or mismatched filename/login-key pairing, has an empty, whitespace-padded, or embedded-NUL login, has a zero login key, has a missing/zero `issued_at`, or violates the character/item/equipment/quickslot invariants shared with ticket load/consume. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely. Multiple all-zero character records are accepted as empty select-screen slots, but any zero-ID slot with leftover persisted character state fails validation before the handoff can be consumed; non-zero character records must carry a non-empty, unpadded, NUL-free name, must not duplicate character IDs, case-folded names, or non-zero VIDs within the pending ticket payload, and any persisted guild name must also be NUL-free.

Successful responses are JSON summaries with:

- `ticket_count`
- `character_count` — select-screen character records embedded in pending tickets, including all-zero empty slot placeholders
- optional `empty_character_slot_count` when all-zero empty slot placeholders are present in pending ticket payloads
- `logins` sorted in deterministic ticket-list order
- `login_keys` in the same order as `logins`
- optional `oldest_issued_at` / `newest_issued_at` bounds when at least one committed ticket is present
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.ticket-*.json` temp files are present

Crash leftovers such as hidden regular `.ticket-*.json` temp files are not treated as pending handoff tickets, but validation reports them as deterministic residue. Crash-temp-shaped symlinks fail closed instead of being reported or followed, so validation cannot traverse outside the configured login-ticket store through hidden temp residue. The issued-at bounds are calculated from committed tickets only, so operators can quickly see the age span of pending one-shot handoffs before choosing a cutoff for stale-ticket preview or cleanup. Use this endpoint to inspect both pending committed handoff state and interrupted ticket writes before debugging authd/gamed login-key issues; it is not a replay, consume, restore, or remote admin API.

### `POST /local/login-tickets/crash-temps/cleanup`

Removes same-directory `.ticket-*.json` crash-temp residue from the one-shot login-ticket handoff store after first validating the committed ticket set through the same strict loader used by `/local/login-tickets/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if committed ticket state is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup login-ticket JSON summary (`ticket_count`, `character_count`, optional `empty_character_slot_count`, deterministic `logins`, matching `login_keys`, and issued-at bounds when committed tickets remain). Because cleanup validates before removing anything, corrupt committed tickets or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.ticket-*.json` temp files are removed; committed handoff tickets, symlinks, and unrelated hidden files are preserved. Use `/local/login-tickets/validate` first when you want a read-only residue report, then this endpoint when the operator has decided interrupted temp ticket writes are disposable.

### `POST /local/login-tickets/issued-before/preview`

Dry-runs the stale login-ticket cutoff logic without consuming or deleting any one-shot handoff tickets. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed ticket set fails validation.

Request body JSON fields:

- `issued_before` — RFC3339/RFC3339Nano timestamp cutoff; tickets with `issued_at < issued_before` are reported as stale

The preview path validates the whole committed ticket set through the same strict listing boundary used by `/local/login-tickets/validate`, including the requirement that every committed ticket has a non-zero `issued_at`, reports `stale_count`, deterministic `stale_logins` / `stale_login_keys`, and embeds the unchanged `current` login-ticket summary with issued-at bounds plus embedded character and empty-slot counts when tickets are present. Hidden `.ticket-*.json` crash-temp files are visible in the `current` summary but are not stale cleanup candidates. Use this endpoint before `/local/login-tickets/issued-before/cleanup` when an operator wants a no-mutation audit of abandoned authd-to-gamed handoff keys.

### `POST /local/login-tickets/issued-before/cleanup`

Removes committed one-shot login-ticket files whose `issued_at` timestamp is strictly older than an operator-supplied cutoff. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed ticket set fails validation or if deletion/directory sync fails.

Request body JSON fields:

- `issued_before` — RFC3339/RFC3339Nano timestamp cutoff; only tickets with `issued_at < issued_before` are removed

The cleanup path validates the whole committed ticket set through the same strict listing boundary used by `/local/login-tickets/validate` before deleting anything, so corrupt committed tickets or tickets with missing/zero `issued_at` fail closed and leave all pending handoff files available for inspection. It also checks same-directory crash-temp residue before deletion; hidden regular `.ticket-*.json` crash-temp files are reported in the returned `remaining` summary but are not removed by this endpoint, while crash-temp-shaped symlinks fail closed before any stale ticket is deleted. Use `/local/login-tickets/crash-temps/cleanup` for interrupted temp writes and `/local/login-tickets/issued-before/preview` for a no-mutation stale-ticket audit. Successful responses include the cutoff, `removed_count`, deterministic `removed_logins` / `removed_login_keys`, and a `remaining` login-ticket summary including issued-at bounds plus embedded character and empty-slot counts for the surviving handoff set, so operators can verify pending ticket count, select-screen payload size, and age span after pruning stale tickets. This is a bounded local recovery primitive for abandoned authd-to-gamed handoff keys, not a remote admin API or a normal ticket-consume path.

### `POST /local/login-tickets/backup`

Copies the one-shot authd-to-gamed login-ticket handoff store into an operator-supplied empty destination directory and returns the validation summary of the copied ticket set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only destination paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source store is invalid, the destination is non-empty, the destination is equal to or nested under the active login-ticket store, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

The backup path uses the same committed-ticket list/validate contract as `/local/login-tickets/validate`: hidden regular crash-temp files are ignored as backup payload, crash-temp-shaped symlinks fail closed before the requested destination is created, corrupt committed tickets fail closed, and successful responses contain `ticket_count`, `character_count`, optional `empty_character_slot_count`, deterministic `logins` / `login_keys`, and issued-at bounds for the backup that was just written. A successful backup also writes `login-ticket-backup-manifest.json` with the backup format marker, deterministic ticket summary, per-ticket filenames, byte sizes, and SHA-256 checksums. When the active login-ticket store already contains restored backup metadata, backup validates that active manifest before creating the destination and fails closed on stale or malformed manifest state.

### `POST /local/login-tickets/backup/validate`

Dry-runs an operator-supplied login-ticket backup source through the same strict restore-source loader and backup-manifest checks used by `/local/login-tickets/restore`, but does not create or mutate the active login-ticket store directory. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only source paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `login-ticket-backup-manifest.json` plus committed ticket snapshots that pass the strict login-ticket loader

Successful responses are the deterministic backup summary (`ticket_count`, `character_count`, optional `empty_character_slot_count`, sorted `logins` / `login_keys`, issued-at bounds, plus optional `crash_temp_count` / `crash_temp_files`) that would be restored. Backup directories are required to be manifest-closed and symlink-free: every non-temp entry must be either `login-ticket-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.ticket-*.json` crash leftovers remain visible in the preflight summary but are not backup payload.

### `POST /local/login-tickets/restore`

Restores the one-shot login-ticket handoff store from an operator-supplied source backup directory into the active store directory and returns the validation summary of the restored ticket set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON or blank/whitespace-only source paths with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active login-ticket store directory is non-empty, the destination is equal to or nested under the backup source, live selected-character sessions are present, or restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `login-ticket-backup-manifest.json` plus committed ticket snapshots that pass the strict login-ticket loader

The restore path is intentionally a replacement primitive, not an online merge or live gameplay reload API. It refuses to write into a non-empty active login-ticket directory, refuses destinations that are equal to or nested under the backup source (including symlink-resolved paths), refuses to run while connected selected-character sessions are live, validates the backup manifest and symlink-free backup entries before creating target files, writes a fresh `login-ticket-backup-manifest.json` alongside the restored tickets, and leaves corrupt or incomplete restore attempts rolled back to an empty destination when possible. Subsequent ticket issue/consume/stale cleanup removes that restored manifest so backup metadata does not survive after the live handoff set changes.

### `POST /local/item-templates/validate`

Validates the authored bootstrap item-template snapshot store without mutating item-template state. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed item-template snapshot is malformed, is a symlink, has unknown/trailing JSON, duplicates a vnum, or violates template policy such as invalid max counts, equipment slots, display metadata, use effects, or equip effects. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `template_count`
- `vnums` sorted in deterministic template order
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.item-templates-*.json` temp files are present

A missing committed `item-templates.json` is reported as an empty authored-template store, matching the runtime fallback to built-in bootstrap item templates. Hidden regular crash leftovers are reported for operator visibility but are not treated as committed templates. Crash-temp-shaped symlinks fail closed instead of being reported or followed, so validation/backup cannot traverse outside the configured item-template store through hidden temp residue. Use this endpoint before importing content bundles, debugging merchant catalog/template mismatches, or planning item-template migration work; it is not a gameplay API or a remote admin API.

### `POST /local/item-templates/crash-temps/cleanup`

Removes same-directory `.item-templates-*.json` crash-temp residue from the authored bootstrap item-template snapshot store after first validating the committed `item-templates.json` snapshot through the same strict loader used by `/local/item-templates/validate`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup item-template JSON summary (`template_count` and deterministic `vnums`). Because cleanup validates before removing anything, corrupt committed item-template snapshots or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.item-templates-*.json` temp files are removed; committed snapshots, symlinks, and unrelated hidden files are preserved. Use `/local/item-templates/validate` first when you want a read-only residue report, then this endpoint when the operator has decided interrupted item-template temp writes are disposable.

### `POST /local/static-actor-store/validate`

Validates the authored bootstrap static-actor snapshot store without mutating actor content. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed `static-actors.json` snapshot is a symlink, is malformed, has unknown/trailing JSON, duplicates actor IDs or spawn-group refs, or violates actor policy such as missing names, zero map indexes, invalid race numbers, invalid interaction refs, invalid combat profiles, or invalid reward descriptors. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `actor_count`
- optional `interactable_actor_count`
- optional `spawn_group_count`
- deterministic `actor_ids`
- deterministic `actor_names`
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.static-actors-*.json` temp files are present

A missing committed `static-actors.json` is reported as an empty authored actor store. Hidden regular crash leftovers are reported for operator visibility but are not treated as committed static actors. Crash-temp-shaped symlinks fail closed instead of being reported or followed, so validation cannot traverse outside the configured static-actor store through hidden temp residue. Use this endpoint before content import/restore work or before deleting static-actor crash-temp residue; it is not a gameplay API or a remote admin API.

### `POST /local/static-actor-store/crash-temps/cleanup`

Removes same-directory `.static-actors-*.json` crash-temp residue from the authored static-actor snapshot store after first validating the committed `static-actors.json` snapshot through the same strict loader used by `/local/static-actor-store/validate` and `/local/persistence/status`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup static-actor JSON summary (`actor_count`, deterministic `actor_ids` / `actor_names`, and authored-content counters). Because cleanup validates before removing anything, corrupt committed static-actor snapshots or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.static-actors-*.json` temp files are removed; committed snapshots, symlinks, and unrelated hidden files are preserved.

### `POST /local/interaction-store/validate`

Validates the authored bootstrap interaction-definition snapshot store without mutating interaction content. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed `interaction-definitions.json` snapshot is a symlink, is malformed, has unknown/trailing JSON, duplicates a `kind:ref`, or violates definition policy for supported `info`, `talk`, `quest_flag`, `shop_preview`, or `warp` definitions. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `definition_count`
- deterministic `definition_keys` (`kind:ref`)
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.interaction-definitions-*.json` temp files are present

A missing committed `interaction-definitions.json` is reported as an empty authored interaction store. Hidden regular crash leftovers are reported for operator visibility but are not treated as committed definitions. Crash-temp-shaped symlinks fail closed instead of being reported or followed, so validation cannot traverse outside the configured interaction-definition store through hidden temp residue. Use this endpoint before content import/restore work or before deleting interaction crash-temp residue; it is not a gameplay API or a remote admin API.

### `POST /local/interaction-store/crash-temps/cleanup`

Removes same-directory `.interaction-definitions-*.json` crash-temp residue from the authored interaction-definition snapshot store after first validating the committed `interaction-definitions.json` snapshot through the same strict loader used by `/local/interaction-store/validate` and `/local/persistence/status`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if the committed snapshot is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup interaction JSON summary (`definition_count` and deterministic `definition_keys`). Because cleanup validates before removing anything, corrupt committed interaction-definition snapshots or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.interaction-definitions-*.json` temp files are removed; committed snapshots, symlinks, and unrelated hidden files are preserved.

### `POST /local/quest-state/validate`

Validates the standalone bootstrap quest-state snapshot store without mutating quest flags. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-empty request bodies with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the committed `quest-state.json` snapshot is a symlink, is malformed, has unknown/trailing JSON, duplicates a `(character, quest_ref, name)` row, or violates the identity/value rules frozen in `spec/protocol/quest-state-bootstrap.md`. Empty or whitespace-only bodies remain accepted so local scripts can issue a plain `POST` safely.

Successful responses are JSON summaries with:

- `flag_count`
- deterministic `characters`
- deterministic `quest_refs`
- deterministic `flag_keys` (`character:quest_ref:name`)
- optional `crash_temp_count` and `crash_temp_files` when same-directory `.quest-state-*.json` temp files are present

A missing committed `quest-state.json` is reported as an empty valid store. Hidden regular crash leftovers are reported for operator visibility but are not treated as committed quest flags. Crash-temp-shaped symlinks fail closed instead of being reported or followed. This is a local persistence preflight for the quest-state primitive, not a client-visible quest runtime or remote admin API.

### `POST /local/quest-state/transition-preview`

Dry-runs one standalone bootstrap quest-state compare-and-set transition against the configured `gamed` quest-state store without writing the committed snapshot. This endpoint is loopback-only, rejects non-`POST` methods with `405`, rejects invalid JSON, unknown fields, trailing JSON, invalid UTF-8, JSON `null`, or oversized bodies before evaluating the transition, and returns `409` for store/runtime failures that prevent preview evaluation.

Request body:

```json
{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}
```

Successful HTTP responses use the same JSON transition-attempt shape as the mutating endpoint: `transition`, `result`, and the hypothetical post-attempt `summary`. Compare-and-set misses such as `current_value_mismatch` still return `200 OK` with `applied = false` and the current committed summary. A missing committed `quest-state.json` is previewed as an empty snapshot, but the preview endpoint does not create the store file. Use this before `/local/quest-state/transition` when local QA wants to inspect the exact flag-count/key impact of an authored transition without mutating quest state.

### `POST /local/quest-state/transition`

Applies one standalone bootstrap quest-state compare-and-set transition through the configured `gamed` quest-state store. This endpoint is loopback-only, rejects non-`POST` methods with `405`, rejects invalid JSON, unknown fields, trailing JSON, invalid UTF-8, JSON `null`, or oversized bodies before mutating state, and returns `409` for store/runtime failures that prevent transition evaluation or persistence.

Request body:

```json
{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}
```

Successful HTTP responses are JSON transition-attempt results with `transition`, `result`, and post-attempt `summary`. Compare-and-set misses such as `current_value_mismatch` still return `200 OK` with `applied = false`; they are authored quest-state outcomes rather than transport errors. The endpoint is a local bootstrap/operator harness for validating quest-state progression and recovery. It is not a client quest packet, NPC dialog hook, reward hook, or remote admin API.

### `GET /local/quest-state`

Returns a read-only overview of the committed standalone bootstrap quest-state store. This endpoint is registered only on `gamed`, is loopback-only, accepts only `GET`, treats a missing committed `quest-state.json` as an empty overview, and returns `409` when the committed snapshot cannot be loaded or validated.

Successful responses include `flag_count`, `character_count`, `quest_count`, sorted `quest_refs`, deterministic per-character flag snapshots, and deterministic per-quest grouped snapshots. Use it when local QA needs the whole committed quest-state projection without fetching `/local/content-bundle/summary` or calling each character/quest/flag reader separately. It is not a client quest protocol endpoint and does not mutate quest state.

### `GET /local/quest-state/characters/{character}`

Returns one read-only exact-character quest-state snapshot from the configured `gamed` quest-state store. The endpoint is loopback-only, rejects non-`GET` methods with `405`, rejects blank or slash-containing character path values with `400`, returns `404` when no persisted non-zero flags exist for that character, and returns `409` if the committed quest-state snapshot cannot be loaded or validated.

Successful responses use:

```json
{
  "character": "QuestHero",
  "flags": [
    {"quest_ref": "quest:first_steps", "name": "step", "value": 2}
  ]
}
```

The flag list follows the store's deterministic order and the endpoint does not mutate quest state, infer account rosters, or expose a client-visible quest runtime.

### `GET /local/quest-state/quests/{quest_ref}`

Returns one read-only exact-quest quest-state snapshot from the configured `gamed` quest-state store. The endpoint is loopback-only, rejects non-`GET` methods with `405`, rejects blank, slash-containing, or non-`quest:<name>` lower-snake quest refs with `400`, returns `404` when no persisted non-zero flags exist for that quest, and returns `409` if the committed quest-state snapshot cannot be loaded or validated.

Successful responses group deterministic flag rows by character:

```json
{
  "quest_ref": "quest:first_steps",
  "flag_count": 2,
  "characters": [
    {"character":"AnotherHero","flags":[{"quest_ref":"quest:first_steps","name":"met_guard","value":1}]},
    {"character":"QuestHero","flags":[{"quest_ref":"quest:first_steps","name":"step","value":2}]}
  ]
}
```

The grouping is an operator inspection shape only. It does not define quest objectives, availability, NPC dialog state, rewards, or client quest packets.

### `GET /local/quest-state/flags/{character}/{quest_ref}/{flag}`

Returns one read-only exact quest-state flag row from the configured `gamed` quest-state store. The endpoint is loopback-only, rejects non-`GET` methods with `405`, rejects blank or slash-containing path values with `400`, validates the same bootstrap character, `quest:<name>`, and lower-snake flag-name identities as the store primitive, returns `404` when the exact non-zero flag is absent, and returns `409` if the committed quest-state snapshot cannot be loaded or validated.

Successful responses use the canonical persisted row shape:

```json
{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2}
```

This is a focused read-only QA aid for checking one flag after content-bundle import or local transition testing. It does not mutate quest state or make static actors execute quest transitions.

### `POST /local/quest-state/crash-temps/cleanup`

Removes same-directory `.quest-state-*.json` crash-temp residue from the bootstrap quest-state snapshot store after first validating the committed `quest-state.json` snapshot through the same strict loader used by `/local/quest-state/validate` and `/local/persistence/status`. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, and returns `409` if committed quest-state data is corrupt, if a temp file cannot be removed, or if the final directory sync fails.

The endpoint does not accept a request body: empty or whitespace-only bodies are accepted, non-empty bodies are rejected with `400`, and bodies over 4 KiB are rejected with `413`. Successful responses are the post-cleanup quest-state JSON summary (`flag_count`, deterministic `characters`, deterministic `quest_refs`, and deterministic `flag_keys`). Because cleanup validates before removing anything, corrupt committed quest-state snapshots or crash-temp-shaped symlinks leave crash-temp files in place for manual recovery. Only hidden regular `.quest-state-*.json` temp files are removed; the committed snapshot, symlinks, and unrelated hidden files are preserved.

### `POST /local/quest-state/backup`

Copies the standalone bootstrap quest-state snapshot into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source snapshot is invalid, the destination is non-empty, the destination is equal to or nested under the active quest-state store directory, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

A successful backup writes `quest-state-backup-manifest.json` with the backup format marker, deterministic quest-state summary, copied snapshot filename, byte size, and SHA-256 checksum. Missing committed `quest-state.json` snapshots are backed up as an empty store with no synthetic snapshot file. Hidden regular `.quest-state-*.json` crash leftovers are ignored as backup payload, while crash-temp-shaped symlinks fail closed before the requested destination is created. Before creating the requested destination, backup also validates any active restored quest-state backup manifest against the current committed snapshot bytes; stale, malformed, or symlinked active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active quest-state store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest as an ordinary non-symlink file against the current committed snapshot bytes and fail closed on malformed manifests, dangling manifest symlinks, summary drift, size drift, checksum drift, filename drift, or a manifest that omits an existing committed snapshot. A later successful quest-state save or applied transition removes the restored manifest, so changed quest-flag state stops claiming to be the exact restored backup. If snapshot copying, manifest writing, or final directory sync fails after files were committed, backup removes the snapshot file and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/quest-state/backup/validate`

Dry-runs an operator-supplied quest-state backup source through the same strict manifest and snapshot checks used by backup, but does not create or mutate the active quest-state store. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `quest-state-backup-manifest.json` plus the committed quest-state snapshot when the manifest lists one

Successful responses are the deterministic quest-state backup summary (`flag_count`, sorted `characters` / `quest_refs` / `flag_keys`, plus optional `crash_temp_count` / `crash_temp_files`) that was validated. Backup directories are required to be manifest-closed and symlink-free: every non-temp entry must be either `quest-state-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.quest-state-*.json` crash leftovers remain visible in the preflight summary but are not backup payload. The backup manifest itself must also be an ordinary file with valid UTF-8 before JSON decoding. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. This is a local recovery/audit primitive, not an online restore or live quest reload API.

### `POST /local/quest-state/restore`

Restores the standalone bootstrap quest-state snapshot store from an operator-supplied source backup directory into the active quest-state store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active quest-state store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `quest-state-backup-manifest.json` plus `quest-state.json` when the manifest lists a committed snapshot

The restore path is intentionally a replacement primitive, not an online merge or live gameplay reload API. It refuses to write into a non-empty active quest-state directory, refuses destinations that are equal to or nested under the backup source (including symlink-resolved paths), refuses to run while connected selected-character sessions are live, validates the backup manifest and symlink-free backup entries before creating target files, and writes a fresh `quest-state-backup-manifest.json` alongside the restored snapshot so the replacement store can be preflighted or backed up again. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. The next successful quest-state save or applied transition removes that restored manifest so a later mutation cannot leave the active store looking like the exact validated backup source. Missing committed snapshots restore as an empty store.

### `POST /local/static-actors/backup`

Copies the authored bootstrap static-actor snapshot into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source snapshot is invalid, the destination is non-empty, the destination is equal to or nested under the active static-actor store directory, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

A successful backup writes `static-actor-backup-manifest.json` with the backup format marker, deterministic static-actor summary, copied snapshot filename, byte size, and SHA-256 checksum. Missing committed `static-actors.json` snapshots are backed up as an empty store with no synthetic snapshot file. Hidden regular `.static-actors-*.json` crash leftovers are ignored as backup payload, while crash-temp-shaped symlinks fail closed before the requested destination is created. Before creating the requested destination, backup also validates any active restored static-actor backup manifest against the current committed snapshot bytes; stale, malformed, or symlinked active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active static-actor store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest as an ordinary non-symlink file against the current committed snapshot bytes and fail closed on malformed manifests, dangling manifest symlinks, summary drift, size drift, checksum drift, filename drift, or a manifest that omits an existing committed snapshot. A later successful static-actor save removes the restored manifest, so changed authored NPC/spawn content stops claiming to be the exact restored backup. If snapshot copying, manifest writing, or final directory sync fails after files were committed, backup removes the snapshot file and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/static-actors/backup/validate`

Dry-runs an operator-supplied static-actor backup source through the same strict manifest and snapshot checks used by backup, but does not create or mutate the active static-actor store. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `static-actor-backup-manifest.json` plus the committed static-actor snapshot when the manifest lists one

Successful responses are the deterministic static-actor backup summary (`actor_count`, deterministic `actor_ids` / `actor_names`, optional interactable/spawn counts, plus optional `crash_temp_count` / `crash_temp_files`) that was validated. Backup directories are required to be manifest-closed and symlink-free: every non-temp entry must be either `static-actor-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.static-actors-*.json` crash leftovers remain visible in the preflight summary but are not backup payload. The backup manifest itself must also be an ordinary file with valid UTF-8 before JSON decoding. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. This is a local recovery/audit primitive, not an online restore or live content reload API.

### `POST /local/static-actors/restore`

Restores the authored bootstrap static-actor snapshot store from an operator-supplied source backup directory into the active static-actor store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active static-actor store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `static-actor-backup-manifest.json` plus `static-actors.json` when the manifest lists a committed snapshot

The restore path is intentionally a replacement primitive, not an online merge or live gameplay reload API. It refuses to write into a non-empty active static-actor directory, refuses destinations that are equal to or nested under the backup source (including symlink-resolved paths), refuses to run while connected selected-character sessions are live, validates the backup manifest and symlink-free backup entries before creating target files, reloads the restored snapshot into the live shared-world static-actor set after sessions are drained, and writes a fresh `static-actor-backup-manifest.json` alongside the restored snapshot so the replacement store can be preflighted or backed up again. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. The next successful static-actor save removes that restored manifest so a later mutation cannot leave the active store looking like the exact validated backup source. Missing committed snapshots restore as an empty store.

### `POST /local/interaction-store/backup`

Copies the authored bootstrap interaction-definition snapshot into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source snapshot is invalid, the destination is non-empty, the destination is equal to or nested under the active interaction store directory, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

A successful backup writes `interaction-backup-manifest.json` with the backup format marker, deterministic interaction summary, copied snapshot filename, byte size, and SHA-256 checksum. Missing committed `interaction-definitions.json` snapshots are backed up as an empty store with no synthetic snapshot file. Hidden regular `.interaction-definitions-*.json` crash leftovers are ignored as backup payload, while crash-temp-shaped symlinks fail closed before the requested destination is created. Before creating the requested destination, backup also validates any active restored interaction backup manifest against the current committed snapshot bytes; stale, malformed, or symlinked active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active interaction store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest as an ordinary non-symlink file against the current committed snapshot bytes and fail closed on malformed manifests, dangling manifest symlinks, summary drift, size drift, checksum drift, filename drift, or a manifest that omits an existing committed snapshot. A later successful interaction-definition save removes the restored manifest, so changed authored definitions stop claiming to be the exact restored backup. If snapshot copying, manifest writing, or final directory sync fails after files were committed, backup removes the snapshot file and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

### `POST /local/interaction-store/backup/validate`

Dry-runs an operator-supplied interaction backup source through the same strict manifest and snapshot checks used by backup, but does not create or mutate the active interaction store. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing, lacks the required manifest, is corrupt, or has an invalid manifest.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `interaction-backup-manifest.json` plus the committed interaction snapshot when the manifest lists one

Successful responses are the deterministic interaction backup summary (`definition_count`, sorted `definition_keys`, plus optional `crash_temp_count` / `crash_temp_files`) that was validated. Backup directories are required to be manifest-closed and symlink-free: every non-temp entry must be either `interaction-backup-manifest.json` or a snapshot file named in that manifest, while hidden `.interaction-definitions-*.json` crash leftovers remain visible in the preflight summary but are not backup payload. The backup manifest itself must also be an ordinary file with valid UTF-8 before JSON decoding. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. This is a local recovery/audit primitive, not an online restore or live interaction reload API.

### `POST /local/interaction-store/restore`

Restores the authored bootstrap interaction-definition snapshot store from an operator-supplied source backup directory into the active interaction store directory and returns the validation summary of the restored snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source backup is missing or invalid, the active interaction store directory is non-empty, or the restore cannot be completed.

Request body JSON fields:

- `src_dir` — source backup directory; it must be non-empty after trimming and contain `interaction-backup-manifest.json` plus `interaction-definitions.json` when the manifest lists a committed snapshot

The restore path is intentionally a replacement primitive, not an online merge or live gameplay reload API. It refuses to write into a non-empty active interaction directory, refuses destinations that are equal to or nested under the backup source (including symlink-resolved paths), refuses to run while connected selected-character sessions are live, validates the backup manifest and symlink-free backup entries before creating target files, writes a fresh `interaction-backup-manifest.json` alongside the restored snapshot so the replacement store can be preflighted or backed up again, and reloads the restored definitions into the live `gamed` interaction index when sessions are drained. Symlinked manifests, manifested snapshots, or crash-temp-shaped entries fail closed with `409`. The next successful interaction-definition save removes that restored manifest so a later mutation cannot leave the active store looking like the exact validated backup source. Missing committed snapshots restore as an empty store.


### `POST /local/item-templates/backup`

Copies the authored bootstrap item-template snapshot into an operator-supplied empty destination directory and returns the validation summary of the copied snapshot set. This endpoint is available only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects malformed JSON with `400`, rejects request bodies over 4 KiB with `413`, and returns `409` if the source snapshot is invalid, the destination is non-empty, the destination is equal to or nested under the active item-template store directory, or the backup cannot be completed.

Request body JSON fields:

- `dst_dir` — destination directory for the backup; it must be non-empty after trimming and should point to a local path prepared by the operator

A successful backup writes `item-template-backup-manifest.json` with the backup format marker, deterministic item-template summary, copied snapshot filename, byte size, and SHA-256 checksum. Missing committed `item-templates.json` snapshots are backed up as an empty authored-template store with no synthetic snapshot file, preserving the runtime fallback to built-in bootstrap templates. Hidden regular `.item-templates-*.json` crash leftovers are ignored as backup payload, while crash-temp-shaped symlinks fail closed before the requested destination is created. Before creating the requested destination, backup also validates any active restored item-template backup manifest against the current committed snapshot bytes; stale, malformed, or symlinked active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active item-template store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest as an ordinary non-symlink file against the current committed snapshot bytes and fail closed on malformed manifests, dangling manifest symlinks, summary drift, size drift, checksum drift, filename drift, or a manifest that omits an existing committed snapshot. A later successful item-template save removes the restored manifest, so changed authored-template state stops claiming to be the exact restored backup. If snapshot copying, manifest writing, or final directory sync fails after files were committed, backup removes the snapshot file and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

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

The backup path uses the same committed-snapshot list/validate contract as `/local/account-store/validate`: hidden regular crash-temp files are ignored as backup payload, crash-temp-shaped symlinks fail closed before the requested destination is created, corrupt or non-UTF-8 committed snapshots fail closed, and successful responses contain `account_count`, `character_count`, optional `empty_character_slot_count`, and deterministic `logins` for the backup that was just written. A successful backup also writes `account-backup-manifest.json` with the backup format marker, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums. Before creating the destination, backup also validates any active restored backup manifest against the current committed account files; stale, malformed, or symlinked active manifest state fails closed with `409` and leaves the requested destination uncreated. If an active account store still has a restored backup manifest, normal validation and `/local/persistence/status` verify that manifest as an ordinary non-symlink file against the current committed account files and fail closed on malformed manifests, dangling manifest symlinks, summary drift, size drift, checksum drift, or login/filename drift. A later successful account save removes the restored manifest, so changed live account state stops claiming to be the exact restored backup. The destination must be empty and must not be equal to or nested under the active account-store directory, including through destination symlinks, so this endpoint cannot silently merge unrelated operator files with a runtime backup or recursively copy its own in-progress output. If account copying, manifest writing, or the final destination-directory sync fails after files were committed, backup removes the account files and manifest it already wrote and syncs the destination again so operators are not left with a partial backup that looks usable.

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

### `GET /local/db/migrations/catalog`

Returns a loopback-only, read-only summary of the validated embedded `db/migrations` catalog without consulting any configured database. The response uses format marker `go-metin2-migration-catalog-summary-v1`, reports `latest_version`, and lists deterministic migration rows with `version`, `name`, `up_path`, `down_path`, `up_sha256`, and `down_sha256`. It never includes executable SQL text, DSNs, applied-ledger rows, runtime store data, or apply/rollback output. Use it to compare a shipped daemon's project-owned migration inventory with offline runbooks before planning against a live or copied ledger.

### `GET /local/db/migrations/status`

Returns a loopback-only, read-only migration dry-run plan from the validated `db/migrations` catalog to the latest embedded migration. When DB preflight config is disabled, `gamed` plans against an empty applied ledger. When both `METIN2_*_DB_DRIVER` and `METIN2_*_DB_DSN` are configured, it reads only `version`, `name`, and `up_sha256` from `schema_migrations` through `database/sql`, validates those rows against the embedded catalog, and returns `409` on query, ledger, checksum, or catalog drift. The response is metadata-only (`current_version`, `latest_version`, `up_to_date`, and pending step `version` / `name` / `direction` / `path` / `sha256`) and never includes executable SQL.

### `GET /local/db/migrations/plan?target_version=N`

Returns the same metadata-only migration dry-run shape for an explicit target version. Target `0` previews a complete rollback using down migrations in reverse applied-version order; intermediate targets preview only the up/down steps required to move from the current ledger version to `N`; targets outside the embedded catalog range fail closed with `409`. The endpoint rejects missing, repeated, or non-integer `target_version` values with `400`, rejects non-`GET` methods with `405`, and is registered only on `gamed`. This is an operator preflight surface, not an apply/rollback command and not proof that runtime stores are DB-backed.

### `GET /local/db/migrations/ledger-snapshot`

Exports the running daemon's migration ledger preflight input as a strict metadata-only snapshot. This endpoint is registered only on `gamed`, is loopback-only, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the configured database driver, ledger query, row scan, or snapshot validation fails. With DB preflight disabled, it returns an explicit empty snapshot (`format = go-metin2-schema-migrations-ledger-v1`, `entries = []`). With both DB driver and DSN configured, it opens that `database/sql` target only long enough to query `schema_migrations` metadata (`version`, `name`, `up_sha256`), validates and sorts the rows, closes the connection, and returns the snapshot.

The response deliberately contains only the offline ledger snapshot fields accepted by `POST /local/db/migrations/plan-from-ledger-snapshot?target_version=N`; it never includes executable SQL, DSNs, apply/rollback output, or runtime store data. Use it to copy ledger metadata from a live or staged daemon into an offline planning runbook without granting the planner direct DB access.

### `POST /local/db/migrations/plan-from-ledger-snapshot?target_version=N`

Returns the same metadata-only migration dry-run shape from an operator-supplied offline ledger snapshot instead of the daemon's configured DB preflight target. This endpoint is registered only on `gamed`, is loopback-only, rejects non-`POST` methods with `405`, rejects non-loopback callers with `403`, rejects missing/repeated/non-integer `target_version` with `400`, rejects invalid snapshot JSON with `400`, rejects bodies over 64 KiB with `413`, and returns `409` if the decoded snapshot does not validate against the embedded catalog or target boundary.

Request body JSON uses format marker `go-metin2-schema-migrations-ledger-v1` and contains only `entries` rows with `version`, `name`, and `up_sha256`. The decoder rejects invalid UTF-8, unknown fields, trailing JSON, missing/null entries, malformed names, malformed checksums, zero/negative versions, and duplicate versions. An empty applied ledger is encoded explicitly as `"entries": []`. The response never includes executable SQL and the endpoint does not open a configured DB, execute SQL, apply migrations, roll migrations back, or mutate `schema_migrations`; it exists so operators and future CLI/runbook tooling can plan from copied ledger metadata safely. The companion `GET /local/db/migrations/ledger-snapshot` endpoint can produce this strict body shape from the daemon's current DB-preflight target.

`metin2-migrate plan-artifact-status --plan-artifact <path>` is the read-only retained-plan inspection helper for the CLI-only apply path. It validates an existing non-symlink regular `go-metin2-migration-plan-artifact-v1` JSON file with a 128 KiB cap and returns `go-metin2-migration-plan-artifact-status-v1` metadata (`present: false` when absent, `present: true` plus the strict artifact object when valid) without deleting the artifact, opening the DB target, reserving lock/audit files, applying migrations, rolling migrations back, or exposing DSNs / executable SQL. It also checks the embedded plan checksum and contiguous pending-step sequence, but it is still artifact validation only; it does not prove that a database currently matches the artifact's input ledger.

`metin2-migrate apply-preflight` is the read-only companion for the final apply runbook. It accepts the same offline ledger snapshot and target as `apply`, validates a reviewed `--plan-sha256` or `--plan-artifact` when supplied, enforces the same rollback acknowledgement rule (`--allow-rollback` plus plan confirmation for down plans), and emits `go-metin2-migration-apply-preflight-v1` metadata with both `ledger_snapshot_sha256` and `plan_sha256` for runbook/audit correlation without opening a database, accepting a DSN, reserving a lock file, creating an audit file, or exposing executable SQL. The mutating CLI can now require the retained preflight with `apply --apply-preflight <path>`; that re-validates the preflight artifact and requires its ledger checksum, resolved target, plan checksum, and embedded metadata-only plan to match the supplied `apply` ledger snapshot and target before any DB open.

`metin2-migrate apply-lock-status --lock-file <path>` is the read-only stale-lock triage helper for the CLI-only apply path. It validates an existing non-symlink regular `go-metin2-migration-apply-lock-v1` JSON file with a 16 KiB cap and returns `go-metin2-migration-apply-lock-status-v1` metadata (`present: false` when absent, `present: true` plus the strict lock object when valid) without deleting the lock, opening the DB target, applying migrations, or exposing DSNs / executable SQL. Treat it as inspection only; it does not define stale-lock removal policy.

`metin2-migrate apply-audit-status --audit-file <path>` is the read-only retained-audit inspection helper for the CLI-only apply path. It validates an existing non-symlink regular `go-metin2-migration-apply-audit-v1` JSON file with a 128 KiB cap and returns `go-metin2-migration-apply-audit-status-v1` metadata (`present: false` when absent, `present: true` plus the strict audit object when valid) without deleting the audit, opening the DB target, applying migrations, rolling migrations back, or exposing DSNs / executable SQL. It also checks the audit's result sequence and plan checksum consistency, but it is still artifact validation only; it does not prove the target database is currently at that boundary.

There is intentionally no `/local/db/migrations/apply` endpoint. Shipped daemons still expose read-only catalog, status, target-plan, and ledger-snapshot surfaces only. Mutating migration execution is available only outside daemon ops surfaces: through the `db/migrations` package primitive and the CLI-only `metin2-migrate apply` command. Both paths validate conservative SQL statement boundaries, execute each terminated migration statement individually inside the caller-supplied transaction, and re-verify transaction-local `schema_migrations` rows around non-zero migration targets so stale caller preflight input fails closed before commit. `metin2-migrate ledger-snapshot` can export the same strict metadata-only snapshot shape from an operator-supplied `database/sql` target without using the daemon ops endpoint; it performs only the ledger metadata query and redacts the supplied DSN from errors. `metin2-migrate apply` additionally requires an offline ledger snapshot, operator-supplied driver/DSN, and target version, emits metadata-only `ApplyResult` JSON, and redacts the supplied DSN from apply errors. Any plan containing down/rollback steps is rejected before opening the database unless the operator passes both `--allow-rollback` and an exact plan confirmation (`--plan-sha256 <hex>`, `--plan-artifact <path>`, or `--apply-preflight <path>`), so rollback requires an explicit direction acknowledgement plus a reviewed-plan guard beyond the numeric target, lock, or audit options. Operators may pass `--lock-file <path>` to create an exclusive local filesystem lock after plan/artifact validation and before opening the DB target; an existing lock fails closed before mutation, and a reserved lock is removed on success or failure. While reserved, the lock file contains metadata-only `go-metin2-migration-apply-lock-v1` JSON with local PID, target, ledger-snapshot checksum, plan checksum, and optional confirmed plan/preflight checksum, but never the DSN or executable SQL. Operators may pass `--audit-file <path>` to create an exclusive metadata-only `go-metin2-migration-apply-audit-v1` JSON record for non-empty apply plans; the audit records the applied result, resolved target, driver name, DSN-present flag, ledger-snapshot SHA-256, computed plan SHA-256, and optional confirmed-plan SHA-256 without storing the DSN or executable SQL, and the reserved audit file is removed if apply fails. The lock file is a local single-host guard only, not a distributed database advisory lock.

### `GET /local/account-store/exports/account-character-roster`

Returns a loopback-only, read-only JSON projection of the committed bootstrap account snapshots onto the `0002_account_character_roster` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the account store cannot be listed or any committed snapshot would violate the roster schema shape.

Successful responses include `migration_version`, `migration_name`, deterministic `accounts`, and deterministic `characters`. Account rows expose stable project-owned ids, original login, normalized login, and empire. Character rows expose only non-empty select-screen slots and the account/slot/name/appearance/stat/location/guild/gold fields frozen by the roster migration. The response deliberately omits inventory, equipment, quickslots, quest state, login tickets, authored content, and runtime world state, and it does not open a database, emit SQL, apply migrations, or mutate the account store. Use it as an operator/backfill preflight before future import or repository work; it is not a DB-backed runtime path.

### `GET /local/account-store/exports/character-item-state`

Returns a loopback-only, read-only JSON projection of committed bootstrap account snapshots onto the `0003_character_item_state` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the account store cannot be listed or any committed snapshot would violate the roster or item-state schema shape.

Successful responses include `migration_version`, `migration_name`, deterministic `inventory_items`, deterministic `equipment_items`, and deterministic `quickslots`. Inventory rows are ordered by account/character/slot and expose item id, character id, carried slot, vnum, count, and lock flag. Equipment rows expose item id, character id, named equipment slot, vnum, count, and lock flag. Quickslot rows expose character id, quickslot position, type, and slot. The response deliberately omits roster account rows, executable SQL, item-template definitions, quest state, login tickets, authored content, and runtime world state, and it does not apply migrations or mutate the account store. Use it as a second-stage operator/backfill preflight after the account/character roster export.

### `GET /local/account-store/exports/character-point-state`

Returns a loopback-only, read-only JSON projection of committed bootstrap account snapshots onto the `0011_character_point_state` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the account store cannot be listed or any committed snapshot would violate the roster or point-state schema shape.

Successful responses include `migration_version`, `migration_name`, and deterministic `points` rows. Each non-empty character emits all 255 fixed point-vector indices in ascending order, including zero and negative values, with `character_id`, `point_index`, and signed `value`. The response deliberately omits roster account rows, executable SQL, item-state rows, item-template definitions, quest state, login tickets, authored content, and runtime world state, and it does not apply migrations or mutate the account store. Use it as a point-state backfill preflight after the account/character roster export and before any future DB-backed selected-character repository work.

### `GET /local/login-tickets/exports/auth-login-ticket-handoff`

Returns a loopback-only, read-only JSON projection of committed pending bootstrap login-ticket snapshots onto the `0007_auth_login_ticket_handoff` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the login-ticket store cannot be listed or any committed pending ticket would violate the handoff schema shape.

Successful responses include `migration_version`, `migration_name`, and deterministic `tickets` rows ordered by normalized login and login key. Rows expose the active non-zero `login_key`, `issued_at`, original and normalized login, empire, nullable `consumed_at` (currently omitted because file-backed tickets are deleted on consume), and a transitional `characters_snapshot_json` payload containing only the select-screen character snapshot carried by the ticket. The response deliberately omits executable SQL, account passwords, account roster rows, item-template definitions, quest state, authored content, and runtime world state, and it does not consume tickets, apply migrations, or mutate the login-ticket store. Use it as an operator/backfill preflight for the current authd-to-gamed handoff boundary, not as a DB-backed ticket repository.

### `GET /local/quest-state/exports/character-quest-state`

Returns a loopback-only, read-only JSON projection of the standalone bootstrap quest-state snapshot onto the `0004_character_quest_state` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the account roster cannot be exported, the quest-state snapshot is invalid, or any quest flag references a character name that is not present in the committed roster projection.

Successful responses include `migration_version`, `migration_name`, and deterministic `flags` rows. Each row exposes the resolved `character_id`, source `character` name, `quest_ref`, `flag`, and non-zero `value`. A missing quest-state snapshot returns an empty migration-shaped export, matching `/local/quest-state/validate`. The response deliberately omits executable SQL, quest scripts, account roster rows, item state, login tickets, authored content, and runtime world state, and it does not apply migrations or mutate the account or quest-state stores. Use it as a third-stage operator/backfill preflight after the roster export, because the quest-state projection depends on committed character ids from that roster boundary.

### `GET /local/item-templates/exports/item-template-state`

Returns a loopback-only, read-only JSON projection of the committed authored item-template snapshot onto the current item-template migration boundary (`0009_item_template_refine_info`, after the base `0005_item_template_state` schema and the additive `0006_item_template_safebox_reject_message` storage guard). This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the committed item-template snapshot is invalid or cannot be projected onto the schema shape.

Successful responses include `migration_version`, `migration_name`, deterministic `templates`, and deterministic child rows for non-zero `sockets`, non-zero `attributes`, `use_effects`, `equip_effects`, `refine_infos`, and `refine_materials`. A missing committed `item-templates.json` returns an empty migration-shaped export instead of exporting built-in fallback bootstrap templates. The response deliberately omits executable SQL, content bundles, account/item-instance rows, login tickets, authored actor state, runtime world state, and accepted refine result execution, and it does not apply migrations or mutate the item-template store. Use it as an operator/backfill preflight for authored item metadata before future import or repository work.

### `GET /local/static-actors/exports/static-actor-content-state`

Returns a loopback-only, read-only JSON projection of the committed authored static-actor and interaction-definition snapshots onto the `0008_static_actor_content_state` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if either committed content store cannot be loaded, if any actor/definition/catalog/reward row would violate the schema shape, or if a newer file-backed interaction kind such as `quest_flag` has no owned columns/kind in migration `0008` yet.

Successful responses include `migration_version`, `migration_name`, deterministic `interaction_definitions`, `merchant_catalog_entries`, `static_actors`, and `reward_drops`. Missing committed static-actor or interaction-definition snapshots are exported as empty migration-shaped collections. Actor rows keep only the current authored content boundary (placement, race, optional interaction ref, optional spawn home/group/combat profile, and reward scalar fields), while ordered reward drops are emitted as child rows. The response deliberately omits executable SQL, account/item-instance rows, live runtime-only actor HP/respawn timers/combat targets, content-bundle import previews, and any database apply output; it does not mutate the JSON stores.

### `GET /local/ground-items/exports/bootstrap-ground-item-state`

Returns a loopback-only, read-only JSON projection of the currently pending live bootstrap ground handles onto the `0010_bootstrap_ground_item_state` migration boundary. This endpoint is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if any live item-shaped or gold-shaped handle cannot be projected onto the schema shape.

Successful responses include `migration_version`, `migration_name`, and deterministic `ground_items` rows sorted by visible ground `vid`. Item-shaped rows expose `item_count`; gold-shaped rows expose `gold_amount` and use the current bootstrap gold marker `vnum = 1`; both shapes expose owner identity, map position, and `pickup_range`. This projection reads live in-memory runtime state rather than a committed JSON store, deliberately omits executable SQL and database apply output, and does not make pending ground handles durable across restart. Use it as an operator/backfill preflight for the `0010` boundary before any future import, recovery, or DB-backed world-state repository work.

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
- `persistence.quest_state_store_path`
- `database.configured`
- `database.driver` when configured
- `database.dsn_configured`

The default bootstrap runtime reports local channel `1` and whole-map visibility. When `gamed` is configured for radius AOI, this snapshot reports the active radius and sector-size values selected from the `METIN2_VISIBILITY_*` / `METIN2_GAMED_VISIBILITY_*` environment overrides.

The `persistence` block reports the active bootstrap JSON store locations selected from `METIN2_*_STORE_*` / service-specific environment overrides. Use it before running local backup, restore, validation, or stale-ticket cleanup endpoints so the operator confirms the daemon is pointing at the intended account, login-ticket, static-actor, interaction, item-template, and quest-state stores. A running `gamed` has already rejected empty or overlapping persistence paths at startup: account and login-ticket stores must be separate directory trees that are not existing regular files and not symlink roots, file-backed content and quest-state stores must be separate files that are not existing directories, and no file-backed store may resolve into or be lexically placed inside either persistence directory.

The `database` block is intentionally redacted. It reports whether both DB driver and DSN are configured and shows the driver name, but it never returns the DSN string. Partial DB config fails startup before the ops listener or legacy TCP listener starts, so a running process either has DB migration preflight disabled or has a complete `database/sql` preflight target for `/local/db/migrations/status`, `/local/db/migrations/plan?target_version=N`, and `/local/db/migrations/ledger-snapshot`. The separate `/local/db/migrations/plan-from-ledger-snapshot?target_version=N` endpoint plans from an operator-supplied offline ledger snapshot and does not open the configured DB at all.

### `GET /local/persistence/status`

Returns a loopback-only JSON health snapshot for the bootstrap persistence stores that already have strict runtime validation primitives. This endpoint is read-only, is registered only on `gamed`, rejects non-`GET` methods with `405`, and returns `200` even when one store is invalid so operators can inspect all store statuses in one response.

Current response fields:

- `ok` — `true` only when every included store validates successfully
- `live_selected_character_count` — number of currently connected selected-character sessions registered for live runtime snapshots
- `account_store`
  - `path`
  - `valid`
  - `summary` with the same `account_count`, `character_count`, `empty_character_slot_count` when select-screen holes exist, `logins`, and optional crash-temp fields returned by `/local/account-store/validate`
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present in the active store directory
  - `restore_blocked_by_live_sessions` — `true` when a live selected-character session would make `/local/account-store/restore` fail closed
  - optional `error` when validation fails
- `login_ticket_store`
  - `path`
  - `valid`
  - `summary` with the same `ticket_count`, `character_count`, optional `empty_character_slot_count`, `logins`, `login_keys`, optional issued-at bounds, and optional crash-temp fields returned by `/local/login-tickets/validate`
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present in the active login-ticket store directory
  - `restore_blocked_by_live_sessions` — `true` when a live selected-character session would make `/local/login-tickets/restore` fail closed
  - optional `error` when validation fails
- `item_template_store`
  - `path`
  - `valid`
  - `summary` with the same `template_count`, `vnums`, and optional crash-temp fields returned by `/local/item-templates/validate`
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present next to the active item-template snapshot
  - `restore_blocked_by_live_sessions` — `true` when a live selected-character session would make `/local/item-templates/restore` fail closed
  - optional `error` when validation fails
- `static_actor_store`
  - `path`
  - `valid`
  - `summary` with `actor_count`, deterministic `actor_ids` / `actor_names`, optional `interactable_actor_count`, optional `spawn_group_count`, and optional static-actor crash-temp fields
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present next to the active static-actor snapshot
  - `restore_blocked_by_live_sessions` — `true` when a live selected-character session would make `/local/static-actors/restore` fail closed
  - optional `error` when validation fails
- `interaction_store`
  - `path`
  - `valid`
  - `summary` with `definition_count`, deterministic `definition_keys` (`kind:ref`), and optional interaction crash-temp fields
  - optional `error` when validation fails
- `quest_state_store`
  - `path`
  - `valid`
  - `summary` with `flag_count`, deterministic `characters`, deterministic `quest_refs`, deterministic `flag_keys`, and optional quest-state crash-temp fields
  - `backup_manifest` with `present`, `path`, `format`, `file_count`, total declared `snapshot_size_bytes`, actual `manifest_size_bytes`, and actual `manifest_sha256` when a restored/backup metadata file is currently present next to the active quest-state snapshot
  - `restore_blocked_by_live_sessions` — `true` when a live selected-character session would make `/local/quest-state/restore` fail closed
  - optional `error` when validation fails

Use this endpoint as the first read-only persistence triage check before choosing a narrower validate, crash-temp cleanup, stale-ticket cleanup, backup, restore, or DB-migration status endpoint. It deliberately keeps checking the remaining stores after one store fails, so a corrupt account snapshot does not hide healthy login-ticket, item-template, static-actor, interaction-definition, or quest-state stores. Authored content and quest-state stores that have no committed snapshot are reported as valid empty stores, while corrupt committed snapshots fail closed and still let operators inspect the other persistence surfaces in the same response. The live-session fields make the existing restore guard visible before an operator attempts a replacement restore: account-store, login-ticket, item-template, quest-state, static-actor, and interaction-definition restores are blocked while selected-character sessions are registered, because replacing durable account snapshots, pending auth handoffs, authored item templates, quest flags, authored NPC/spawn content, or authored interaction definitions underneath live player state would diverge from in-memory runtime ownership. Account snapshots, login-ticket snapshots, item-template snapshots, static-actor snapshots, interaction-definition snapshots, and quest-state snapshots that are symlinks now fail closed before JSON decoding, so active committed state cannot transparently point outside the configured store boundary. Crash-temp-shaped symlinks also fail closed across the account, login-ticket, item-template, static-actor, interaction-definition, and quest-state stores instead of being followed, removed, or reported as disposable temp residue. Account and login-ticket character lists also reject duplicate non-zero VIDs in addition to duplicate character IDs and case-folded names, preventing restore/consume paths from admitting ambiguous visible-player identities. Static-actor and interaction-definition store files now also reject a JSON `null` document root or `null` root collection (`static_actors` / `definitions`) instead of treating those lossy snapshots as empty authored content. Login-ticket summaries now expose the embedded select-screen character count and empty-slot count just like account summaries, which helps operators compare pending one-shot authd handoffs against durable account snapshots before consuming or cleaning stale keys. Account, login-ticket, item-template, static-actor, interaction-definition, and quest-state stores that still carry a restored backup manifest now report that manifest explicitly under `backup_manifest`, and validation still verifies that active manifest against the current committed snapshot bytes; malformed manifests, stale checksum/summary data, or item-template manifests that omit an existing committed snapshot make the affected store invalid so operators can detect post-restore drift before treating a replacement store as an exact backup copy. If that manifest path is a symlink, the status snapshot reports only `present` plus the in-store `path` and deliberately does not follow the link, decode the target, hash the target bytes, or expose target metadata; the matching validation error still marks the store invalid. It is an operator/debugging surface, not a gameplay API and not a remote admin API.

### `GET /local/db/migrations/catalog`

Returns a loopback-only metadata-only summary of the embedded project-owned SQL migration catalog. This endpoint is read-only, is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and does not open or inspect the configured DB. Successful responses use the `go-metin2-migration-catalog-summary-v1` format and include `latest_version` plus deterministic migration rows with `version`, `name`, `up_path`, `down_path`, `up_sha256`, and `down_sha256`. The response deliberately excludes executable SQL, DSNs, applied-ledger rows, and runtime store data so operators can compare shipped catalog inventory before using ledger status or offline planning endpoints.

### `GET /local/db/migrations/status`

Returns a loopback-only dry-run migration plan for the embedded project-owned SQL migration catalog. This endpoint is read-only, is registered only on `gamed`, rejects non-`GET` methods with `405`, rejects non-loopback callers with `403`, and returns `409` if the embedded catalog, configured database driver, ledger query, or supplied planning boundary fails validation. With no DB config it plans against an empty applied ledger and reports every catalog migration as pending. With both DB driver and DSN configured, it opens that `database/sql` target only for this read-only status request, queries `schema_migrations` metadata (`version`, `name`, `up_sha256`), closes the connection, and then returns the same metadata-only plan.

Successful responses use the `db/migrations.Plan` JSON shape:

- `current_version`
- `latest_version`
- `up_to_date`
- `pending`
  - `version`
  - `name`
  - `direction`
  - `path`
  - `sha256`

The response deliberately exposes metadata only: it does not include executable SQL text, create tables, apply migrations, roll migrations back, or mutate `schema_migrations`. The project still does not ship a DB driver dependency or production DB engine selection; a configured driver name must already be registered in `database/sql` at startup, so a stock binary with an unknown/unregistered driver now exits before starting ops or legacy listeners instead of running with a migration-preflight configuration that can only fail later. The companion `GET /local/db/migrations/ledger-snapshot` endpoint exports the strict offline ledger-snapshot JSON from the configured ledger target, and `POST /local/db/migrations/plan-from-ledger-snapshot?target_version=N` accepts that shape and returns this same `Plan` response without using the configured DB. Use these as first on-box/offline visibility surfaces for the migration catalog before future slices add driver-backed integration tests, production apply/rollback commands, or DB-backed repositories. They are production-ops preflight scaffolding, not proof that account, character, item, content, or world runtime stores are DB-backed. The programmatic up-migration apply primitive added under `db/migrations` is intentionally not reachable from this ops mux.

### `GET` / `POST /local/static-actor-combat-profiles`

Lists and registers process-local bootstrap static-actor combat profiles for later static-actor or spawn-group authoring. This is loopback-only operator tooling, not gameplay protocol and not durable content storage.

`GET` returns a deterministic JSON object whose `profiles` field lists the built-in `practice_mob` and `training_dummy` profiles plus any registered process-local profiles. Each entry exposes the same canonical defaults returned by registration, including derived `damage_per_normal_attack`, formula fields, presentation fields, respawn delay, and cloned reward descriptors. Nested `death_reward` fields use the same stable snake-case JSON keys accepted by `POST`: `experience`, `gold`, and `drop_vnums`.

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

Exports or imports the deterministic authored bootstrap content bundle used by static actors, interaction definitions, spawn groups, their authored combat-profile snapshots, and the standalone quest-state snapshot.

`GET` canonicalizes and validates the exported bundle before writing pretty-printed canonical JSON, so local operators always receive the same deterministic byte shape used by bundle import/example tests. Export keeps item templates needed only by expanded combat-profile reward-default drop lists as well as templates referenced directly by merchant catalogs and spawn groups, and now includes the configured quest-state flags so content + quest-state QA can be backed up or inspected as one portable artifact. If the runtime exporter ever returns an invalid bundle, the endpoint fails closed with `500` instead of leaking a partial or non-canonical snapshot.

`POST` canonicalizes and validates the whole bundle before applying it, then returns the imported bundle with the same pretty-printed canonical JSON encoder used by export/validate. The request body is bounded to 1 MiB and oversized bodies are rejected before import; invalid UTF-8 request bodies and a JSON `null` root are also rejected before the import callback runs instead of being treated as an empty or lossy replacement bundle. In addition to the per-row validation, non-built-in `combat_profiles` entries must be referenced by at least one static actor or spawn group in the same bundle; unreferenced snapshots are rejected so this endpoint cannot mutate process-local combat profiles without importing authored content that uses them. If a portable combat-profile snapshot names a profile that is already registered in the running process, its canonical defaults must match exactly; conflicting snapshots are rejected before import. Structured merchant `shop_preview` definitions and item-shaped reward drops (`reward_drop_vnums` on spawn groups or bundled combat-profile defaults) must also carry the referenced `item_templates` in the same portable bundle; bundles that omit those templates are rejected before import. Duplicate reward drop vnums in either spawn groups or bundled combat-profile defaults are rejected rather than silently deduplicated during canonicalization. The optional `quest_state` collection uses the same strict quest-flag identity/value rules as `/local/quest-state/validate`; importing a bundle replaces the configured quest-state snapshot with the canonical bundle flags, including an empty snapshot when `quest_state` is omitted.

### `GET` / `POST /local/content-bundle/summary`

Returns a loopback-only JSON summary of authored content bundles without returning the full authoring payload.

`GET` summarizes the currently exported canonical bundle. It is read-only and uses the same export + canonicalization rules as `GET /local/content-bundle`; if the live authored content cannot be exported as a valid bundle, the endpoint fails closed with `500` rather than summarizing a partial snapshot.

`POST` is a dry-run helper for candidate bundles. It uses the same 1 MiB request bound, strict JSON decoding, invalid UTF-8 rejection, JSON `null` root rejection, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle` / `POST /local/content-bundle/validate`, then returns only the compact summary. It does not call the live runtime exporter and does not import or mutate runtime state. Use it when an operator wants quick counts/kind/map impact for a candidate bundle before requesting the full canonical payload from `/local/content-bundle/validate` or committing it with `/local/content-bundle`.

The summary includes deterministic counts for static actors, interactable static actors, spawn groups, portable combat profiles, item templates, quest-state flags and characters, structured shop catalog entries, interaction definitions, per-kind referenced/unreferenced interaction definitions, exact referenced/unreferenced interaction definition identities, compact `interaction_definition_previews` for every authored definition, exact portable `static_actors` identities (`name`, `map_index`, `x`, `y`, `race_num`, optional `combat_profile`, optional `interaction_kind`, optional `interaction_ref`) for both plain and interactable actors, exact `interactable_static_actors` identities with the same compact `preview` strings used by `/local/interaction-visibility`, exact spawn-group identities (`ref`, `name`, `map_index`, `combat_profile`, and reward descriptor) plus resolved `reward_drop_items` metadata for every authored drop vnum, aggregate reward totals (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) plus deterministic `reward_drops` grouped by item vnum with `source_count` and resolved item metadata, exact portable `combat_profiles` snapshots, exact item-template identities (`vnum`, `name`, `stackable`, `max_count`, optional `shop_buy_price`, optional `shop_sell_price`, optional owned transfer/merchant guard flags `anti_get` / `anti_drop` / `anti_give` / `anti_sell` / `anti_stack`, optional selected-character guard metadata (`anti_male`, `anti_female`, job/empire guards, `min_level`), optional `buy_reject_message`, `drop_reject_message`, `give_reject_message`, `pickup_reject_message`, `sell_reject_message`, and optional `pickup_range`), exact structured `shop_catalogs` with per-entry slot / item vnum / resolved item name / count / price / stack metadata plus optional authored buy/sell price, the same owned transfer/merchant guard flags, selected-character guard metadata, rejection messages, and pickup range, exact `warp_destinations`, and per-map static actor / interactable static actor / spawn-group occupancy with per-map `info_actor_count`, `talk_actor_count`, `shop_preview_actor_count`, `shop_catalog_entry_count`, `warp_actor_count`, spawn reward totals, and drop item counts. Invalid candidate bundles return `400`, non-loopback callers return `403`, and methods other than `GET` / `POST` return `405`.

### `GET /local/content-bundle/maps/{map_index}`

Returns one exact per-map row from the live content-bundle summary. This is a loopback-only read-only operator endpoint on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs a compact authored-content view for one map without fetching the full bundle summary or inferring counts from live runtime occupancy. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/maps/{map_index}`

Returns one exact per-map delta row from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `map_index` as a non-zero authored map id, and returns the matching `deltas.maps[]` row with count/amount deltas plus any map-local static-actor, spawn-group, quest-flag-route, shop-route, and warp-route delta rows. It returns `404` when that map has no added/removed/changed tracked content, `400` for malformed map indexes or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one authored map's import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/static-actors`

Returns every portable static-actor row authored on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no static actors, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to audit all authored static placements on a map before checking narrower interactable, merchant, teleporter, or spawn-group projections. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/interactable-static-actors`

Returns every interactable static-actor summary row authored on one map, including the compact resolved preview strings from the broader content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no interactable actors, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs a map-local list of clickable/service-style NPC content without fetching the full summary or filtering global `/local/content-bundle/interactable-static-actors/{name}` responses. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/quest-flag-routes`

Returns every authored quest-flag trigger route whose source static actor is on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no `quest_flag` routes, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to audit map-local quest-state trigger placement without fetching the full bundle summary or filtering global `/local/content-bundle/quest-flag-routes/{actor_name}` responses by `source_map_index`. It is not a gameplay protocol endpoint and does not mutate authored content or live quest state.

### `GET /local/content-bundle/maps/{map_index}/shop-routes`

Returns every authored merchant route whose source static actor is on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no `shop_preview` routes, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to audit map-local merchant placement without fetching the full bundle summary or filtering global `/local/content-bundle/shop-routes/{actor_name}` responses by `source_map_index`. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/warp-routes`

Returns every authored teleporter route whose source static actor is on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no `warp` routes, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to audit map-local teleporter placement without fetching the full bundle summary or filtering global `/local/content-bundle/warp-routes/{actor_name}` responses by `source_map_index`. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/spawn-groups`

Returns every authored spawn-group row whose source placement is on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no `spawn_groups`, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to audit map-local practice-mob placement and reward descriptors without fetching the full bundle summary or filtering global `/local/content-bundle/spawn-groups/{ref}` responses by `map_index`. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/maps/{map_index}/reward-drops`

Returns aggregate authored reward-drop rows contributed by spawn groups on one map. This loopback-only read-only endpoint is registered only on `gamed`; it reuses the same live export + canonicalization + summary path as `GET /local/content-bundle/summary`, returns `404` when the authored bundle has no row for that `map_index`, returns an empty JSON array for a known map with no item-shaped reward drops, rejects malformed/zero map indexes with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Each row uses the same `reward_drops[]` shape as the global summary, but `source_count` is recomputed from only the matching map's `spawn_groups[].reward_drop_vnums`. Use it when local QA needs to audit map-local reward item exposure without expanding every spawn group or fetching the full bundle summary. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/static-actors/{name}`

Returns every exact portable static-actor row for one authored actor name from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `name` is URL-decoded with the same path-safe name rules used by `/local/content-bundle/interactable-static-actors/{name}`. It returns matching `static_actors[]` rows for plain and interactable placements, returns `404` when the live authored bundle has no matching static actor, rejects blank or slash-containing names with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect duplicated authored static placements without fetching the full bundle summary or filtering out plain non-interactable actors. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/static-actors/{name}`

Returns the exact static-actor deltas for one authored actor name from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, URL-decodes `name` with the same path-safe name rules as the read-only static-actor reader, and returns matching `deltas.static_actors[]` rows whose current or candidate portable actor name matches. It returns `404` when that actor name has no added/removed/changed static-actor delta, `400` for malformed names or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one authored NPC/static placement import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/spawn-groups/{ref}`

Returns one exact spawn-group row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `ref` must satisfy the same path-safe authored spawn-group ref rule used by `/local/spawn-groups/by-ref/{ref}`. It returns the matching `spawn_groups[]` row including resolved reward-drop item metadata when present, returns `404` when the live authored bundle has no matching spawn group, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one practice-mob/spawn definition without fetching the full bundle summary or querying live runtime entity state. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/spawn-groups/{ref}`

Returns one exact spawn-group delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `ref` with the same path-safe authored spawn-group rule as the read-only spawn-group reader, and returns the matching `deltas.spawn_groups[]` row with `change` plus canonical current/candidate spawn metadata and resolved reward-drop item details. It returns `404` when that spawn group has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one practice-mob/spawn import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/combat-profiles/{profile}`

Returns one exact portable custom combat-profile snapshot from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `profile` must satisfy the same canonical lowercase snake-case profile identity used by bundled `combat_profiles[].profile`. It returns the matching `combat_profiles[]` row with HP, attack/defense formula metadata, presentation values, respawn delay, optional custom retaliation delta, and death-reward defaults, returns `404` when the live authored bundle has no matching portable profile, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one process-local portable combat profile without fetching the full content-bundle summary or querying the broader runtime profile registry. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/combat-profiles/{profile}`

Returns one exact portable combat-profile delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `profile` with the same canonical lowercase snake-case rule as the read-only combat-profile reader, and returns the matching `deltas.combat_profiles[]` row with `change` plus canonical current/candidate HP, formula, presentation, respawn, and death-reward defaults. It returns `404` when that profile has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one portable combat-profile import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/interaction-kinds/{kind}`

Returns one exact per-kind interaction summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `kind` must be one of the currently owned interaction kinds. It returns JSON with `kind`, `count`, `referenced_count`, and `unreferenced_count`, returns `404` when the live authored bundle has no definitions for that kind, rejects malformed or unsupported identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one authored interaction family without fetching the full content-bundle summary or enumerating every definition preview. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/interaction-definitions/{kind}/{ref}`

Returns one compact authored interaction-definition preview row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `kind` must be one of the currently owned interaction kinds and `ref` must satisfy the canonical path-safe interaction reference rule used by `/local/interactions/{kind}/{ref}`. It returns JSON with `kind`, `ref`, `preview`, and `referenced`, returns `404` when the live authored bundle has no matching definition preview, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one authored definition preview and whether it is referenced by any static actor without fetching the full content-bundle summary or full bundle payload. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/interaction-definitions/{kind}/{ref}`

Returns one exact authored interaction-definition delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `kind` and `ref` with the same owned interaction identity rules as the read-only definition reader, and returns the matching `deltas.interaction_definitions[]` row with `change` plus compact current/candidate previews. It returns `404` when that exact definition has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one authored interaction-definition import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/item-templates/{vnum}`

Returns one exact item-template summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `vnum` must be a non-zero unsigned item template ID. It returns the matching `item_templates[]` row with the currently summarized item metadata, including authored `use_effect` / `equip_effect` payloads when present, returns `404` when the live authored bundle has no matching item template, rejects malformed or zero vnums with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one merchant/reward item template plus its point-effect, guard, and rejection metadata without fetching the full content-bundle summary or opening an in-game merchant window. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/item-templates/{vnum}`

Returns one exact item-template delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `vnum` as a non-zero unsigned item template ID, and returns the matching `deltas.item_templates[]` row with `change` plus canonical current/candidate template metadata. It returns `404` when that item template has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one merchant/reward item-template import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/reward-drops/{item_vnum}`

Returns one exact aggregate reward-drop summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `item_vnum` must be a non-zero unsigned item template ID. It returns the matching `reward_drops[]` row with `source_count` and resolved item-template metadata, returns `404` when the live authored bundle has no matching reward-drop aggregate, rejects malformed or zero vnums with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one authored reward item's aggregate source count plus point-effect, guard, and rejection metadata without fetching the full content-bundle summary or expanding every spawn group. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/reward-drops/{item_vnum}`

Returns one exact aggregate reward-drop delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `item_vnum` as a non-zero unsigned item template ID, and returns the matching `deltas.reward_drops[]` row with `change` plus current/candidate source-count and template metadata. It returns `404` when that reward item has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one authored reward-item import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/quest-state`

Returns a compact quest-state overview from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; it returns the live exported bundle's quest-state counts, quest refs, per-character summaries, and per-quest summaries, rejects non-loopback callers with `403`, forwards live bundle-summary export failures, and accepts only `GET`.

Use it when local QA needs the whole portable quest-state snapshot projection without fetching the broader content-bundle summary or scanning character/quest/flag-specific readers one at a time. It is not a client quest protocol endpoint and does not mutate quest state.

### `GET /local/content-bundle/quest-state/characters/{character}`

Returns one exact quest-state character summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `character` is URL-decoded with the same path-safe name rules used by other local character-name readers. It returns the matching `quest_state_characters[]` row with deterministic flag summaries, returns `404` when the live exported bundle has no persisted non-zero flags for that character, rejects blank or slash-containing names with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect authored/bundled quest-state flags for one character without fetching the full content-bundle summary or calling the mutable `/local/quest-state/transition` harness. It is not a client quest protocol endpoint and does not mutate quest state.

### `GET /local/content-bundle/quest-state/quests/{quest_ref}`

Returns one exact quest-state quest summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `quest_ref` is URL-decoded and must satisfy the currently owned `quest:<name>` lower-snake reference rule. It returns the matching `quest_state_quests[]` row grouped by quest ref with deterministic character and flag summaries, returns `404` when the live exported bundle has no persisted non-zero flags for that quest, rejects blank, slash-containing, or otherwise invalid quest refs with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect all authored/bundled quest-state flags for one quest across characters without fetching the full content-bundle summary or calling the mutable `/local/quest-state/transition` harness. It is not a client quest protocol endpoint and does not mutate quest state.

### `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}`

Returns one exact quest-state flag row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `character` is URL-decoded and must satisfy the bootstrap quest-state character-name rule, `quest_ref` must satisfy the owned `quest:<name>` lower-snake reference rule, and `flag` must be a lower-snake quest flag name. It returns the canonical row shape with `character`, `quest_ref`, `name`, and `value`, returns `404` when that exact persisted non-zero flag is absent, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one authored/bundled flag without fetching the full content-bundle summary or scanning a whole character/quest grouping. It is not a client quest protocol endpoint and does not mutate quest state.

### `POST /local/content-bundle/import-preview/quest-state/characters/{character}`

Returns the character-scoped quest-state flag deltas from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `character` with the bootstrap quest-state character-name rule, and returns only matching `quest_state_flags[]` delta rows for that character. It returns `404` when that character has no added/removed/changed deltas, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect every authored quest-state import impact for one character before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a client quest protocol endpoint and does not mutate quest state.

### `POST /local/content-bundle/import-preview/quest-state/quests/{quest_ref}`

Returns the quest-scoped quest-state flag deltas from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates `quest_ref` with the owned `quest:<name>` lower-snake reference rule, and returns only matching `quest_state_flags[]` delta rows for that quest. It returns `404` when that quest has no added/removed/changed deltas, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect every authored quest-state import impact for one quest ref before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a client quest protocol endpoint and does not mutate quest state.

### `POST /local/content-bundle/import-preview/quest-state/flags/{character}/{quest_ref}/{flag}`

Returns one exact quest-state flag delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, validates the path identity with the same `character`, `quest_ref`, and lower-snake `flag` rules as the read-only quest-state flag endpoints, and returns the matching `quest_state_flags[]` delta with `change` plus canonical current/candidate snapshots. It returns `404` when that exact flag has no added/removed/changed delta, `400` for malformed identities or invalid candidate bundles, `403` for non-loopback callers, and accepts only `POST`.

Use it when local QA needs to inspect one authored quest-state import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a client quest protocol endpoint and does not mutate quest state.

### `GET /local/content-bundle/shop-catalogs/{kind}/{ref}`

Returns one exact structured shop-catalog summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; the only accepted `kind` for this path is `shop_preview`, and `ref` must satisfy the same path-safe interaction reference rule used by `/local/interactions/{kind}/{ref}`. It returns `404` when the live authored bundle has no matching catalog, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one merchant catalog, including resolved item-template metadata and guard/rejection fields, without fetching the full bundle summary or opening the merchant in-game. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/shop-catalogs/{kind}/{ref}`

Returns one exact structured shop-catalog delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, accepts only `kind = shop_preview`, and validates `ref` with the same path-safe interaction reference rule used by `/local/content-bundle/shop-catalogs/{kind}/{ref}`. It returns the matching `deltas.shop_catalogs[]` row with `change` plus canonical current/candidate catalog snapshots, returns `404` when that catalog has no added/removed/changed delta, rejects malformed identities or invalid candidate bundles with `400`, rejects non-loopback callers with `403`, and accepts only `POST`.

Use it when local QA needs to inspect one merchant catalog import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/quest-flag-routes/{actor_name}`

Returns every exact quest-flag route summary row for one authored static-actor name from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `actor_name` is URL-decoded with the same path-safe name rules used by `/local/content-bundle/interactable-static-actors/{name}`. It returns every matching `quest_flag_routes[]` row so duplicated quest-trigger placements with the same actor name remain inspectable, returns `404` when no route uses that name, rejects blank or slash-containing names with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect exact actor-to-quest-flag trigger placement (`ref`, text, `quest_ref`, `quest_flag`, `quest_from`, `quest_to`) without fetching the full content-bundle summary or mutating a selected character's quest state. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/quest-flag-routes/{actor_name}`

Returns every exact quest-flag route delta for one authored static-actor name from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, URL-decodes `actor_name` with the same path-safe name rules used by `/local/content-bundle/quest-flag-routes/{actor_name}`, and returns matching `deltas.quest_flag_routes[]` rows whose current or candidate route actor name matches. It returns `404` when that actor has no added/removed/changed quest-flag route delta, rejects malformed identities or invalid candidate bundles with `400`, rejects non-loopback callers with `403`, and accepts only `POST`.

Use it when local QA needs to inspect one quest-trigger placement import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content or live quest state.

### `GET /local/content-bundle/shop-routes/{actor_name}`

Returns every exact shop-route summary row for one authored merchant actor name from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `actor_name` is URL-decoded with the same path-safe name rules used by `/local/content-bundle/interactable-static-actors/{name}`. It returns every matching `shop_routes[]` row so duplicated merchant placements with the same actor name remain inspectable, returns `404` when no route uses that name, rejects blank or slash-containing names with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect exact actor-to-catalog placement for one merchant name without fetching the full content-bundle summary or opening the merchant in-game. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/shop-routes/{actor_name}`

Returns every exact shop-route delta for one authored merchant actor name from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, URL-decodes `actor_name` with the same path-safe name rules used by `/local/content-bundle/shop-routes/{actor_name}`, and returns matching `deltas.shop_routes[]` rows whose current or candidate route actor name matches. It returns `404` when that merchant actor has no added/removed/changed route delta, rejects malformed identities or invalid candidate bundles with `400`, rejects non-loopback callers with `403`, and accepts only `POST`.

Use it when local QA needs to inspect one merchant placement import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/warp-destinations/{kind}/{ref}`

Returns one exact authored warp-destination summary row from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; the only accepted `kind` for this path is `warp`, and `ref` must satisfy the canonical path-safe interaction reference rule. It returns `404` when the live authored bundle has no matching warp destination, rejects malformed identities with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect one teleporter destination (`text`, `map_index`, `x`, `y`) without fetching the full bundle summary or triggering an in-game transfer. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/warp-destinations/{kind}/{ref}`

Returns one exact authored warp-destination delta from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, accepts only `kind = warp`, and validates `ref` with the same path-safe interaction reference rule used by `/local/content-bundle/warp-destinations/{kind}/{ref}`. It returns the matching `deltas.warp_destinations[]` row with `change` plus canonical current/candidate destination snapshots, returns `404` when that destination has no added/removed/changed delta, rejects malformed identities or invalid candidate bundles with `400`, rejects non-loopback callers with `403`, and accepts only `POST`.

Use it when local QA needs to inspect one teleporter destination import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `GET /local/content-bundle/warp-routes/{actor_name}`

Returns every exact warp-route summary row for one authored teleporter actor name from the live content-bundle summary. This loopback-only read-only endpoint is registered only on `gamed`; `actor_name` is URL-decoded with the same path-safe name rules used by `/local/content-bundle/interactable-static-actors/{name}`. It returns every matching `warp_routes[]` row so duplicated teleporter placements with the same actor name remain inspectable, returns `404` when no route uses that name, rejects blank or slash-containing names with `400`, rejects non-loopback callers with `403`, and accepts only `GET`.

Use it when local QA needs to inspect exact actor-to-destination placement for one teleporter name without fetching the full content-bundle summary or triggering an in-game transfer. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview/warp-routes/{actor_name}`

Returns every exact warp-route delta for one authored teleporter actor name from a candidate content-bundle import preview without mutating runtime state. This loopback-only endpoint is registered only on `gamed`; it accepts the same candidate bundle body as `POST /local/content-bundle/import-preview`, URL-decodes `actor_name` with the same path-safe name rules used by `/local/content-bundle/warp-routes/{actor_name}`, and returns matching `deltas.warp_routes[]` rows whose current or candidate route actor name matches. It returns `404` when that teleporter actor has no added/removed/changed route delta, rejects malformed identities or invalid candidate bundles with `400`, rejects non-loopback callers with `403`, and accepts only `POST`.

Use it when local QA needs to inspect one teleporter placement import impact before applying a bundle, without fetching and manually filtering the broad import-preview response. It is not a gameplay protocol endpoint and does not mutate authored content.

### `POST /local/content-bundle/import-preview`

Previews the impact of importing a candidate authored bootstrap content bundle without mutating runtime state. This loopback-only endpoint uses the same 1 MiB request bound, strict JSON decoding, invalid UTF-8 rejection, JSON `null` root rejection, and `contentbundle.Canonicalize(...)` rules as `POST /local/content-bundle` / `POST /local/content-bundle/validate`, then compares the candidate against the currently exported canonical live bundle.

Successful responses are JSON with:

- `current` — the current live content-bundle summary
- `candidate` — the candidate bundle summary after canonicalization
- `deltas` — compact count deltas where each count has `current`, `candidate`, and signed `delta`

The current delta set covers static actors, exact added/removed portable static-actor rows, interactable actors, spawn groups, exact added/removed/changed spawn-group records plus focused exact-spawn lookup, portable combat profiles, exact added/removed/changed combat-profile records plus focused exact-profile lookup, item templates, exact added/removed/changed item-template records, quest-flag triggers, exact added/removed/changed quest-flag route records plus focused actor-name quest-flag-route lookup, shop catalog entries, exact added/removed/changed shop catalog records, shop routes, exact added/removed/changed shop route records plus focused actor-name shop-route lookup, warp destinations, exact added/removed/changed warp destination records, warp routes, exact added/removed/changed warp route records plus focused actor-name warp-route lookup, reward drop items, interaction definitions, referenced interaction definitions, unreferenced interaction definitions, per-interaction-kind count/reference deltas, and a deterministic `maps` array for only map indexes whose tracked authored counts change. Each static-actor delta carries `change` (`added` or `removed`) plus the canonical `current` and/or `candidate` portable static-actor row that caused it; each item-template delta is keyed by `vnum`, carries `change` (`added`, `removed`, or `changed`), and includes the canonical `current` and/or `candidate` template snapshot that caused the change; each combat-profile delta is keyed by `profile`, carries `change` (`added`, `removed`, or `changed`), and includes the canonical current and/or candidate profile snapshot with HP, damage/formula, presentation, respawn delay, and death-reward defaults; each spawn-group delta is keyed by `ref`, carries the same `change` values, and includes the summarized `current` and/or `candidate` spawn group with resolved reward-drop item metadata when applicable; each reward-drop delta is keyed by item `vnum`, carries `change`, and includes the grouped current and/or candidate source counts plus template metadata; each shop-catalog delta is keyed by interaction `kind` + `ref`, carries `change`, and includes the canonical current and/or candidate catalog with title, entry count, and resolved per-entry item/template metadata; each warp-destination delta is keyed by interaction `kind` + `ref`, carries `change`, and includes the canonical current and/or candidate destination with optional text plus target `map_index`/`x`/`y`; each quest-flag-route delta is keyed by actor name, source `map_index`/`x`/`y`, and interaction `ref`, carries `change`, and includes the compact current and/or candidate route with text plus `quest_ref`, `quest_flag`, `quest_from`, and `quest_to`; each shop-route delta is keyed by actor name, source `map_index`/`x`/`y`, and interaction `ref`, carries `change`, and includes the compact current and/or candidate route with title and entry count; each warp-route delta uses the same actor/source/ref key and includes the compact current and/or candidate route with text plus target `map_index`/`x`/`y`; each interaction-kind delta carries before/after/signed values for total definitions, referenced definitions, and unreferenced definitions for the changed kind; each map delta carries before/after/signed count values for static actors, interactables, info/talk/quest-flag/shop/warp service actors, shop catalog entries, spawn groups, and reward-drop item counts. Invalid JSON, unknown fields, dangling refs, invalid static actors/spawn groups/combat profiles, missing merchant or reward-drop item templates, and other candidate bundle validation failures return `400`; non-loopback callers return `403`; methods other than `POST` return `405`. Use this endpoint when an operator wants a no-mutation before/after audit before applying a replacement bundle through `POST /local/content-bundle`.

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

### `GET /local/players/{name}`

Returns one exact-name connected-character snapshot using the same JSON shape as a single row from `/local/players`.
This endpoint is loopback-only and read-only. Percent-encoded spaces in character names are accepted, but empty names or names containing `/` after decoding return `400`; well-formed names without a currently connected selected-session snapshot return `404`.

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

### `GET /local/visibility/{name}`

Returns one exact-name character visibility snapshot using the same JSON shape as a single row from `/local/visibility`.
This endpoint is loopback-only and read-only. Percent-encoded spaces in character names are accepted, but empty names or names containing `/` after decoding return `400`; well-formed names without a currently connected selected-session snapshot return `404`.

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
Temporary pending ground items are surfaced with their visible `vid`, `vnum`, optional `count`, optional display `owner_name`, owner identity (`owner_login`, `owner_character_id`, `owner_vid`), optional `gold_amount`, `pickup_range`, effective `map_index`, and `x/y/z` position so operator map snapshots show both connected actors and transient ground occupancy without losing the owned ground-entry identity used by stale-pickup guards.

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
- `pickup_range`
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

Current previews cover self-only `info` / `talk`, state-aware `quest_flag` acknowledgement-or-mismatch text, structured merchant `shop_preview` catalog summaries, and compact `warp` destination summaries. `quest_flag` previews use the same read-only compare-and-set evaluator as `/local/quest-state/transition-preview`, so a selected character that no longer satisfies the authored `quest_from` value sees `Quest requirements are not met.` in the preview without mutating the quest-state store.
The per-character subject snapshot in this endpoint also reuses the same player `dead: true` flag exposed by `/local/players` while a still-connected owner remains at the retaliation-owned `0`-HP floor.

`GET /local/interaction-visibility/{name}` returns the same interaction-visibility snapshot for one connected bootstrap character by exact character name.
It is loopback-only, read-only, accepts URL-escaped names, rejects blank or slash-containing path values with `400`, and returns `404` when the character is not connected.
This exact-name view mirrors `/local/visibility/{name}` but narrows the payload to interactable static actors plus their compact resolved previews or fail-closed resolution markers.

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
`GET /local/maps/{map_index}/combat-targets` returns the same snapshot shape filtered to active target selections whose selected subject is currently on one effective map.
These responses reuse the runtime debug snapshot shape documented in `spec/protocol/combat-normal-attack-bootstrap.md`:

- `subject_entity_id`
- `subject`
- `target_vid`
- `snapshot_version`
- `hp_percent`
- `target_current_hp`
- `target_max_hp`
- `normal_attack_damage`
- `target_attack_value`
- `target_defense_value`
- `actor`
- optional `engaged_by_entity_id`
- optional `engaged_by`
- optional `retaliation_point_delta`
- optional `retaliation_server_origin`
- optional `retaliation_pending`
- optional `retaliation_ready_at`
- optional `retaliation_remaining_ms`

The embedded `subject` field uses the same effective connected-character snapshot shape exposed by `/local/players`, so combat-target debugging can verify the current selected subject location/dead-state without a second lookup.
The resolved HP/damage fields expose the current runtime-owned target HP, profile max HP, compact normal-hit damage result, and authored attack/defense formula inputs used by accepted attacks. After a spawn-backed practice mob has accepted an owner-side hit, the optional engagement fields expose the current aggro-lite owner, fresh third-party target attempts fail closed internally as `target_engaged`, and the optional retaliation fields expose the runtime-owned owner-side point-loss cadence without introducing a new gameplay packet or mutation endpoint. When a delayed server-origin retaliation beat is armed for the currently selected target snapshot, the read-only row includes `retaliation_pending: true`, `retaliation_ready_at`, and `retaliation_remaining_ms`; due-but-unflushed beats remain visible with `retaliation_remaining_ms = 0` until the pending server-frame path flushes them.
All combat-target endpoints are loopback-only and read-only. The exact-name endpoint returns `404` when the character is not connected, no longer has a live session hook, has no active target, or the target no longer resolves through the current visibility/range/leash/aggro/runtime combat rules; the global and map-local list endpoints omit unresolved/stale selections instead of leaking hidden or invalid target data. These reads mirror the gameplay gates without cleaning up stale engagement or target state as a side effect. The map-local endpoint rejects malformed or zero map-index path values with `400`, returns `404` when the runtime cannot resolve that map-scoped snapshot, and returns an empty JSON array for a known map that currently has no active target selections.

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

### `GET /local/spawn-group-return-steps` and `GET /local/spawn-group-return-steps/{entity_id}`

Returns the deterministic list of pending server-owned spawn-group return-step timers for live authored spawn groups that currently classify as `return_required`.
These endpoints are loopback-only, read-only, reject non-`GET` methods with `405`, and are intended for QA/debugging of the capped return-step executor frozen in `spec/protocol/spawn-leash-bootstrap.md`.

Each row exposes:

- `entity_id` — runtime static-actor entity / client-visible static-actor `VID`
- `ready_at` — the server-owned timestamp when the next flush can apply one capped return step
- `remaining_ms` — milliseconds until `ready_at`, clamped to `0` when the timer is already due but has not yet been flushed through the pending server-frame path
- `actor` — the current materialized spawn-group snapshot, including the leash state that is still `return_required`
- `step` — the same planned capped step shape returned by the mutating one-step trigger, including `next` and `complete`

Rows are sorted by `entity_id`.
Actors that are missing, dead, no longer spawn-backed, no longer `return_required`, or no longer plan a safe capped step are omitted from the list and return `404` from the exact-entity endpoint.
Successful content-bundle replacement prunes stale pending rows after the imported actor set commits, so old return-step deadlines for removed actors do not survive and fire against unrelated replacement content. Canonical no-op content-bundle imports run the same stale-row pruning pass while preserving still-valid live actor deadlines.
Dead return-required spawn groups that still own a pending respawn timer remain visible in the broader spawn/respawn snapshots, but do not arm or expose automatic return-step rows until respawn makes them live again.
Once `FlushServerFrames()` runs after a due timer and the actor steps back inside leash radius, that actor disappears from this snapshot.
If the step leaves the actor still live and `return_required`, the row remains visible with a refreshed `ready_at` after the next server-owned deadline is armed.

### `GET /local/spawn-groups`

Returns the deterministic list of currently materialized spawn-backed runtime actors: static-actor snapshots whose `spawn_group_ref` is non-empty.
This endpoint is loopback-only, read-only, rejects non-`GET` methods with `405`, and is intended for QA/debugging of the authored `spawn_groups` contract frozen in `spec/protocol/content-spawn-groups-bootstrap.md`.

`GET /local/maps/{map_index}/static-actors` returns the full static-actor snapshot subset for one effective map.
It uses the same snapshot shape as `/local/static-actors`, rejects malformed or zero map-index path values with `400`, returns `404` when the runtime cannot resolve that map-scoped snapshot, and returns an empty JSON array when the map is known but has no static actors.
Use it when local QA needs all service actors plus spawn-backed actors on one map without fetching the broader `/local/maps/{map_index}` occupancy row.

`GET /local/spawn-groups/{entity_id}` returns one exact spawn-backed actor row by runtime entity ID / client-visible static-actor `VID`.
Invalid path IDs return `400`; missing entities or ordinary non-spawn static actors return `404`.
`GET /local/spawn-groups/{entity_id}/leash?radius=<positive-int>` returns a read-only spawn-leash classification for one materialized spawn-backed actor. The response embeds the same `actor` row plus `home`, `current`, `radius`, `status`, `return_required`, and optional `return_target`. The `home` fields stay anchored to the authored `spawn_groups` placement while `current` reflects the materialized actor position at lookup time, so local QA can confirm `within_radius` or `return_required` after a runtime/static-actor position update. It rejects malformed entity IDs or missing/non-positive `radius` values with `400`, returns `404` for missing/non-spawn actors, and does not mutate actor position, HP, death state, respawn timers, target ownership, or visible-world membership.
`GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` returns the same read-only leash rows for every materialized spawn-backed actor on one effective map. It rejects malformed/zero map indexes and missing/non-positive radii with `400`, returns `404` for unknown maps, and returns an empty JSON array for known maps whose current occupancy has no spawn groups.
`POST /local/spawn-groups/{entity_id}/return-step?max_step=<positive-int>` is the loopback-only one-step return trigger for live spawn-backed actors that are currently `return_required`. It accepts no body, plans one capped step with the default leash radius, persists the materialized static-actor position to `step.next` before mutating runtime state, returns `{actor,step}`, preserves HP/death/reward/combat metadata, and reuses ordinary static-actor visibility deltas for already-online viewers. When the step actually moves the actor, it also releases current practice-mob engagement and clears selected combat targets bound to that actor's visible `VID`; actors that are already `at_home` or still `within_radius` return a no-op `complete = true` step without persistence, target clears, engagement release, queued frames, or automatic scheduling. If a manual/operator step leaves the actor still `return_required`, it replaces any older pending automatic deadline with a new one-second return-step deadline measured from the manual step time, preventing stale pre-manual due times from firing immediately after operator recovery. The runtime now also applies the first server-owned due return step from the pending-frame flush loop for actors already left `return_required`, including live persisted actors restored already outside leash at startup, using fixed `max_step = 100`, the same stepped visibility/target-reset path, and one-second re-arm only while they remain outside leash. A due server-owned step whose static snapshot persistence fails emits no visibility frames, leaves runtime/persisted position unchanged, and retries on a later one-second deadline while the actor still reports `return_required`. It rejects malformed IDs or missing/non-positive `max_step` values with `400`, rejects non-loopback callers with `403`, and returns `404` for missing/non-spawn/dead actors or unsafe step failures.
`POST /local/spawn-groups/{entity_id}/return-home` is the paired loopback-only controlled exact-home trigger for live spawn-backed actors. It accepts no body, persists the materialized static-actor position back to preserved authored home before mutating runtime state only when coordinates actually change, returns the same leash snapshot shape with `status = at_home`, clears stale selected-target ownership for that actor, clears any pending automatic return-step deadline for that actor, and reuses ordinary static-actor visibility deltas (`CHARACTER_DEL` for old-position viewers and add/info/update for home viewers). Already-at-home actors skip the no-op snapshot write and still run the lifecycle reset, so a temporary static-store failure cannot preserve stale target/engagement ownership. Removing a materialized spawn-backed actor also clears any pending automatic return-step deadline for that entity ID after the removal commits. It rejects malformed IDs with `400`, rejects non-loopback callers with `403`, and returns `404` for missing/non-spawn/dead actors or unsafe return failures. These endpoints are operator lifecycle tooling, not autonomous mob AI or final chase/return packet choreography.
`GET /local/maps/{map_index}/spawn-groups` returns the same snapshot shape for one effective map's materialized spawn-backed actors, rejects malformed or zero map-index path values with `400`, and returns `404` when the runtime cannot resolve that map-scoped snapshot.
Use it when local QA already knows the map under investigation and needs the authored spawn subset without fetching the broader `/local/maps/{map_index}` occupancy row.
`GET /local/maps/{map_index}/static-actor-respawns` returns pending static-actor respawn rows for one effective map, using the same row shape as `/local/static-actor-respawns` and returning a non-null empty JSON array when the map is known but currently has no pending respawn timers. For spawn-backed actors in the shipped file-backed runtime, a due respawn first persists the materialized actor back to its authored home; if that write fails, the dead actor stays at its previous position and the due respawn row remains visible for retry with `remaining_ms = 0`.
`GET /local/maps/{map_index}/spawn-group-return-steps` returns pending server-owned spawn return-step rows for one effective map, using the same row shape as `/local/spawn-group-return-steps` and returning an empty JSON array when the map is known but currently has no pending return-step timers.
`GET /local/maps/{map_index}/combat-targets` returns active selected combat-target rows for one effective map, using the same row shape as `/local/combat-targets` and returning an empty JSON array when the map is known but currently has no active selections.

Each row reuses the same static-actor snapshot shape exposed by `/local/static-actors`, including:

- `entity_id`
- `name`
- `map_index`
- `x`
- `y`
- `race_num`
- `combat_profile`
- `combat_max_hp`
- `combat_normal_damage`
- `combat_attack_value`
- `combat_defense_value`
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
Returned static-actor snapshots expose the resolved `combat_max_hp`, `combat_normal_damage`, `combat_attack_value`, `combat_defense_value`, `combat_level`, `combat_rank`, and any non-default `retaliation_point_delta` from that combat profile, so operator/debug consumers can inspect authored formula, presentation, and hostility metadata without re-resolving the profile registry.
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
- `GET /local/content-bundle/summary` and dry-run `POST /local/content-bundle/summary` include compact `shop_routes` and `warp_routes` entries for placed service actors, aggregate `reward_drops` entries for authored reward items, distinct quest-state flag/character/quest counts for bundled quest-state rows, and focused readers (`/local/content-bundle/static-actors/{name}` / `/local/content-bundle/shop-routes/{actor_name}` / `/local/content-bundle/warp-routes/{actor_name}` / `/local/content-bundle/reward-drops/{item_vnum}` / `/local/content-bundle/maps/{map_index}/reward-drops`) let local QA inspect one authored actor name, service placement, global reward-drop aggregate, or map-local reward-drop aggregate without fetching or applying the full bundle
- `POST /local/content-bundle/import-preview` compares a candidate replacement against the live exported bundle and returns no-mutation `current` / `candidate` summaries plus count/amount `deltas`, including per-interaction-kind reference deltas, per-definition `added` / `removed` / `changed` deltas with compact current/candidate previews, per-map before/after/signed deltas for changed authored map counts, grouped reward-drop deltas, spawn reward EXP/gold totals, and quest-state flag/character/quest-count changes

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
curl http://127.0.0.1:6060/local/interaction-visibility/MkmkWar
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
curl http://127.0.0.1:6060/local/maps/42/combat-targets
```

These combat-target endpoints are read-only local runtime snapshots. They do not introduce new client packets and still fail closed when a selected target is stale, invisible, or no longer combat-targetable.

List currently materialized authored spawn-group actors:

```bash
curl http://127.0.0.1:6060/local/spawn-groups
curl http://127.0.0.1:6060/local/spawn-groups/by-ref/practice.reward_mob
curl 'http://127.0.0.1:6060/local/spawn-groups/117440769/leash?radius=400'
curl 'http://127.0.0.1:6060/local/maps/42/spawn-group-leashes?radius=400'
curl -X POST 'http://127.0.0.1:6060/local/spawn-groups/117440769/return-step?max_step=100'
curl http://127.0.0.1:6060/local/spawn-group-return-steps
curl http://127.0.0.1:6060/local/spawn-group-return-steps/117440769
curl -X POST http://127.0.0.1:6060/local/spawn-groups/117440769/return-home
curl http://127.0.0.1:6060/local/maps/42/static-actors
curl http://127.0.0.1:6060/local/maps/42/spawn-groups
curl http://127.0.0.1:6060/local/maps/42/static-actor-respawns
curl http://127.0.0.1:6060/local/maps/42/spawn-group-return-steps
```

The spawn-group snapshots filter to attackable content materialized from `spawn_groups`; use `/local/static-actors` for the global full static-actor set, or `/local/maps/{map_index}/static-actors` for the full map-local set.
The by-ref endpoint is loopback-only like the rest of the spawn-group inspection surface and looks up the materialized actor by authored `spawn_group_ref`, returning `400` for malformed refs and `404` when a well-formed ref is not currently live. The exact leash endpoint is also loopback-only and computes the current pure classifier (`home`, `current`, `radius`, `status`, `return_required`, optional `return_target`) for one materialized spawn group without moving the actor or changing combat/runtime state; `/local/maps/{map_index}/spawn-group-leashes` returns the same classifier shape for the current map-local spawn subset. The return-step and return-home endpoints are mutating local/operator tooling: return-step applies one capped planned step for a `return_required` actor, return-home moves one live spawn group back to authored home and clears stale selected-target ownership, and both persist the static-actor position before reusing ordinary visibility rebuild frames for already-online viewers. The read-only `/local/spawn-group-return-steps` endpoints expose currently armed server-owned return-step deadlines plus their planned next capped step without mutating the actor. The server-owned return-step executor uses the same capped-step path from pending-frame flushes and stops re-arming as soon as the stepped actor is back inside leash radius, rather than scheduling an extra no-op correction to exact home.
`/local/maps` also embeds the full static-actor list and same spawn-backed subset per occupied map as `static_actors`, `spawn_group_count`, and `spawn_groups`; the exact `/local/maps/{map_index}/static-actors`, `/local/maps/{map_index}/spawn-groups`, `/local/maps/{map_index}/spawn-group-leashes`, `/local/maps/{map_index}/static-actor-respawns`, and `/local/maps/{map_index}/combat-targets` endpoints return only those map-local subsets when QA does not need the full occupancy row.

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

Inspect compact authored-content summaries without fetching the full bundle payload:

```bash
curl http://127.0.0.1:6060/local/content-bundle/summary
curl http://127.0.0.1:6060/local/content-bundle/maps/1
curl -X POST --data-binary @docs/examples/bootstrap-npc-service-bundle.json http://127.0.0.1:6060/local/content-bundle/import-preview/maps/1
curl http://127.0.0.1:6060/local/content-bundle/maps/1/quest-flag-routes
curl http://127.0.0.1:6060/local/content-bundle/maps/1/shop-routes
curl http://127.0.0.1:6060/local/content-bundle/maps/1/warp-routes
curl http://127.0.0.1:6060/local/content-bundle/maps/1/spawn-groups
curl http://127.0.0.1:6060/local/content-bundle/maps/1/reward-drops
curl http://127.0.0.1:6060/local/content-bundle/static-actors/Village%20Guide
curl http://127.0.0.1:6060/local/content-bundle/quest-flag-routes/Village%20Guide
curl http://127.0.0.1:6060/local/content-bundle/spawn-groups/practice.qa_reward_mob
curl http://127.0.0.1:6060/local/content-bundle/combat-profiles/practice_reward_profile
curl http://127.0.0.1:6060/local/content-bundle/interactable-static-actors/Village%20Guide
curl http://127.0.0.1:6060/local/content-bundle/interaction-kinds/shop_preview
curl http://127.0.0.1:6060/local/content-bundle/item-templates/27001
curl http://127.0.0.1:6060/local/content-bundle/reward-drops/27001
curl http://127.0.0.1:6060/local/quest-state
curl http://127.0.0.1:6060/local/content-bundle/quest-state/flags/QuestHero/quest:first_steps/step
curl http://127.0.0.1:6060/local/content-bundle/shop-catalogs/shop_preview/npc:qa_merchant
curl -X POST --data-binary @docs/examples/bootstrap-npc-service-bundle.json http://127.0.0.1:6060/local/content-bundle/import-preview/shop-catalogs/shop_preview/npc:qa_merchant
curl http://127.0.0.1:6060/local/content-bundle/warp-destinations/warp/npc:qa_teleporter
curl -X POST --data-binary @docs/examples/bootstrap-npc-service-bundle.json http://127.0.0.1:6060/local/content-bundle/import-preview/warp-destinations/warp/npc:qa_teleporter
```

Dry-run one quest-state transition before applying it:

```bash
curl -X POST http://127.0.0.1:6060/local/quest-state/transition-preview \
  -H 'Content-Type: application/json' \
  --data '{"character":"QuestHero","quest_ref":"quest:first_steps","flag":"step","from":0,"to":1}'
```

`/local/content-bundle/spawn-groups/{ref}` is a loopback-only exact-ref reader over the exported bundle summary. It returns one authored spawn-group row including resolved reward-drop item metadata, or `404` when that authored ref is absent, so local QA can inspect one practice-mob definition without fetching the full summary.

`/local/content-bundle/combat-profiles/{profile}` is a loopback-only exact-profile reader over the exported bundle summary. It returns one portable custom combat-profile snapshot with HP/damage/formula/presentation/respawn/reward defaults, or `404` when the live bundle has no such portable custom profile, so local QA can inspect one authored profile without fetching the full summary or listing the whole process-local registry.

`/local/content-bundle/static-actors/{name}` is a loopback-only exact-name reader over the exported bundle summary. It returns every matching portable static-actor row, including plain non-interactable actors, so duplicate placements with the same name remain inspectable without fetching the full summary.

`/local/content-bundle/interactable-static-actors/{name}` is a loopback-only exact-name reader over the exported bundle summary. It returns every matching authored interactable actor row so duplicate interactable placements with the same name remain inspectable, and it is read-only QA/debug tooling rather than gameplay protocol or content mutation.

`/local/content-bundle/reward-drops/{item_vnum}` is a loopback-only exact-vnum reader over the exported bundle summary. It returns one aggregate reward-drop row with `source_count` and resolved item metadata, or `404` when that item is not currently authored as a reward drop, so local QA can inspect one reward item without fetching the full summary or opening every spawn-group row.

## Docker note

The runtime image keeps debug information because builds are not stripped with `-ldflags="-s -w"`.
That preserves DWARF/symbol data for better profiling and stack analysis while still using a lightweight final image.
