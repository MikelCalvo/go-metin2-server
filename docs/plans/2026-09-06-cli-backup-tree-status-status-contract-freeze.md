# CLI backup-tree-status-status contract freeze — 2026-09-06

## Objective

Freeze a read-only `metin2-migrate backup-tree-status-status` inspector for
retained `backup-tree-status.json` artifacts so operators can re-check
eight-store FileStore backup evidence during incident review or release
packaging **without** walking the original backup-restore tree, restoring,
or opening a database.

This freeze does **not** invent automatic backup/restore execution, a
stock production driver, loopback ops mutation, remote admin, new inner
`go-metin2-backup-tree-status-v1` fields, `--require-no-crash-temps`, or
any claim that a present valid status file proves live FileStore / DB row
state after a later restore.

## Why docs-first

Track E tip chain through `backup-tree-status` is Done
([CLI backup-tree-status contract freeze](2026-09-06-cli-backup-tree-status-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`catalog-status`, `plan-artifact-status`,
`ledger-snapshot-status`, `apply-preflight-status`, `apply-lock-status`,
`apply-lock-aside-status`, `apply-audit-status`, `import-export-status`,
`synthesize-wipe-export-status`, `export-tree-status`,
`export-tree-status-status`, `backup-tree-status`).
`backup-restore-drill` already retains `$BASE/backup-tree-status.json`
after backup/validate, but there is no small command to re-validate that
file by itself after the original tree is archived, copied, or
hand-edited — the export analog is `export-tree-status-status`.

Opening RED without freezing:

- exact command / flag names,
- outer status envelope + checksum field,
- size cap and fail-closed file policy,
- which inner aggregates must recompute from decoded store entries,
- whether the original backup-tree / `ValidateBackupFrom` may be opened,
- which printed `backup-restore-drill` line adopts the new inspector,
- how the hermetic HTTP proof must assert the retained status-status JSON,

would invent operator-facing incident-review semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate backup-tree-status-status \
  --backup-tree-status <path> \
  [--require-stores-complete]
```

Rules:

1. Requires `--backup-tree-status`; extra positional arguments are usage
   errors (exit `2`).
2. The path is a retained `backup-tree-status` JSON file, **not** the
   backup-tree directory. Relative paths are allowed (same file-inspector
   policy as `export-tree-status-status` / `plan-artifact-status`).
   stdin (`-`) is **not** accepted: a path whose name is `-` is a
   missing/regular file named `-`.
3. `--require-stores-complete` is independently opt-in (boolean; default
   false) and uses the **same name and aggregate mapping** as live
   `backup-tree-status`.
4. Usage text lists the command beside `backup-tree-status` /
   `backup-restore-drill` and lists `--require-stores-complete` beside
   `--backup-tree-status`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, HTTP, backup, restore,
   aside-rename, crash-temp cleanup, lock reservation, artifact
   deletion, daemon mutation, filesystem walk of the original
   backup-tree / sibling store subdirs, or `ValidateBackupFrom`.
7. Never emits DSNs, executable SQL, snapshot payloads, login names,
   login keys, tickets, character names, vnums, vids, or actor ids.

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `status`)
   when the path is absent (`os.ErrNotExist` after `Lstat`).
2. Rejects symlink or non-regular paths, oversized files over **128 KiB**
   (same cap as `export-tree-status-status`), invalid UTF-8, empty files,
   malformed JSON, unknown fields (`DisallowUnknownFields`), trailing
   JSON, or an unsupported `format` marker with exit `1`, a short stderr
   reason, and **no** stdout status JSON.
3. Input `format` must be `go-metin2-backup-tree-status-v1`. A
   `go-metin2-backup-tree-status-status-v1` file (this command's own
   output) is the wrong format and must fail closed.
4. `backup_tree_status_sha256` is computed over the **exact retained file
   bytes** so operators can correlate the inspected file with lab notes /
   drill trees.

### C. Inner `present: false` retained snapshots

A valid live inspector snapshot for an absent tree is only:

```json
{
  "format": "go-metin2-backup-tree-status-v1",
  "present": false
}
```

Rules:

1. That shape is success for ungated inspection.
2. Any extra inner fields (`backup_tree`, `store_count`,
   `store_present_count`, `stores_complete`, `stores`) fail closed.
3. Selected `--require-stores-complete` against inner `present: false`
   fails closed with exit `1`, a short stderr reason that names the
   failed gate / absent tree, and **no** stdout JSON — same mapping as
   live `backup-tree-status`.
4. GREEN should reuse the live require-gate helper (or extract
   `enforceBackupTreeStatusRequireGates`) against the decoded inner
   status rather than duplicating the mapping.

### D. Inner `present: true` consistency (no tree walk)

When inner `present` is true, decode into the existing `backupTreeStatus`
shape and fail closed unless **all** of the following hold:

1. `backup_tree` is a non-empty absolute cleaned path (same
   absolute-path policy the live inspector records). Do **not** `Lstat`
   that path: the original tree may already be archived.
2. `store_count == len(stores) == len(backupTreeStoreSpecs)` (`8`).
3. `stores[i].kind` matches `backupTreeStoreSpecs[i]` in that fixed
   order (`accounts`, `login-tickets`, `item-templates`,
   `interaction-store`, `static-actors`, `quest-state`, `ground-items`,
   `safebox`).
4. A missing store entry is exactly `present: false` / `valid: false`
   and omits `path`, `manifest_filename`, `format`, `manifest_sha256`,
   and every count field.
5. A present store entry is `present: true` **and** `valid: true`.
   Live `backup-tree-status` fail-closes before JSON for invalid present
   children, so retained `present: true` / `valid: false` is
   inconsistent.
6. Present stores set `path` to the slash-normalized lab subdir
   (`accounts`, not an absolute path).
7. Present stores set `manifest_filename` / `format` to the store
   package constants already frozen by live `backup-tree-status`:

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

8. Present stores set `manifest_sha256` to lowercase hex SHA-256
   (exactly 64 `[0-9a-f]` characters). Do **not** reopen the original
   manifest to re-hash it.
9. `crash_temp_count` is omitted or a non-negative integer. Crash-temp
   **filenames** remain forbidden.
10. Additive count fields are omitted or non-negative integers and may
    be non-zero **only** for the matching kind:

    - `accounts`: `account_count`, `character_count`
    - `login-tickets`: `ticket_count`, `character_count`
    - `item-templates`: `template_count`
    - `interaction-store`: `definition_count`
    - `static-actors`: `actor_count`
    - `quest-state`: `flag_count`
    - `ground-items`: `ground_item_count`
    - `safebox`: `character_count`, `cell_count`

    A non-zero count on the wrong kind fails closed. Identity slices
    (`logins`, `login_keys`, `vnums`, `vids`, `actor_ids`,
    `actor_names`, `characters`, `quest_refs`, `flag_keys`,
    `character_keys`, `definition_keys`) remain forbidden unknown
    fields.
11. Completeness aggregates **recompute** from the decoded store entries
    and must match the reported values exactly:
    - `store_present_count` = number of entries with `present: true`
    - `stores_complete` is present (not omitted) and true iff
      `store_present_count == 8`

    Live `backup-tree-status` encodes `stores_complete` as a `*bool`
    with `omitempty`, so inner `present: false` omits it, but inner
    `present: true` always emits the boolean. Omitted
    `stores_complete` on a present tree fails closed.
12. After consistency succeeds, apply `--require-stores-complete` when
    selected with the same failure semantics as live
    `backup-tree-status` (exit `1`, no stdout JSON).

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove the original tree still exists, that
manifest checksums still match store files, or that live gamed
FileStores currently match the tree.

### E. Successful outer envelope

```json
{
  "format": "go-metin2-backup-tree-status-status-v1",
  "present": true,
  "backup_tree_status_sha256": "...",
  "status": {
    "format": "go-metin2-backup-tree-status-v1",
    "present": true,
    "backup_tree": "/var/metin2/backups/20260906T120000Z-abcdef012345",
    "store_count": 8,
    "store_present_count": 8,
    "stores_complete": true,
    "stores": []
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-backup-tree-status-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `status` is the
decoded retained `go-metin2-backup-tree-status-v1` object (same field
names as live stdout).

### F. Drill printer wiring (same GREEN as the inspector)

`metin2-migrate backup-restore-drill` already retains a gated
`backup-tree-status --require-stores-complete` snapshot after
backup/validate and before aside-rename. GREEN must add a matching
`backup-tree-status-status` redirect **immediately after** that line.
Printer remains print-only and still does not execute status / backup /
restore itself.

```sh
echo '== backup-tree status =='
metin2-migrate backup-tree-status --backup-tree "$BASE" \
  --require-stores-complete \
  > "$BASE/backup-tree-status.json"
metin2-migrate backup-tree-status-status --backup-tree-status "$BASE/backup-tree-status.json" \
  --require-stores-complete \
  > "$BASE/backup-tree-status-status.json"
```

The require flag is correct here: the printed script inspects the status
JSON it just wrote for the tree it just backed up, and operators should
not aside-rename live stores when that retained evidence is incomplete
or internally inconsistent.

Printed scripts still use `set -eu`, so a failed
`backup-tree-status-status` inspect fails the drill.

Hermetic `/bin/sh` proof already puts `metin2-migrate` on `PATH` from
`backup-tree-status` GREEN. GREEN of this inspector **must** assert
`$BASE/backup-tree-status-status.json` is a valid
`go-metin2-backup-tree-status-status-v1` whose
`backup_tree_status_sha256` matches the retained
`backup-tree-status.json` bytes and whose inner `stores_complete` is
true.

Do **not** list `backup-tree-status-status.json` in
[lab deployment topology](../workflow/lab-deployment-topology.md) or
add a working CLI example until GREEN actually produces the command.

### G. Explicit non-goals

- new inner `go-metin2-backup-tree-status-v1` fields
- walking / hashing original store subdirs, manifests, or snapshots
- restoring / aside-renaming / crash-temp cleanup from the
  status-status command
- opening a database or emitting DSNs / executable SQL
- hashing correlation files (`runtime-config.json`,
  `persistence-status-*.json`, `notes.md`, daemon logs)
- requiring those correlation files for inner `stores_complete`
- accepting stdin (`-`)
- a loopback `GET /local/backup-tree-status-status` endpoint
- remote admin / secrets in git / metrics
- stock production DB driver registration
- automatic / scheduled execution of printed backup / restore scripts
- claiming inner `stores_complete` proves live gamed FileStores currently
  match the tree (operators still compare `/local/persistence/status`
  after restore)
- `--require-no-crash-temps` (follow-up, now GREEN — see [CLI backup-tree-status no-crash-temps require-gate contract freeze](2026-09-06-cli-backup-tree-status-no-crash-temps-require-gate-contract-freeze.md))
- changing default (ungated) missing-file outer `present: false` exit `0`
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/backup_tree_status_status.go` (new)
- `internal/migratecli/backup_tree_status_status_test.go` (new)
- `internal/migratecli/backup_tree_status.go` (shared require-gate /
  consistency helper if needed)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/backup_restore_drill.go` (status-status redirect)
- `internal/migratecli/backup_restore_drill_test.go`
- `internal/minimal/backup_restore_drill_http_test.go` (retained
  `backup-tree-status-status.json`)
- `docs/development.md`
- `docs/workflow/file-store-backup-restore-drill.md`
- `docs/workflow/lab-deployment-topology.md` (list
  `backup-tree-status-status.json` only once GREEN produces it)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing backup-tree-status path → outer `present: false`, no DB open,
  no restore, no tree walk
- valid inner `present: false` snapshot → outer `present: true` +
  checksum + inner status, no DB / tree open
- valid complete eight-store snapshot → outer `present: true`, inner
  aggregates unchanged
- kind-order / store_count / stores_complete drift / present+invalid /
  wrong-kind non-zero count / missing-store extra fields → exit `1`,
  empty stdout
- `--require-stores-complete` with absent path, inner `present: false`,
  or incomplete stores → exit `1`, empty stdout
- symlink / oversized / unknown-field / wrong format (including this
  command's own outer envelope) → exit `1`, empty stdout
- usage / unknown-command mention `backup-tree-status-status`
- stdout omits logins, tickets, DSNs, SQL, crash-temp filenames
- `backup-restore-drill` printer emits the matching
  `backup-tree-status-status --require-stores-complete` redirect
  immediately after `backup-tree-status.json` and before aside-rename

Hermetic HTTP proof in `internal/minimal`:

- printed-script execution still round-trips seeded account + safebox
- retained `backup-tree-status-status.json` exists with matching
  checksum and inner `stores_complete: true`

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'BackupTreeStatusStatus|BackupRestoreDrillPrints|RejectsUnknownCommandMentionsBackupTreeStatusStatus' -count=1
go test ./internal/minimal -run 'BackupRestoreDrillHTTP' -count=1
gofmt -l internal/migratecli/*.go internal/minimal/backup_restore_drill_http_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`: read-only `backup-tree-status-status`,
matching `backup-restore-drill` redirect, and hermetic
`$BASE/backup-tree-status-status.json` assertion.

- Follow-up owned separately and now GREEN: opt-in `--require-no-crash-temps` on live
  `backup-tree-status` and retained `backup-tree-status-status` — see
  [CLI backup-tree-status no-crash-temps require-gate contract freeze](2026-09-06-cli-backup-tree-status-no-crash-temps-require-gate-contract-freeze.md).

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope /
  consistency rules / printer wiring / hermetic status-status assertion
- Track E / migration-contract point at this freeze as the next GREEN
  target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not walk the original backup-tree or call `ValidateBackupFrom`
  from the status-status command.
- Do not restore, aside-rename, or open a database from the
  status-status command.
- Do not auto-run `backup-restore-drill` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not list `backup-tree-status-status.json` in lab topology or add a
  working CLI example until GREEN actually produces the command.
