# File-Store Backup/Restore Drill

This runbook freezes the combined loopback-only backup → validate → restore sequence for the six bootstrap JSON stores that already own manifested backup primitives on `gamed`. It is an operator recovery drill for reconnect/restart and migration preflight windows, not a remote admin API and not daemon startup policy.

## Scope and guardrails

Use this workflow when you need to preserve or replace the current file-backed PvE-relevant state:

- durable accounts / characters / inventory / gold (`accountstore`)
- pending authd-to-gamed login tickets (`loginticket`)
- authored item templates (`itemstore`)
- authored interaction definitions (`interactionstore`)
- authored static actors / spawn content (`staticstore`)
- standalone quest flags (`queststate`)

The current boundary is deliberately narrow:

- endpoints exist only on `gamed` and remain loopback-only (default ops listener `127.0.0.1:6060`);
- every mutating backup/restore call is `POST` with a small JSON body;
- restore refuses live selected-character sessions;
- restore is a replacement into an empty destination, not an online merge;
- ground-item / ground-gold handles remain live runtime state and are **not** covered by this drill;
- committed account gold/inventory and quest flags rematerialize across `gamed` process restart from the same FileStore paths (see [character-state process-restart recovery](../plans/2026-08-20-character-state-process-restart-recovery.md)); this drill still covers backup/restore, not that rematerialization proof;
- SQL migration apply remains CLI-only through `metin2-migrate` — see [migration apply runbook](migration-apply-runbook.md).

## Endpoint inventory

| Store | Kind | Manifest / format | Backup | Backup validate | Restore |
| --- | --- | --- | --- | --- | --- |
| account | directory | `account-backup-manifest.json` / `go-metin2-account-backup-v1` | `POST /local/account-store/backup` | `POST /local/account-store/backup/validate` | `POST /local/account-store/restore` |
| login tickets | directory | `login-ticket-backup-manifest.json` / `go-metin2-login-ticket-backup-v1` | `POST /local/login-tickets/backup` | `POST /local/login-tickets/backup/validate` | `POST /local/login-tickets/restore` |
| item templates | file path | `item-template-backup-manifest.json` / `go-metin2-item-template-backup-v1` | `POST /local/item-templates/backup` | `POST /local/item-templates/backup/validate` | `POST /local/item-templates/restore` |
| interactions | file path | `interaction-backup-manifest.json` / `go-metin2-interaction-backup-v1` | `POST /local/interaction-store/backup` | `POST /local/interaction-store/backup/validate` | `POST /local/interaction-store/restore` |
| static actors | file path | `static-actor-backup-manifest.json` / `go-metin2-static-actor-backup-v1` | `POST /local/static-actors/backup` | `POST /local/static-actors/backup/validate` | `POST /local/static-actors/restore` |
| quest state | file path | `quest-state-backup-manifest.json` / `go-metin2-quest-state-backup-v1` | `POST /local/quest-state/backup` | `POST /local/quest-state/backup/validate` | `POST /local/quest-state/restore` |

Related helpers:

- `GET /local/runtime-config` — confirms active `persistence.*` paths before any backup/restore
- `GET /local/persistence/status` — aggregate validity, `live_selected_character_count`, per-store `backup_manifest`, and `restore_blocked_by_live_sessions`
- per-store `.../validate` and `.../crash-temps/cleanup` — optional triage before backup; static-actor validate/cleanup use the `/local/static-actor-store/...` prefix
- `metin2-migrate backup-restore-drill --runtime-config <path|-> --build-info <path|-> [--authd-ops-base-url http://127.0.0.1:6061]` — read-only printer that turns retained runtime-config + build-info snapshots into the path-aware curl / aside-rename script for the lab `/var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/` tree without executing backup or restore. The printed script retains both-daemon build-info (`gamed` via `--ops-base-url`, `authd` via `--authd-ops-base-url`), `runtime-config.json` / `persistence-status-*.json`, a `notes.md` stub, uses lab store subdirectory names (`accounts`, `interaction-store`, ...), and includes the optional store validate + crash-temp cleanup triage before backup; if an operator runs that printed section, `crash-temps/cleanup` mutates only hidden crash-temp residue after validate.

```bash
metin2-migrate version > /tmp/build-info.json
curl -sS http://127.0.0.1:6060/local/runtime-config \
  | go run ./cmd/metin2-migrate backup-restore-drill \
      --runtime-config - \
      --build-info /tmp/build-info.json
```

Request bodies:

- backup: `{"dst_dir":"<empty directory outside the live store>"}`
- backup validate / restore: `{"src_dir":"<manifested backup directory>"}`

Bodies over 4 KiB fail with `413`. Malformed JSON or blank paths fail with `400`. Contract/validation failures fail with `409`. Non-loopback callers fail with `403`. Wrong methods fail with `405`.

## Persistence layout requirement

Directory stores (`account_store_dir`, `login_ticket_store_dir`) already own their own trees.

File-path stores restore into `filepath.Dir(snapshotPath)`, not “just the JSON file”. That parent directory must be empty before restore, so it may contain only the restored snapshot and the restored backup manifest afterward. The default zero-config temp layout places several `go-metin2-server-*.json` files in the same parent temp directory; that layout is fine for bootstrap smoke tests but is hostile to multi-store restore because emptying one parent would wipe sibling stores.

For any real backup/restore drill, give each file-backed store a dedicated parent directory before starting the daemons, for example:

```text
/var/metin2/accounts/                 # account_store_dir
/var/metin2/login-tickets/            # login_ticket_store_dir
/var/metin2/item-templates/item-templates.json
/var/metin2/interactions/interaction-definitions.json
/var/metin2/static-actors/static-actors.json
/var/metin2/quest-state/quest-state.json
```

Confirm the running daemon with `GET /local/runtime-config` before continuing.

## Preflight

1. Drain selected-character game sessions. Prefer pausing login traffic or stopping `authd` when restoring accounts or login tickets.
2. Confirm ops reachability on loopback:

```bash
curl -sS http://127.0.0.1:6060/healthz
curl -sS http://127.0.0.1:6060/local/runtime-config
curl -sS http://127.0.0.1:6060/local/persistence/status
```

3. Ready-to-restore status shape:

- `ok: true`
- `live_selected_character_count: 0`
- every included store has `valid: true` and `restore_blocked_by_live_sessions: false`

If crash-temp residue is the only complaint, clean with the matching `.../crash-temps/cleanup` endpoint before backup. Do not treat crash-temp cleanup as enough preparation for restore: committed snapshots and any active `*-backup-manifest.json` also make destinations non-empty.

Optional explicit triage sequence (also emitted by `metin2-migrate backup-restore-drill` after the aggregate status check):

```bash
curl -sS -X POST http://127.0.0.1:6060/local/account-store/validate
curl -sS -X POST http://127.0.0.1:6060/local/account-store/crash-temps/cleanup
curl -sS -X POST http://127.0.0.1:6060/local/login-tickets/validate
curl -sS -X POST http://127.0.0.1:6060/local/login-tickets/crash-temps/cleanup
curl -sS -X POST http://127.0.0.1:6060/local/item-templates/validate
curl -sS -X POST http://127.0.0.1:6060/local/item-templates/crash-temps/cleanup
curl -sS -X POST http://127.0.0.1:6060/local/interaction-store/validate
curl -sS -X POST http://127.0.0.1:6060/local/interaction-store/crash-temps/cleanup
curl -sS -X POST http://127.0.0.1:6060/local/static-actor-store/validate
curl -sS -X POST http://127.0.0.1:6060/local/static-actor-store/crash-temps/cleanup
curl -sS -X POST http://127.0.0.1:6060/local/quest-state/validate
curl -sS -X POST http://127.0.0.1:6060/local/quest-state/crash-temps/cleanup
curl -sS http://127.0.0.1:6060/local/persistence/status
```

## Backup

Backup destinations must be empty and must not equal or nest under the live store (lexical and symlink-resolved). Capture order prefers durable player state, then ephemeral handoff, then authored content dependencies:

1. account
2. login tickets
3. item templates
4. interactions
5. static actors
6. quest state

```bash
TS=$(date -u +%Y%m%dT%H%M%SZ)
COMMIT12=$(curl -sS http://127.0.0.1:6060/local/build-info | python3 -c 'import json,sys; print(json.load(sys.stdin)["commit"][:12])')
BASE=/var/metin2/backups/${TS}-${COMMIT12}
mkdir -p "$BASE"/{accounts,login-tickets,item-templates,interaction-store,static-actors,quest-state}
curl -sS http://127.0.0.1:6060/local/runtime-config > "$BASE/runtime-config.json"
curl -sS http://127.0.0.1:6060/local/persistence/status > "$BASE/persistence-status-before.json"

curl -sS -X POST http://127.0.0.1:6060/local/account-store/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/accounts\"}"

curl -sS -X POST http://127.0.0.1:6060/local/login-tickets/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/login-tickets\"}"

curl -sS -X POST http://127.0.0.1:6060/local/item-templates/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/item-templates\"}"

curl -sS -X POST http://127.0.0.1:6060/local/interaction-store/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/interaction-store\"}"

curl -sS -X POST http://127.0.0.1:6060/local/static-actors/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/static-actors\"}"

curl -sS -X POST http://127.0.0.1:6060/local/quest-state/backup \
  -H 'Content-Type: application/json' \
  -d "{\"dst_dir\":\"$BASE/quest-state\"}"
```

Retain the whole `$BASE` tree as the drill artifact set, matching [lab deployment topology](lab-deployment-topology.md). Each successful backup writes its store-specific manifest next to the copied snapshot payload.

## Validate before emptying anything

Dry-run every backup source before renaming live stores aside:

```bash
curl -sS -X POST http://127.0.0.1:6060/local/account-store/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/accounts\"}"
curl -sS -X POST http://127.0.0.1:6060/local/login-tickets/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/login-tickets\"}"
curl -sS -X POST http://127.0.0.1:6060/local/item-templates/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/item-templates\"}"
curl -sS -X POST http://127.0.0.1:6060/local/interaction-store/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/interaction-store\"}"
curl -sS -X POST http://127.0.0.1:6060/local/static-actors/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/static-actors\"}"
curl -sS -X POST http://127.0.0.1:6060/local/quest-state/backup/validate \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/quest-state\"}"
```

Stop on any `409`. Invalid UTF-8 manifests, checksum/size/coverage drift, untracked visible entries, or symlinked manifests/snapshots fail closed and must not be restored.

## Empty active destinations

Restore refuses non-empty destinations. Practical operator pattern: rename the live tree aside, recreate an empty path at the configured location, then restore. Keep `$BASE` outside those aside trees.

Directory stores:

```bash
# ACCOUNT_STORE_DIR and LOGIN_TICKET_STORE_DIR come from /local/runtime-config
mv "$ACCOUNT_STORE_DIR" "${ACCOUNT_STORE_DIR}.aside-${TS}"
mkdir -p "$ACCOUNT_STORE_DIR"

mv "$LOGIN_TICKET_STORE_DIR" "${LOGIN_TICKET_STORE_DIR}.aside-${TS}"
mkdir -p "$LOGIN_TICKET_STORE_DIR"
```

File-path stores (dedicated parents only):

```bash
# Example for item templates; repeat for interactions, static actors, and quest state.
PARENT=$(dirname "$ITEM_TEMPLATE_STORE_PATH")
mv "$PARENT" "${PARENT}.aside-${TS}"
mkdir -p "$PARENT"
```

Re-check:

```bash
curl -sS http://127.0.0.1:6060/local/persistence/status
```

`live_selected_character_count` must still be `0`. After aside-rename, stores should validate as empty (or missing committed snapshots treated as empty authored/quest stores).

## Restore

Restore order prefers authored dependencies and live index reloads before durable account/ticket replacement:

1. item templates — reloads the in-memory template index
2. interactions — reloads the live interaction-definition index
3. static actors — reloads the shared-world static-actor set
4. quest state
5. account
6. login tickets

```bash
curl -sS -X POST http://127.0.0.1:6060/local/item-templates/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/item-templates\"}"
curl -sS -X POST http://127.0.0.1:6060/local/interaction-store/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/interaction-store\"}"
curl -sS -X POST http://127.0.0.1:6060/local/static-actors/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/static-actors\"}"
curl -sS -X POST http://127.0.0.1:6060/local/quest-state/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/quest-state\"}"
curl -sS -X POST http://127.0.0.1:6060/local/account-store/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/accounts\"}"
curl -sS -X POST http://127.0.0.1:6060/local/login-tickets/restore \
  -H 'Content-Type: application/json' -d "{\"src_dir\":\"$BASE/login-tickets\"}"
```

## Post-restore checks

```bash
curl -sS http://127.0.0.1:6060/local/persistence/status > "$BASE/persistence-status-after.json"
```

Expected:

- `ok: true`
- `live_selected_character_count: 0`
- every store `valid: true` and `restore_blocked_by_live_sessions: false`
- every store `backup_manifest.present: true` with the matching `go-metin2-*-backup-v1` format
- summaries match the validated backup responses

Then exercise the PvE vertical that depends on the restored stores: login → map → mob/NPC interaction → inventory/gold continuity after reconnect. Keep the aside trees until that smoke check passes; delete them only under a deployment-specific retention policy.

## Use with migration apply

When a migration window also needs SQL catalog apply, treat this drill as the file-store half of backup validation:

1. complete `metin2-migrate` catalog / ledger-snapshot / plan-artifact / apply-preflight as documented in [migration-apply-runbook.md](migration-apply-runbook.md)
2. run this file-store backup + backup/validate sequence (restore is optional unless the window is a recovery drill)
3. retain deployment-specific DB backup evidence outside this repo
4. only then run `metin2-migrate apply --apply-preflight ... --lock-file ... --audit-file ...`

`apply-preflight` does not substitute for file-store backup validation.

## Failure handling

- If `/local/persistence/status` reports live sessions, stop and drain before restore.
- If backup/validate fails, keep the live store untouched and inspect the backup tree; do not empty destinations.
- If restore fails mid-store, leave later stores unrestored, keep aside trees, and use the failed store's aside path plus `$BASE` for diagnosis. Partial restores are not auto-rolled across stores.
- If a restored store later mutates through normal saves, the restored `*-backup-manifest.json` is removed so live state stops claiming to be the exact backup source.
- If file stores share one parent directory, stop and re-home them before retrying; do not invent a merge restore.

## Anti-goals

Do not use this runbook to justify:

- remote or non-loopback admin access
- GET/PUT restore endpoints
- online merge while selected-character sessions are live
- daemon startup auto-restore
- treating content-bundle import as a substitute for manifested per-store backup/restore
- restoring into backup trees or backing up into live store trees
- claiming ground-item / ground-gold durability across restart
- claiming account/character/item/quest/content repositories are DB-backed
- daemon-local `/local/db/migrations/apply`
