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

### Bootstrap file-backed persistence

The current bootstrap runtime uses three small JSON-backed stores before a compatibility-grade database exists:

- `internal/accountstore` stores durable account snapshots.
- `internal/loginticket` stores one-shot authd-to-gamed login tickets.
- `internal/itemstore` stores authored bootstrap item-template snapshots used by content bundles, merchant previews, and item/equipment runtime policy.

The bootstrap file stores intentionally fail closed on unknown top-level JSON fields and trailing JSON values. The account store validates the persisted login identity, rejecting empty or mismatched snapshot logins instead of trusting only the filename, and validates persisted character identity plus item/equipment/quickslot payloads before accepting a snapshot. Its deterministic account listing boundary scans only committed canonical lowercase hex-login `.json` snapshots, ignores leftover hidden temp files from interrupted writes, returns missing directories as an empty store, sorts by normalized login, and fails closed on corrupt, filename-mismatched, non-canonical, or case-variant duplicate committed snapshots. The login-ticket store uses the same strict decode and character payload validation boundary for ticket files: empty logins, zero login keys, filename/login-key drift, duplicate character IDs/names, malformed inventory, duplicate equipment slots, and malformed quickslots return `ErrInvalidTicket`, and a failed consume leaves the ticket file in place for inspection instead of deleting possibly corrupted handoff state. Its deterministic ticket validation boundary scans only committed canonical lowercase 8-digit hex login-key `.json` snapshots, ignores leftover hidden `.ticket-*.json` temp files, returns missing directories as an empty store, sorts by normalized login and login key, and never consumes tickets while summarizing pending handoffs. The static-actor store also applies that strict decode boundary to restored world/content snapshots before validating actors, interaction metadata, spawn-group refs, and reward descriptors.

Account snapshot writes are committed through same-directory temp files, synced before rename, and followed by a directory sync after rename. Login-ticket issue writes follow the same temp-file sync boundary, then publish the ticket with an exclusive same-directory link so a ticket that appears after the preflight existence check is not overwritten; the store directory is synced after the ticket becomes visible. Destructive login-ticket consumes also sync the store directory after deleting the consumed ticket, so successful authd-to-gamed handoff removal is part of the crash-safety boundary instead of only the issue/write path. Account-listing behavior therefore matches that crash-safety model: incomplete `.account-*.json` temp files are not treated as restorable accounts, while malformed committed snapshots stop the listing so future backup/migration tooling cannot silently skip bad durable state. This makes the current JSON stores more crash-tolerant on normal local filesystems while preserving the intentionally simple bootstrap format.

The account store also has narrow backup and restore primitives for future operator/migration tooling. `FileStore.BackupTo(dstDir)` validates the source through the same deterministic `List()` path, copies only committed snapshots into an empty destination directory outside the active account-store directory, omits crash temp files, and fails closed if any committed source snapshot is corrupt or filename-mismatched. It also writes a deterministic `account-backup-manifest.json` containing the backup format string, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums so operators have a stable audit artifact before restore/migration work. `FileStore.RestoreFrom(srcDir)` and `FileStore.ValidateBackupFrom(srcDir)` now require that manifest before accepting a source directory, apply the same committed-snapshot validation before restore/preflight, ignore the manifest as metadata rather than an account snapshot, omit crash temp files from the source, reject a missing restore source or missing manifest explicitly, and refuse to merge restore output into a non-empty destination. Both sides require an empty destination so operator recovery cannot silently blend stale files with a validated snapshot set, backups refuse `dst_dir` values that lexically or symlink-resolve equal to or nested under the live store so the backup scan cannot copy its own in-progress output, restores refuse destinations that lexically or symlink-resolve equal to or nested under the backup source so recovery cannot write a replacement store into the tree being read, manifest file entries must preserve the exact committed snapshot login casing instead of only matching case-insensitively, and manually assembled snapshot directories must first be converted into a real backup with a manifest before they can be restored. When a restored account store later accepts a normal account save, that write removes the restored manifest before syncing the directory so the live store cannot retain stale backup-integrity metadata after mutation.

For safer on-box checks before a backup, restore, login-handoff investigation, or migration runbook, the file stores expose validation and cleanup summaries. `accountstore.FileStore.Validate()` returns account count, character count, deterministic login list, and any same-directory `.account-*.json` crash-temp residue through loopback-only `POST /local/account-store/validate`; `accountstore.FileStore.CleanupCrashTempFiles()` validates the committed account snapshot set first, then removes only hidden `.account-*.json` temp residue and syncs the account-store directory through loopback-only `POST /local/account-store/crash-temps/cleanup`. If committed state is corrupt, cleanup fails closed and leaves crash-temp files in place for manual recovery. `loginticket.FileStore.Validate()` returns pending ticket count, deterministic login list, matching login-key list, and any `.ticket-*.json` crash-temp residue through loopback-only `POST /local/login-tickets/validate`, without consuming or deleting handoff tickets. Login-ticket recovery also has two distinct local cleanup paths: `CleanupCrashTempFiles()` removes interrupted hidden `.ticket-*.json` temp writes, while `CleanupIssuedBefore(cutoff)` validates the committed ticket set first and then prunes only handoff tickets with `issued_at` strictly older than the operator-supplied cutoff through loopback-only `POST /local/login-tickets/issued-before/cleanup`. These endpoints are shipped on `gamed`, return `409` when committed store state fails validation, and are intentionally operator/debug surfaces, not gameplay or remote admin APIs.

Authored item-template snapshots now follow the same validation posture. `itemstore.FileStore.Validate()` strictly loads the committed `item-templates.json` snapshot when present, treats a missing snapshot as an empty authored-template store that will fall back to built-in bootstrap templates at runtime, reports deterministic template counts and vnums, and includes same-directory `.item-templates-*.json` crash-temp residue while excluding the committed snapshot itself. `FileStore.BackupTo(dstDir)` and `FileStore.ValidateBackupFrom(srcDir)` create and preflight manifest-closed item-template backups with SHA-256-checked snapshot payloads; `FileStore.RestoreFrom(srcDir)` restores only into an empty active item-template directory, rejects backup-source-contained destinations, preserves committed zero-template snapshots, and writes a fresh manifest alongside the restored snapshot set. Like account saves, the next normal item-template save removes that restored manifest before syncing the directory so backup metadata does not survive after the authored template payload changes. `gamed` exposes these through loopback-only `POST /local/item-templates/validate`, `/local/item-templates/backup`, `/local/item-templates/backup/validate`, and `/local/item-templates/restore`; malformed committed snapshots, unknown fields, trailing JSON, duplicate vnums, invalid template policy, missing manifests, checksum drift, or unsafe restore destinations return `409` instead of letting operators mistake corrupt authored content for the built-in fallback path.

This is still bootstrap file persistence, not a migration-ready database layer. Future migration/backfill tooling should either emit the exact current schema or introduce an explicit versioned import/quarantine path instead of relying on silent field coercion.

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
