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

The current bootstrap runtime uses two small JSON-backed stores before a compatibility-grade database exists:

- `internal/accountstore` stores durable account snapshots.
- `internal/loginticket` stores one-shot authd-to-gamed login tickets.

The bootstrap file stores intentionally fail closed on unknown top-level JSON fields and trailing JSON values. The account store validates the persisted login identity, rejecting empty or mismatched snapshot logins instead of trusting only the filename, and validates persisted character identity plus item/equipment/quickslot payloads before accepting a snapshot. Its deterministic account listing boundary scans only committed hex-login `.json` snapshots, ignores leftover hidden temp files from interrupted writes, returns missing directories as an empty store, sorts by normalized login, and fails closed on corrupt or filename-mismatched committed snapshots. The login-ticket store uses the same strict decode and character payload validation boundary for ticket files: duplicate character IDs/names, malformed inventory, duplicate equipment slots, and malformed quickslots return `ErrInvalidTicket`, and a failed consume leaves the ticket file in place for inspection instead of deleting possibly corrupted handoff state. The static-actor store also applies that strict decode boundary to restored world/content snapshots before validating actors, interaction metadata, spawn-group refs, and reward descriptors.

Writes are committed through same-directory temp files, synced before rename, and followed by a directory sync after rename. Destructive login-ticket consumes also sync the store directory after deleting the consumed ticket, so successful authd-to-gamed handoff removal is part of the crash-safety boundary instead of only the issue/write path. Account-listing behavior therefore matches that crash-safety model: incomplete `.account-*.json` temp files are not treated as restorable accounts, while malformed committed snapshots stop the listing so future backup/migration tooling cannot silently skip bad durable state. This makes the current JSON stores more crash-tolerant on normal local filesystems while preserving the intentionally simple bootstrap format.

The account store also has narrow backup and restore primitives for future operator/migration tooling. `FileStore.BackupTo(dstDir)` validates the source through the same deterministic `List()` path, copies only committed snapshots into an empty destination directory outside the active account-store directory, omits crash temp files, and fails closed if any committed source snapshot is corrupt or filename-mismatched. It also writes a deterministic `account-backup-manifest.json` containing the backup format string, copied snapshot summary, per-account filenames, byte sizes, and SHA-256 checksums so operators have a stable audit artifact before restore/migration work. `FileStore.RestoreFrom(srcDir)` and `FileStore.ValidateBackupFrom(srcDir)` now require that manifest before accepting a source directory, apply the same committed-snapshot validation before restore/preflight, ignore the manifest as metadata rather than an account snapshot, omit crash temp files from the source, reject a missing restore source or missing manifest explicitly, and refuse to merge restore output into a non-empty destination. Both sides require an empty destination so operator recovery cannot silently blend stale files with a validated snapshot set, backups refuse `dst_dir` values that lexically or symlink-resolve equal to or nested under the live store so the backup scan cannot copy its own in-progress output, restores refuse destinations that lexically or symlink-resolve equal to or nested under the backup source so recovery cannot write a replacement store into the tree being read, and manually assembled snapshot directories must first be converted into a real backup with a manifest before they can be restored.

For safer on-box checks before a backup, restore, or migration runbook, `FileStore.Validate()` exposes the same committed-snapshot validation as a read-only summary: account count, character count, and deterministic login list. The shipped `gamed` ops mux wires that primitive to loopback-only `POST /local/account-store/validate`, which returns the summary on success and `409` when the durable account snapshot set fails validation. The endpoint does not mutate files and is intentionally an operator/debug surface, not a gameplay or remote admin API.

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
