# CLI backup-tree-status contract freeze — 2026-09-06

## Objective

Freeze a read-only `metin2-migrate backup-tree-status` inspector for a
retained `backup-restore-drill` tree so operators can re-check the eight
manifested FileStore backups as one metadata-only evidence artifact
**without** restoring, emptying live stores, opening a database, or
trusting a hand-walk of `$BASE/<store>/`.

This freeze does **not** invent automatic backup/restore execution, a
stock production driver, loopback ops mutation, remote admin, or any
claim that a present valid backup tree proves live FileStore / DB row
state after a later restore.

## Why docs-first

Track E tip chain through `apply-lock-aside-status` is Done
([CLI apply-lock-aside-status contract freeze](2026-09-05-cli-apply-lock-aside-status-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`catalog-status`, `plan-artifact-status`,
`ledger-snapshot-status`, `apply-preflight-status`, `apply-lock-status`,
`apply-lock-aside-status`, `apply-audit-status`, `import-export-status`,
`synthesize-wipe-export-status`, `export-tree-status`,
`export-tree-status-status`).

`backup-restore-drill` already creates
`/var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/` with eight lab store
subdirs (`accounts`, `login-tickets`, `item-templates`,
`interaction-store`, `static-actors`, `quest-state`, `ground-items`,
`safebox`) and the printed script already curls per-store
`/local/*/backup/validate` **while gamed is up**. After the tree is
copied aside, archived, or the daemon is down, there is no small CLI
command that re-validates those manifested backups as one tree — the
export analog is `export-tree-status`.

Opening RED without freezing:

- exact command / flag names,
- outer status envelope + per-store checksum / count fields,
- which eight lab subdirs and dummy snapshot basenames GREEN must use,
- how missing vs invalid store children behave,
- whether `ValidateBackupFrom` may restore or open a database,
- which printed `backup-restore-drill` line adopts the new inspector,
- how the hermetic HTTP proof must put `metin2-migrate` on `PATH`,

would invent operator-facing incident-review semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate backup-tree-status \
  --backup-tree <absolute-path> \
  [--require-stores-complete]
```

Rules:

1. Requires `--backup-tree`; extra positional arguments are usage errors
   (exit `2`).
2. `--backup-tree` must be an absolute cleaned path (same absolute-path
   policy as `export-tree-status` / `import-export-drill`). stdin (`-`)
   is **not** accepted.
3. `--require-stores-complete` is independently opt-in (boolean; default
   false).
4. Usage text lists the command beside `backup-restore-drill` /
   `export-tree-status` and lists `--require-stores-complete` beside
   `--backup-tree`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, HTTP, backup, restore,
   aside-rename, crash-temp cleanup, lock reservation, artifact
   deletion, or daemon mutation.
7. Never emits DSNs, executable SQL, snapshot payloads, login names,
   login keys, tickets, character names, vnums, vids, or actor ids.

### B. Tree presence and fail-closed I/O

1. Returns success with outer `present: false` (and no `stores`) when
   the path is absent (`os.ErrNotExist` after `Lstat`).
2. Rejects a present symlink or non-directory path with exit `1`, a
   short stderr reason, and **no** stdout status JSON.
3. Other `Lstat` errors fail closed the same way.
4. When present, inspects the eight lab store subdirs in this **fixed
   order** (same names and order as `backup-restore-drill` `mkdir -p`
   / backup curls):

   1. `accounts`
   2. `login-tickets`
   3. `item-templates`
   4. `interaction-store`
   5. `static-actors`
   6. `quest-state`
   7. `ground-items`
   8. `safebox`

5. Missing child store subdirs are reported as `present: false` /
   `valid: false` inside the store entry and do **not** fail the
   command by themselves.
6. A present store child that is a symlink, a non-directory, or that
   fails the matching `ValidateBackupFrom` contract (missing manifest,
   wrong format, checksum mismatch, untracked visible entry, snapshot
   basename drift, invalid UTF-8, unknown fields, trailing JSON) fails
   closed with exit `1`, a short stderr reason that names the store
   kind, and **no** stdout status JSON.
7. Hidden crash-temp residue already accepted by `ValidateBackupFrom`
   does **not** fail the command; GREEN reports `crash_temp_count`
   from the returned summary.

### C. Per-store validation seam (no restore)

GREEN must reuse each store's existing
`(*FileStore).ValidateBackupFrom(srcDir)` (ground items:
`worldruntime.NewGroundItemFileStore(...).ValidateBackupFrom`). That
seam already checksums manifested snapshots, rejects symlinks, and
does **not** restore into a live destination.

Constructor / dummy snapshot path for the inspecting FileStore:

| kind | constructor | inspecting store path |
| --- | --- | --- |
| `accounts` | `accountstore.NewFileStore(subdir)` | the store subdir itself |
| `login-tickets` | `loginticket.NewFileStore(subdir)` | the store subdir itself |
| `item-templates` | `itemstore.NewFileStore(join(subdir, "item-templates.json"))` | conventional lab snapshot basename |
| `interaction-store` | `interactionstore.NewFileStore(join(subdir, "interaction-definitions.json"))` | conventional lab snapshot basename |
| `static-actors` | `staticstore.NewFileStore(join(subdir, "static-actors.json"))` | conventional lab snapshot basename |
| `quest-state` | `queststate.NewFileStore(join(subdir, "quest-state.json"))` | conventional lab snapshot basename |
| `ground-items` | `worldruntime.NewGroundItemFileStore(join(subdir, "ground-items.json"))` | conventional lab snapshot basename |
| `safebox` | `safeboxstore.NewFileStore(join(subdir, "safebox.json"))` | conventional lab snapshot basename |

Those dummy file-path basenames are the lab topology /
`backup-restore-drill` names. They do **not** need to exist as live
gamed stores; `ValidateBackupFrom` validates `srcDir` and only uses
`filepath.Base(s.path)` to match a manifested snapshot filename.

A backup whose manifested snapshot basename is **not** the conventional
lab name fails closed. This inspector is for retained
`backup-restore-drill` / runbook trees, not arbitrary operator
directories.

Empty manifested backups (valid manifest, `files: []`, no snapshot)
remain valid, matching FileStore backup of an empty store.

### D. Successful outer envelope

```json
{
  "format": "go-metin2-backup-tree-status-v1",
  "present": true,
  "backup_tree": "/var/metin2/backups/20260906T120000Z-abcdef012345",
  "store_count": 8,
  "store_present_count": 8,
  "stores_complete": true,
  "stores": [
    {
      "kind": "accounts",
      "present": true,
      "valid": true,
      "path": "accounts",
      "manifest_filename": "account-backup-manifest.json",
      "format": "go-metin2-account-backup-v1",
      "manifest_sha256": "...",
      "crash_temp_count": 0,
      "account_count": 1,
      "character_count": 1
    }
  ]
}
```

When the path is absent:

```json
{
  "format": "go-metin2-backup-tree-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. `backup_tree` is the
normalized absolute path. `path` on each store is the slash-normalized
relative subdir (`accounts`, not an absolute path).

`manifest_sha256` is computed over the **exact retained manifest file
bytes** after `ValidateBackupFrom` succeeds. `manifest_filename` /
`format` are the store package constants:

| kind | manifest filename | format |
| --- | --- | --- |
| `accounts` | `account-backup-manifest.json` | `go-metin2-account-backup-v1` |
| `login-tickets` | `login-ticket-backup-manifest.json` | `go-metin2-login-ticket-backup-v1` |
| `item-templates` | `item-template-backup-manifest.json` | `go-metin2-item-template-backup-v1` |
| `interaction-store` | `interaction-backup-manifest.json` | `go-metin2-interaction-backup-v1` |
| `static-actors` | `static-actor-backup-manifest.json` | `go-metin2-static-actor-backup-v1` |
| `quest-state` | `quest-state-backup-manifest.json` | `go-metin2-quest-state-backup-v1` |
| `ground-items` | `ground-item-backup-manifest.json` | `go-metin2-ground-item-backup-v1` |
| `safebox` | `safebox-backup-manifest.json` | `go-metin2-safebox-backup-v1` |

Count fields are additive `omitempty` integers copied from the
`ValidateBackupFrom` summary **without** identity slices:

- `accounts`: `account_count`, `character_count`
- `login-tickets`: `ticket_count`, `character_count`
- `item-templates`: `template_count`
- `interaction-store`: `definition_count`
- `static-actors`: `actor_count`
- `quest-state`: `flag_count`
- `ground-items`: `ground_item_count`
- `safebox`: `character_count`, `cell_count`

Do **not** copy `logins`, `login_keys`, `vnums`, `vids`, `actor_ids`,
`actor_names`, `characters`, `quest_refs`, `flag_keys`,
`character_keys`, `definition_keys`, or crash-temp **filenames**.

`store_count` is always `8` on a present tree. `store_present_count` is
how many store entries have `present: true` (and therefore `valid:
true`, because invalid present children fail closed before JSON).
`stores_complete` is true only when `store_present_count == 8`.

Correlation files (`runtime-config.json`, `persistence-status-*.json`,
`notes.md`, daemon logs / build-info) are **out of scope** for
`stores_complete`. They stay operator-checklist artifacts.

A `go-metin2-backup-tree-status-v1` file (this command's own output)
is the wrong input and is not a backup tree directory.

### E. `--require-stores-complete`

When selected:

- absent tree → exit `1`, empty stdout, stderr names the failed gate /
  absent tree
- present tree with `stores_complete: false` → exit `1`, empty stdout,
  stderr names the failed gate

Ungated missing-tree `present: false` remains exit `0`.

### F. Printer wiring (same GREEN as the inspector)

GREEN must add a matching `backup-tree-status` redirect **immediately
after** the existing eight-store backup/validate curls and **before**
aside-rename / restore, so a bad tree fails closed under `set -eu`
before live destinations are emptied:

```sh
echo '== backup-tree status =='
metin2-migrate backup-tree-status --backup-tree "$BASE" \
  --require-stores-complete \
  > "$BASE/backup-tree-status.json"
```

`backup-restore-drill` remains print-only and still does not execute
status / backup / restore itself.

The require flag is correct here: the printed script inspects the tree
it just backed up, and operators should not aside-rename live stores
when any manifested backup is incomplete.

Printed scripts still use `set -eu`, so a failed `backup-tree-status`
inspect fails the drill.

Hermetic `/bin/sh` proof currently executes that printed script
**without** a `metin2-migrate` binary on `PATH` (curl / `mv` / `mkdir`
only). GREEN **must** update
`TestBackupRestoreDrillHTTPExecutesAgainstDrainedGamedOps` to build
`./cmd/metin2-migrate` onto `PATH` the same way
`TestExportQuarantineDrillHTTP…` already does, then assert
`$BASE/backup-tree-status.json` is a valid
`go-metin2-backup-tree-status-v1` with `stores_complete: true`.

Do **not** list `backup-tree-status.json` in
[lab deployment topology](../workflow/lab-deployment-topology.md) or
add a working CLI example until GREEN actually produces the command.

### G. Explicit non-goals

- restoring / aside-renaming / crash-temp cleanup from the status
  command
- opening a database or emitting DSNs / executable SQL
- hashing correlation files (`runtime-config.json`,
  `persistence-status-*.json`, `notes.md`, daemon logs)
- requiring those correlation files for `stores_complete`
- accepting relative `--backup-tree` paths or stdin
- a loopback `GET /local/backup-tree-status` endpoint
- remote admin / secrets in git / metrics
- stock production DB driver registration
- automatic / scheduled execution of printed backup / restore scripts
- claiming `stores_complete` proves live gamed FileStores currently
  match the tree (operators still compare `/local/persistence/status`
  after restore)
- `--require-no-crash-temps` (follow-up, not this freeze)
- a `backup-tree-status-status` inspector for the retained JSON
  (follow-up, not this freeze)
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/backup_tree_status.go` (new)
- `internal/migratecli/backup_tree_status_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/backup_restore_drill.go` (status redirect)
- `internal/migratecli/backup_restore_drill_test.go`
- `internal/minimal/backup_restore_drill_http_test.go` (`PATH` + retained
  `backup-tree-status.json`)
- `docs/development.md`
- `docs/workflow/file-store-backup-restore-drill.md`
- `docs/workflow/lab-deployment-topology.md` (list
  `backup-tree-status.json` only once GREEN produces it)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing backup-tree path → outer `present: false`, no DB open, no
  restore
- present tree with all eight valid (including empty) store backups →
  outer `present: true` + `stores_complete: true` + per-store checksums
  / counts
- missing one store subdir → ungated `stores_complete: false`, exit `0`
- `--require-stores-complete` with absent tree or incomplete stores →
  exit `1`, empty stdout
- present invalid store (checksum mismatch / missing manifest /
  symlink subdir / snapshot basename drift) → exit `1`, empty stdout
- usage / unknown-command mention `backup-tree-status`
- stdout omits logins, tickets, DSNs, SQL
- `backup-restore-drill` printer emits the matching
  `backup-tree-status --require-stores-complete` redirect after backup
  validate and before aside-rename

Hermetic HTTP proof in `internal/minimal`:

- printed-script execution still round-trips seeded account + safebox
- retained `backup-tree-status.json` exists with
  `stores_complete: true` and an accounts `account_count` of `1`

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'BackupTreeStatus|BackupRestoreDrillPrints|RejectsUnknownCommandMentionsBackupTreeStatus' -count=1
go test ./internal/minimal -run 'BackupRestoreDrillHTTP' -count=1
gofmt -l internal/migratecli/*.go internal/minimal/backup_restore_drill_http_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- Read-only `metin2-migrate backup-tree-status --backup-tree <absolute-path>
  [--require-stores-complete]` re-validates a retained `backup-restore-drill`
  tree without restoring, emptying live stores, opening a database, or
  walking live gamed FileStores.
- Outer envelope is `go-metin2-backup-tree-status-v1`; missing path is
  ungated `present: false`; present trees walk the eight lab store subdirs
  through existing `ValidateBackupFrom` seams and report checksums / counts
  without identity slices.
- `backup-restore-drill` prints a matching
  `backup-tree-status --require-stores-complete` redirect to
  `$BASE/backup-tree-status.json` after backup/validate and before
  aside-rename / restore.
- Hermetic `/bin/sh` backup-restore drill HTTP proof now puts
  `metin2-migrate` on `PATH` and asserts that retained JSON.
- Upsert / auto-run / stock production driver / cascade-delete /
  `backup-tree-status-status` / `--require-no-crash-temps` remain deferred.

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope /
  eight-store walk / `ValidateBackupFrom` seams / printer wiring /
  hermetic `PATH` requirement
- Track E / migration-contract point at this freeze as the next GREEN
  target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not restore, aside-rename, or open a database from the status
  command.
- Do not auto-run `backup-restore-drill` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not list `backup-tree-status.json` in lab topology or add a
  working CLI example until GREEN actually produces the command.
  GREEN now owns that listing, the backup-restore-drill redirect, and
  the hermetic `/bin/sh` PATH proof.
