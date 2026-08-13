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

### pprof / local ops

- `authd`: `METIN2_AUTHD_PPROF_ADDR` (default `127.0.0.1:6061`)
- `gamed`: `METIN2_GAMED_PPROF_ADDR` (default `127.0.0.1:6060`)
- global override: `METIN2_PPROF_ADDR`

The ops listener also carries loopback-only recovery/debug endpoints under `/local/*`, so startup rejects wildcard or non-loopback pprof binds such as `:6060`, `0.0.0.0:6060`, `[::]:6060`, or a remote hostname. If remote profiling is needed, keep the daemon bound to loopback and use an SSH tunnel or another explicit local transport instead of exposing the ops mux directly.

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

### Bootstrap persistence paths

The file-backed bootstrap stores can now be pointed at explicit paths without changing code. Service-specific overrides take precedence over the global value for each store, matching the existing address and visibility configuration rules:

- login-ticket handoff store directory:
  - `METIN2_LOGIN_TICKET_STORE_DIR`
  - `METIN2_AUTHD_LOGIN_TICKET_STORE_DIR`
  - `METIN2_GAMED_LOGIN_TICKET_STORE_DIR`
- durable account snapshot store directory:
  - `METIN2_ACCOUNT_STORE_DIR`
  - `METIN2_AUTHD_ACCOUNT_STORE_DIR`
  - `METIN2_GAMED_ACCOUNT_STORE_DIR`
- authored static-actor snapshot path:
  - `METIN2_STATIC_ACTOR_STORE_PATH`
  - `METIN2_GAMED_STATIC_ACTOR_STORE_PATH`
- authored interaction-definition snapshot path:
  - `METIN2_INTERACTION_STORE_PATH`
  - `METIN2_GAMED_INTERACTION_STORE_PATH`
- authored item-template snapshot path:
  - `METIN2_ITEM_TEMPLATE_STORE_PATH`
  - `METIN2_GAMED_ITEM_TEMPLATE_STORE_PATH`

Default paths remain under the process temp directory (`go-metin2-server-login-tickets`, `go-metin2-server-accounts`, `go-metin2-server-static-actors.json`, `go-metin2-server-interaction-definitions.json`, and `go-metin2-server-item-templates.json`) so local bootstrap runs remain zero-config. For persistent QA runs or backup/restore drills, set explicit store directories/paths before starting both daemons; `authd` and `gamed` must share the same login-ticket and account-store locations when exercising the one-shot ticket handoff.

Startup now validates the effective bootstrap persistence layout before daemon runtime wiring. On `authd`, the login-ticket and account-store paths must both be non-empty directory stores, must not point at existing regular files, must not be symlink roots, and must not overlap, so issuing a login key cannot accidentally write into the same tree as durable account snapshots. On `gamed`, directory-backed stores (`login_ticket_store_dir`, `account_store_dir`) must be non-empty, must not point at existing regular files, must not be symlink roots (including dangling symlinks), and must own distinct directory trees; file-backed stores (`static_actor_store_path`, `interaction_store_path`, `item_template_store_path`) must be non-empty, must not point at existing directories, must not share the same file path, and must not live inside either directory-backed store. The overlap check uses cleaned absolute paths, lexical path placement, and existing symlink resolution for the file-backed stores, while directory-backed store roots are required to be direct filesystem directories so backup, restore, cleanup, and ticket-consume directory syncs cannot silently operate through an alias outside the operator-visible store boundary.

`GET /local/runtime-config` includes the active persistence paths under `persistence`, so local operators can verify which JSON stores a running `gamed` instance will validate, back up, restore, or mutate before invoking the local-only persistence endpoints.

### Database migration preflight config

The database-backed runtime is still future work, but `gamed` can now point the read-only migration-status preflight at an explicitly configured `schema_migrations` ledger through `database/sql`:

- `METIN2_DB_DRIVER`
- `METIN2_GAMED_DB_DRIVER`
- `METIN2_DB_DSN`
- `METIN2_GAMED_DB_DSN`

Both driver and DSN must be empty to keep DB preflight disabled, or both must be set. Partial values fail daemon startup instead of silently returning the embedded empty-ledger migration plan. The project does not yet ship a DB driver dependency or select a production database engine, so operators must not treat this as a finished DB-backed runtime. `/local/runtime-config` reports only whether a DSN is configured plus the driver name; it intentionally never exposes the DSN value. For offline preflight, `GET /local/db/migrations/ledger-snapshot` exports the running daemon's configured ledger metadata as strict `go-metin2-schema-migrations-ledger-v1` JSON (`version` / `name` / `up_sha256` only), returning an explicit empty snapshot when DB preflight is disabled. `POST /local/db/migrations/plan-from-ledger-snapshot?target_version=N` accepts that same metadata-only snapshot shape and returns the dry-run plan without opening the configured DB or exposing executable SQL.

`db/migrations` also exposes a first programmatic apply primitive for future CLI/repository work. `ApplyCatalogUpToVersion` validates the same catalog/ledger/target boundary used by dry-run planning, rejects rollback targets before opening a transaction, executes only pending up SQL, writes the matching `schema_migrations` row after each successful migration body, and rolls the transaction back if either migration SQL or ledger insertion fails. This primitive is deliberately not registered as a local ops endpoint and is not wired into daemon startup; production use still needs an explicit driver/engine choice, statement-splitting policy if the selected driver requires it, backup/restore runbook, and operator command surface.

### Bootstrap file-backed persistence

The current bootstrap runtime uses several small JSON-backed stores before a compatibility-grade database exists:

- `internal/accountstore` stores durable account snapshots.
- `internal/loginticket` stores one-shot authd-to-gamed login tickets.
- `internal/staticstore` stores authored bootstrap static-actor snapshots.
- `internal/interactionstore` stores authored bootstrap interaction definitions.
- `internal/itemstore` stores authored bootstrap item-template snapshots used by content bundles, merchant previews, and item/equipment runtime policy.

The bootstrap file stores intentionally fail closed on unknown top-level JSON fields and trailing JSON values. The account store validates the persisted login identity, rejecting empty, whitespace-padded, or mismatched snapshot logins instead of trusting only the filename, and validates persisted character identity plus item/equipment/quickslot payloads before accepting a snapshot. Character identity validation requires non-zero character records to carry non-empty, unpadded names and treats repeated all-zero character records as intentional empty select-screen slots, while rejecting any zero-ID slot that still carries non-empty name, VID, location, stats, guild, gold, item, equipment, or quickslot state so deleted-slot residue cannot masquerade as either a real character or a reusable empty slot. Its deterministic account listing boundary scans only committed canonical lowercase hex-login `.json` snapshots, ignores leftover hidden temp files from interrupted writes, returns missing directories as an empty store, sorts by normalized login, and fails closed on symlinked, corrupt, filename-mismatched, non-canonical, or case-variant duplicate committed snapshots. The login-ticket store uses the same strict decode and character payload validation boundary for ticket files: empty or whitespace-padded logins, zero login keys, filename/login-key drift, duplicate non-zero character IDs/names, blank or whitespace-padded non-zero character names, non-empty zero-ID character slots, malformed inventory, duplicate equipment slots, and malformed quickslots return `ErrInvalidTicket`, and a failed consume leaves the ticket file in place for inspection instead of deleting possibly corrupted handoff state. Its deterministic ticket validation boundary scans only committed canonical lowercase 8-digit hex login-key `.json` snapshots, ignores leftover hidden `.ticket-*.json` temp files, returns missing directories as an empty store, sorts by normalized login and login key, rejects symlinked committed ticket snapshots before decoding, and never consumes ticket... [truncated]

Account snapshot writes are committed through same-directory temp files, synced before rename, and followed by a directory sync after rename. Login-ticket issue writes follow the same temp-file sync boundary, then publish the ticket with an exclusive same-directory link so a ticket that appears after the preflight existence check is not overwritten; the store directory is synced after the ticket becomes visible. Destructive login-ticket consumes also sync the store directory after deleting the consumed ticket, so successful authd-to-gamed handoff removal is part of the crash-safety boundary instead of only the issue/write path. Account-listing behavior therefore matches that crash-safety model: incomplete `.account-*.json` temp files are not treated as restorable accounts, while malformed committed snapshots stop the listing so future backup/migration tooling cannot silently skip bad durable state. This makes the current JSON stores more crash-tolerant on normal local filesystems while preserving the intentionally simple bootstrap format.

The account store also has narrow backup and restore primitives for future operator/migration tooling. `FileStore.BackupTo(dstDir)` validates the source through the same deterministic `List()` path, copies only committed snapshots into an empty destination directory outside the active account-store directory, omits crash temp files, and fails closed if any committed source snapshot is corrupt or filename-mismatched. It also writes a deterministic `account-backup-manifest.json` containing the backup format string, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums so operators have a stable audit artifact before restore/migration work. When the active account store already contains restored backup metadata, `BackupTo` validates that active manifest before creating the destination and fails closed on stale or malformed manifest state, so an operator cannot mint a new backup from a store whose own restored-backup marker no longer matches the committed account files. `FileStore.RestoreFrom(srcDir)` and `FileStore.ValidateBackupFrom(srcDir)` now require that manifest before accepting a source directory, apply the same committed-snapshot validation before restore/preflight, ignore the manifest as metadata rather than an account snapshot, reject symlinked manifests, manifested account snapshots, or crash-temp-shaped entries, ignore regular-file crash temp residue as restorable payload while reporting it in dry-run backup summaries, reject crash-temp-shaped directories as untracked backup entries, reject a missing restore source or missing manifest explicitly, and refuse to merge restore output into a non-empty destination. Both sides require an empty destination so operator recovery cannot silently blend stale files with a validated snapshot set, backups refuse `dst_dir` values that lexically or symlink-resolve equal to or nested under the live store so the backup scan cannot copy its own in-progress output, restores refuse destinations that lexically or symlink-resolve equal to or nested under the backup source so recovery cannot write into the source tree being verified, manifest file entries must preserve the exact committed snapshot login casing instead of only matching case-insensitively, manually assembled snapshot directories must first be converted into a real backup with a manifest before they can be restored, committed-copy/manifest-write/final-sync failures roll back files already written into the destination, and later normal account saves remove restored manifest metadata before syncing so mutated live state cannot keep claiming exact backup equivalence.

For safer on-box checks before a backup, restore, login-handoff investigation, content import, or migration runbook, the file stores expose validation and cleanup summaries. `accountstore.FileStore.Validate()` returns account count, character slot count, the number of all-zero empty select-screen slots when present, deterministic login list, and any same-directory `.account-*.json` crash-temp residue through loopback-only `POST /local/account-store/validate`; `accountstore.FileStore.CleanupCrashTempFiles()` validates the committed account snapshot set first, then removes only hidden `.account-*.json` temp residue and syncs the account-store directory through loopback-only `POST /local/account-store/crash-temps/cleanup`. If committed state is corrupt, cleanup fails closed and leaves crash-temp files in place for manual recovery. `loginticket.FileStore.Validate()` returns pending ticket count, deterministic login list, matching login-key list, oldest/newest committed `issued_at` bounds when tickets exist, and any `.ticket-*.json` crash-temp residue through loopback-only `POST /local/login-tickets/validate`, without consuming or deleting handoff tickets. Login-ticket recovery separates inspection from mutation: `CleanupCrashTempFiles()` removes interrupted hidden `.ticket-*.json` temp writes, `PreviewIssuedBefore(cutoff)` reports the committed handoff tickets with `issued_at` strictly older than the operator-supplied cutoff through loopback-only `POST /local/login-tickets/issued-before/preview` without deleting them while preserving the current summary age bounds, and `CleanupIssuedBefore(cutoff)` validates the committed ticket set first and then prunes only those stale handoff tickets through loopback-only `POST /local/login-tickets/issued-before/cleanup` with remaining-ticket age bounds. `staticstore.FileStore.Validate()` and `interactionstore.FileStore.Validate()` add read-only validation summaries for authored static actors and interaction definitions through loopback-only `POST /local/static-actor-store/validate` and `POST /local/interaction-store/validate`: missing committed snapshots are reported as valid empty stores, symlinked committed snapshots are rejected before JSON decoding, committed snapshots are decoded through their existing strict loaders, and same-directory `.static-actors-*.json` / `.interaction-definitions-*.json` temp residue is reported for operator triage. Their `CleanupCrashTempFiles()` paths validate committed content first, remove only their own hidden temp residue, sync the store directory, and are exposed through loopback-only `POST /local/static-actor-store/crash-temps/cleanup` and `POST /local/interaction-store/crash-temps/cleanup`. The aggregate loopback-only `GET /local/persistence/status` endpoint includes account, login-ticket, item-template, static-actor, and interaction stores and keeps checking every store even if one fails, so a corrupt content snapshot does not mask unrelated handoff or account-store state. It also reports active account/item-template backup-manifest presence and manifest paths, letting operators distinguish a restored backup-equivalent store from a normal post-mutation store before choosing a narrower validate, backup, restore, or migration action. These endpoints are shipped on `gamed`, return `409` when committed store state fails validation on mutation/preflight endpoints, and are intentionally operator/debug surfaces, not gameplay or remote admin APIs.

Authored item-template snapshots now follow the same validation posture. `itemstore.FileStore.Validate()` strictly loads the committed `item-templates.json` snapshot when present, rejects symlinked committed snapshots before decoding, treats a missing snapshot as an empty authored-template store that will fall back to built-in bootstrap templates at runtime, reports deterministic template counts and vnums, and includes same-directory `.item-templates-*.json` crash-temp residue while excluding the committed snapshot itself. `FileStore.BackupTo(dstDir)` and `FileStore.ValidateBackupFrom(srcDir)` create and preflight manifest-closed, symlink-free item-template backups with SHA-256-checked snapshot payloads; the backup path now validates any active restored manifest before creating the requested destination, so a store whose current `item-templates.json` has drifted from its restored manifest or whose manifest is a dangling/symlinked file cannot mint a new backup that looks trustworthy; the preflight summary reports ignored `.item-templates-*.json` crash-temp residue so operators can see interrupted writes without treating them as restorable template payload; `FileStore.BackupTo(dstDir)` also treats item-template snapshot save/directory-sync failures as possibly committed and removes both the copied snapshot and any manifest before returning, so a failed backup is not left looking like a valid source for restore. `FileStore.RestoreFrom(srcDir)` restores only into an empty active item-template directory, rejects backup-source-contained destinations, rejects symlinked manifests, manifested snapshots, or crash-temp-shaped entries, preserves committed zero-template snapshots, and writes a fresh manifest alongside the restored snapshot set. Like account saves, the next normal item-template save removes that restored manifest before syncing the directory so backup metadata does not survive after the authored template payload changes. `gamed` exposes these through loopback-only `POST /local/item-templates/validate`, `/local/item-templates/backup`, `/local/item-templates/backup... [truncated]

This is still bootstrap file persistence, not a migration-ready database layer. Future migration/backfill tooling should either emit the exact current schema or introduce an explicit versioned import/quarantine path instead of relying on silent field coercion.

### Database migration catalog

`db/migrations` now owns the first validated migration catalog skeleton for future DB-backed stores. The embedded catalog is intentionally small: `0001_bootstrap_schema_migrations` creates only the schema migration ledger, `0002_account_character_roster` freezes the first schema-only account/character roster boundary, `0003_character_item_state` freezes the first schema-only carried-inventory/equipment/quickslot boundary, `0004_character_quest_state` freezes the standalone bootstrap quest-flag boundary, `0005_item_template_state` plus `0006_item_template_safebox_reject_message` freeze the authored bootstrap item-template boundary, and `0007_auth_login_ticket_handoff` freezes the first schema-only authd-to-gamed login-ticket handoff boundary. These migrations do not imply that accounts, characters, items, quests, login tickets, content, or runtime state are DB-backed yet. The SQL-ledger snapshot helper and loopback `GET /local/db/migrations/ledger-snapshot` endpoint are likewise metadata-only export/preflight surfaces: they copy only `schema_migrations` rows needed for offline planning and never include executable SQL. The first apply primitive is programmatic only: it executes pending up migrations and matching ledger inserts in one transaction after the same catalog/ledger validation, but it is not exposed through the shipped daemons and does not add rollback execution or repository writes. DB preflight configuration is now validated at startup: partial or malformed driver/DSN pairs fail closed, and a configured driver name must already be registered in `database/sql` before `gamed` starts its ops or legacy listeners. The stock project still does not ship a DB driver dependency, so leave DB preflight disabled unless a custom build links the intended driver.

The `schema_migrations` ledger stores `version`, `name`, `up_sha256`, and `applied_at` so a future migrator can verify that an applied historical migration still matches the project-owned SQL body. The account/character roster migration defines `accounts` and `characters` tables for normalized logins, explicit account IDs, four select-screen slots, normalized character names, bootstrap appearance/stat/location/guild/gold fields, and timestamps. The character item-state migration defines `character_inventory_items`, `character_equipment_items`, and `character_quickslots` for the item-bearing selected-character state that is already persisted in bootstrap account snapshots. The character quest-state migration defines `character_quest_flags` for the standalone bootstrap quest-state snapshot and resolves flags to roster character ids before export. The item-template-state migrations define `item_templates` plus display socket/attribute and optional use/equip effect child tables for committed authored item-template snapshots, with the additive safebox-reject column guarded by `anti_safebox`. The auth login-ticket handoff migration defines `auth_login_tickets` for active non-zero login keys, issued timestamps, login identity, empire context, optional consumed timestamp, and a transitional character snapshot JSON payload; it does not change the current JSON ticket store. Broader authored content, exchange/trade state, ground-item handles, and world runtime state remain deliberately out of scope. `internal/accountstore` can project committed JSON snapshots into the roster shape through `ExportAccountCharacterRoster()` and into the item-state shape through `ExportCharacterItemState()`; `internal/loginticket` can project committed pending JSON login tickets through `ExportAuthLoginTicketHandoff()`; `internal/queststate` can project the standalone quest-state snapshot through `ExportCharacterQuestState(...)`; `internal/itemstore` can project committed authored item-template snapshots through `ExportItemTemplateState()`. `gamed` exposes those read-only projections through loopback-only `GET /local/account-store/exports/account-character-roster`, `GET /local/account-store/exports/character-item-state`, `GET /local/login-tickets/exports/auth-login-ticket-handoff`, `GET /local/quest-state/exports/character-quest-state`, and `GET /local/item-templates/exports/item-template-state`. The projections are deterministic, schema-shaped, metadata-bearing (`migration_version` / `migration_name`), and read-only: they do not open a database, emit SQL, apply migrations, consume tickets, or mutate the account, login-ticket, quest-state, or item-template stores, and they fail closed if a committed snapshot would violate the target contract. The `db/migrations` package validates migration file naming, up/down pairing, contiguous versions from `0001`, non-empty UTF-8 SQL bodies, deterministic ordering, project-owned header comments, and the mandatory `migrations.manifest.json` checksum/path manifest. It also exposes read-only dry-run planners: `PlanUpToLatest` / `PlanCatalogUpToLatest` compare already-read ledger rows with the catalog, `PlanToVersion` / `PlanCatalogToVersion` can preview an explicit target version including rollback-to-zero, and `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` / `PlanToVersionFromSQLLedger` add a narrow `database/sql`-compatible reader seam that queries only `version`, `name`, and `up_sha256` from `schema_migrations`, closes rows, and fails closed on query/scan/iteration/close errors. These planners return pending migration metadata without applying SQL or exposing SQL as the plan payload.

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
