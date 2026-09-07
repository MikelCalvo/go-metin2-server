# CLI persistence-status-status contract freeze — 2026-09-07

## Objective

Freeze a read-only `metin2-migrate persistence-status-status` inspector for
retained `GET /local/persistence/status` artifacts (`persistence-status-before.json`
/ `persistence-status-after.json`) so operators can re-check eight-store live
FileStore health evidence during a backup-restore window or incident review
**without** curling the live ops mux, restoring, walking FileStores, or opening
a database.

This freeze does **not** invent automatic backup/restore execution, a stock
production driver, loopback ops mutation, remote admin, a live
`go-metin2-persistence-status-v1` format marker on `GET /local/persistence/status`,
`migration-run-retention` printer wiring, or any claim that a present valid
status file proves live FileStore / DB row state after a later restore.

## Why docs-first

Track E tip chain through `--require-no-crash-temps` is Done
([CLI backup-tree-status no-crash-temps require-gate contract freeze](2026-09-06-cli-backup-tree-status-no-crash-temps-require-gate-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`catalog-status`, `plan-artifact-status`,
`ledger-snapshot-status`, `apply-preflight-status`, `apply-lock-status`,
`apply-lock-aside-status`, `apply-audit-status`, `import-export-status`,
`synthesize-wipe-export-status`, `export-tree-status`,
`export-tree-status-status`, `backup-tree-status`,
`backup-tree-status-status`).
`backup-restore-drill` already retains `$BASE/persistence-status-before.json`
before backup and `$BASE/persistence-status-after.json` after restore, and
`backup-tree-status` already re-validates the **backup tree**. There is still
no small command to re-validate those **live** `/local/persistence/status`
snapshots by themselves after the original daemon is stopped, the tree is
archived, copied, or hand-edited.

The live endpoint already:

- returns `200` even when one store is invalid so operators can inspect all
  eight stores in one body
- reports aggregate `ok` plus `live_selected_character_count`
- reports per-store `valid`, `summary`, `backup_manifest`,
  `restore_blocked_by_live_sessions`, and optional `error`
- includes identity slices in summaries (`logins`, `login_keys`, `vnums`,
  `actor_ids` / `actor_names`, `definition_keys`, `characters` / `quest_refs`
  / `flag_keys`, `vids`, safebox `character_keys`)

Ungated `jq` of a retained file still works for browsing. Operators still
reassemble stop/go by hand when they need fail-closed exit semantics after
restore (`ok` + drained sessions) without curling the live mux.

`backup-restore-drill` already uses `set -eu`. A gated post-restore inspect
must fail the printed drill when the retained after-status is absent,
malformed, `ok: false`, or still reports live selected-character sessions.

Opening RED without freezing the command / flag names, the live (no-format)
inner shape, `ok` / drained recompute rules, identity-slice count mapping,
stderr mapping, printer wiring, and the hermetic no-new-live-endpoint rule
would invent those operator-facing exit semantics mid-implementation.
Freeze first; GREEN is now landed on `lane/persistence`.

Working CLI example after GREEN:

```bash
metin2-migrate persistence-status-status \
  --persistence-status /var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/persistence-status-after.json \
  --require-ok \
  --require-drained
```

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate persistence-status-status \
  --persistence-status <path> \
  [--require-ok] \
  [--require-drained] \
  [--require-no-crash-temps]
```

Rules:

1. Requires `--persistence-status`; extra positional arguments are usage
   errors (exit `2`).
2. The path is a retained `GET /local/persistence/status` JSON file
   (`persistence-status-before.json` / `persistence-status-after.json`),
   **not** a backup-tree directory, **not** `backup-tree-status.json`, and
   **not** stdin. Relative paths are allowed (same file-inspector policy as
   `backup-tree-status-status` / `export-tree-status-status` /
   `catalog-status`). stdin (`-`) is **not** accepted: a path whose name is
   `-` is a missing/regular file named `-`.
3. `--require-ok`, `--require-drained`, and `--require-no-crash-temps` are
   independently opt-in (boolean; default false) and may be combined freely.
4. Usage text lists the command beside `backup-tree-status-status` /
   `backup-restore-drill` and lists the three require flags beside
   `--persistence-status`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, HTTP, backup, restore,
   aside-rename, crash-temp cleanup, lock reservation, artifact deletion,
   daemon mutation, filesystem walk of live gamed FileStores / backup-tree
   store subdirs, or `Validate` / `ValidateBackupFrom`.
7. Never emits DSNs, executable SQL, snapshot payloads, login keys as
   secrets, tickets, or raw store files beyond echoing the already-retained
   inner persistence-status JSON (identity slices that the live endpoint
   already wrote stay in the inner `status` object; GREEN must not invent a
   stripped projection).

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `status`) when
   the path is absent.
2. Rejects symlink or non-regular paths, oversized files over **128 KiB**
   (same cap as `backup-tree-status-status` / `export-tree-status-status`;
   live persistence-status includes identity slices, so a very large lab
   snapshot could hit it — drill-sized files fit; bumping the cap is
   follow-on), invalid UTF-8, empty files, malformed JSON, unknown fields,
   trailing JSON, or a wrapping `format` marker with exit `1`, a short
   stderr reason, and **no** stdout status JSON.
3. Input is the **live** `/local/persistence/status` object. That live JSON
   currently has **no** `format` field. A
   `go-metin2-persistence-status-status-v1` file (this command's own output)
   is the wrong shape and must fail closed.
4. `persistence_status_sha256` is computed over the **exact retained file
   bytes** so operators can correlate the inspected file with lab notes /
   drill trees.

### C. Inner live snapshot consistency (no mux / FileStore open)

When the file is present, decode fail-closed into **migratecli-local** types
that use the same JSON field names as
`internal/minimal.PersistenceStatusSnapshot`. GREEN must **not** import
`internal/minimal` (that package is the gamed runtime). Fail closed unless
**all** of the following hold.

#### C1. Aggregate fields

1. `live_selected_character_count` is an integer `>= 0`.
2. All eight store objects are present with these exact keys, no extras:
   - `account_store`
   - `login_ticket_store`
   - `item_template_store`
   - `static_actor_store`
   - `interaction_store`
   - `quest_state_store`
   - `ground_item_store`
   - `safebox_store`
3. `ok` **recomputes** as the AND of the eight store `valid` bits and must
   match the reported `ok` exactly.
4. For **every** store, `restore_blocked_by_live_sessions` **recomputes** as
   `live_selected_character_count != 0` and must match the reported bit
   (today's runtime sets the same live-session guard on all eight stores).

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove live FileStores currently match, and it does
not prove the original daemon is still running.

#### C2. Per-store envelope

For each store object:

1. `path` is a non-empty NUL-free string (the live endpoint reports the
   configured FileStore path; GREEN does not require the path to exist now).
2. `valid` is a boolean.
3. `summary` is present (the live Go structs do not omit it).
4. When `valid` is true: `error` is omitted or empty.
5. When `valid` is false: `error` is a non-empty string; summary count fields
   may be zero. Do **not** fail the ungated inspector solely because
   `valid` is false — the live endpoint returns `200` for that case.
6. `backup_manifest` is present. Today's live endpoint can report
   `present: true` for an unreadable/symlink/malformed manifest with only
   `path` (and maybe size/checksum) filled in, so GREEN must not require a
   complete metadata set on every `present: true`.
   - `present: false` omits `path` / `format` / counts / checksum.
   - `present: true` requires a non-empty NUL-free `path`.
   - When `valid` is true **and** `format` is present, `format` must match
     that store's backup format marker below. Omitted `format` is allowed
     (symlink / unreadable / malformed live manifests). Do **not** fail
     ungated inspect solely because an invalid store reports a wrong or
     unexpected `format` — live `Validate()` can fail after the status
     helper still copies `manifest.Format`.
   - When `file_count`, `snapshot_size_bytes`, or `manifest_size_bytes` are
     present they must be `>= 0` (`omitempty` zeros may be omitted).
   - When `manifest_sha256` is present it must be lowercase hex SHA-256
     (exactly 64 `[0-9a-f]` characters).
7. Backup format markers (already landed):
   - accounts → `go-metin2-account-backup-v1`
   - login tickets → `go-metin2-login-ticket-backup-v1`
   - item templates → `go-metin2-item-template-backup-v1`
   - interactions → `go-metin2-interaction-backup-v1`
   - static actors → `go-metin2-static-actor-backup-v1`
   - quest state → `go-metin2-quest-state-backup-v1`
   - ground items → `go-metin2-ground-item-backup-v1`
   - safebox → `go-metin2-safebox-backup-v1`

#### C3. Per-store summary counts (valid stores)

When `valid` is true, fail closed unless the store-specific count mapping
holds. Omitted `omitempty` crash-temp fields count as zero. Identity slices
may be empty arrays (valid empty stores). `null` identity slices fail closed
on a valid store.

- **accounts:** `account_count == len(logins) >= 0`; `character_count >= 0`;
  optional `empty_character_slot_count >= 0` and
  `empty_character_slot_count <= character_count`; logins are unique
  non-empty strings.
- **login tickets:** `ticket_count == len(logins) == len(login_keys) >= 0`;
  `character_count >= 0`; optional `empty_character_slot_count >= 0` and
  `empty_character_slot_count <= character_count`; logins unique
  non-empty; `login_keys` unique uint32 JSON numbers; when both issued-at
  bounds are present, `oldest_issued_at <= newest_issued_at`.
- **item templates:** `template_count == len(vnums) >= 0`; vnums unique.
- **static actors:** `actor_count == len(actor_ids) == len(actor_names) >= 0`;
  optional `interactable_actor_count` / `spawn_group_count` are `>= 0` and
  `<= actor_count`; actor ids unique.
- **interactions:** `definition_count == len(definition_keys) >= 0`;
  definition keys unique non-empty (`kind:ref`).
- **quest state:** `flag_count == len(flag_keys) >= 0`; `flag_keys`,
  `characters`, and `quest_refs` are unique non-empty strings;
  `len(characters) <= flag_count`; `len(quest_refs) <= flag_count`.
- **ground items:** `ground_item_count == len(vids) >= 0`;
  `item_shaped_count >= 0`; `gold_shaped_count >= 0`;
  `item_shaped_count + gold_shaped_count == ground_item_count`; vids unique.
- **safebox:** `character_count == len(character_keys) >= 0`;
  `cell_count >= 0`; logins unique non-empty;
  `len(logins) <= character_count`; character keys unique non-empty
  (`login:character_id`).

Crash-temp residue (all stores): `crash_temp_count >= 0`; when
`crash_temp_files` is present, `len(crash_temp_files) == crash_temp_count`.
Filenames are already in the retained live JSON; GREEN echoes them inside
inner `status` and must not invent extra stdout listing.

When `valid` is false, do not enforce the count mapping (zero / omitted
summaries are allowed).

### D. Require failure semantics

Evaluate selected gates **after** inner consistency, in this order:

1. `--require-ok`
2. `--require-drained`
3. `--require-no-crash-temps`

so a missing file with several flags still reports the first selected gate's
absent-file reason.

When a selected gate fails: exit `1`, short stderr reason that names the
failed gate, **no** stdout status JSON.

| Gate | Absent path / outer missing file | Present inner snapshot |
| --- | --- | --- |
| `--require-ok` | fail: persistence-status is absent | fail unless recomputed `ok == true` |
| `--require-drained` | fail: persistence-status is absent | fail unless `live_selected_character_count == 0` (the per-store restore-blocked bits already recompute from that count) |
| `--require-no-crash-temps` | fail: persistence-status is absent | fail if **any** store summary has `crash_temp_count > 0` |

Suggested stderr shapes (GREEN may wrap with the `persistence-status-status:`
prefix already used by sibling inspectors):

```text
--require-ok failed: persistence-status is absent
--require-ok failed: ok=false
--require-drained failed: live_selected_character_count=1
--require-no-crash-temps failed: crash_temp_count>0 on account_store
```

Default inspect with **no** require flags stays ungated: `ok: false` and
non-zero `crash_temp_count` still exit `0` with JSON, matching the live
endpoint's "always 200, inspect every store" policy.

### E. Successful outer envelope

```json
{
  "format": "go-metin2-persistence-status-status-v1",
  "present": true,
  "persistence_status_sha256": "...",
  "status": {
    "ok": true,
    "live_selected_character_count": 0,
    "account_store": {},
    "login_ticket_store": {},
    "item_template_store": {},
    "static_actor_store": {},
    "interaction_store": {},
    "quest_state_store": {},
    "ground_item_store": {},
    "safebox_store": {}
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-persistence-status-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `status` is the decoded
retained live persistence-status object (same field names as
`GET /local/persistence/status`). Do **not** add a live inner `format`
marker in this slice.

### F. Drill printer wiring (same GREEN as the inspector)

`metin2-migrate backup-restore-drill` already retains ungated
`persistence-status-before.json` during correlation and
`persistence-status-after.json` after restore. GREEN must add a matching
`persistence-status-status` redirect **immediately after** each of those
lines. Printer remains print-only and still does not execute status /
backup / restore itself.

1. **Before-status** stays ungated live capture, then ungated retained
   inspection:

```sh
curl -sS "$OPS/local/persistence/status" > "$BASE/persistence-status-before.json"
metin2-migrate persistence-status-status \
  --persistence-status "$BASE/persistence-status-before.json" \
  > "$BASE/persistence-status-before-status.json"
```

2. **After-status** keeps today's live capture, then inspects the retained
   after JSON with `--require-ok --require-drained` (post-restore stop/go):

```sh
curl -sS "$OPS/local/persistence/status" > "$BASE/persistence-status-after.json"
metin2-migrate persistence-status-status \
  --persistence-status "$BASE/persistence-status-after.json" \
  --require-ok \
  --require-drained \
  > "$BASE/persistence-status-after-status.json"
```

Do **not** print `--require-no-crash-temps` on these redirects in this
GREEN. Live post-restore crash-temp residue is a separate operator choice;
the already-landed backup-tree `--require-no-crash-temps` gate covers the
retained **backup** tree, not live FileStores.

Printed scripts still use `set -eu`, so a failed after-status inspect fails
the drill.

Hermetic `/bin/sh` proof already puts `metin2-migrate` on `PATH` and asserts
`$BASE/persistence-status-before.json` plus `$BASE/persistence-status-after.json`.
GREEN of this inspector **must** keep that proof green under the extra
redirects and assert:

- `$BASE/persistence-status-before-status.json` is
  `go-metin2-persistence-status-status-v1` with `present: true`
- `$BASE/persistence-status-after-status.json` is the same format with
  inner `ok: true` and `live_selected_character_count: 0`

Do **not** invent a new retained filename beyond those two `*-status.json`
companions.

Do **not** change `migration-run-retention` in this GREEN. That printer also
retains `persistence-status-*.json`, but its hermetic SQLite curl stub emits
an incomplete `{"ok":true,"live_selected_character_count":0}` body that would
fail this inspector's eight-store consistency rules. Expanding that stub is
now GREEN — see
[CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md).

The inspector command already exists. `migration-run-retention` now emits
`persistence-status-*-status.json` companions — see
[CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md).

### G. Explicit non-goals

- adding a live inner `format` / `go-metin2-persistence-status-v1` marker to
  `GET /local/persistence/status`
- changing the live endpoint to `4xx` when `ok` is false
- walking / hashing live FileStores or backup-tree store subdirs
- stripping identity slices from inner `status` stdout
- `migration-run-retention` / contrib helper printer wiring (now GREEN — see [CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md))
- crash-temp cleanup / restore / aside-rename from the status command
- opening a database or emitting DSNs / executable SQL
- accepting stdin (`-`)
- a loopback `GET /local/persistence-status-status` endpoint
- remote admin / secrets in git / metrics
- stock production DB driver registration
- automatic / scheduled execution of printed backup / restore scripts
- claiming a gated clean after-status proves live FileStores currently match
  the retained backup tree (operators still compare `backup-tree-status` plus
  `/local/persistence/status`)
- changing default (ungated) missing-file outer `present: false` exit `0`
- printing `--require-no-crash-temps` on drill persistence-status redirects
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/persistence_status_status.go` (new; migratecli-local
  snapshot types, no `internal/minimal` import)
- `internal/migratecli/persistence_status_status_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/backup_restore_drill.go` (before/after redirects)
- `internal/migratecli/backup_restore_drill_test.go`
- `internal/minimal/backup_restore_drill_http_test.go` (assert the two
  `*-status.json` companions)
- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/workflow/file-store-backup-restore-drill.md`
- `docs/workflow/lab-deployment-topology.md` (working CLI examples only once
  GREEN produces the command)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing persistence-status path → outer `present: false`, no HTTP, no DB
- present valid drained eight-store snapshot → outer `present: true` +
  checksum + inner `ok: true` / `live_selected_character_count: 0`
- present `ok: false` (one invalid store + error) still exits `0` ungated
- `--require-ok` + `ok: false` or absent path → exit `1`, empty stdout
- `--require-drained` + `live_selected_character_count: 1` → exit `1`,
  empty stdout, stderr names the gate / count
- `--require-no-crash-temps` + `crash_temp_count: 1` on one store → exit `1`,
  stderr names the gate / store key
- both `--require-ok` and `--require-drained` on an absent path still report
  `--require-ok` first
- `ok: true` that disagrees with a `valid: false` child → fail closed before
  require-gating
- `restore_blocked_by_live_sessions: true` while count is `0` → fail closed
- valid accounts `account_count` / `len(logins)` mismatch → fail closed
- wrapping `go-metin2-persistence-status-status-v1` input → fail closed
- usage / unknown-command text lists `persistence-status-status`
- unknown extra flag still exit `2`
- stdout omits DSNs / executable SQL
- `backup-restore-drill` printer emits ungated before-status-status immediately
  after `persistence-status-before.json` and gated
  `--require-ok --require-drained` after-status-status immediately after
  `persistence-status-after.json`

Hermetic HTTP proof in `internal/minimal`:

- printed-script execution still round-trips seeded account + safebox
- retained before/after status-status JSON exists with the frozen envelopes
- after-status-status inner snapshot is `ok: true` and drained

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'PersistenceStatusStatus|BackupRestoreDrillPrints|RejectsUnknownCommandMentionsPersistenceStatusStatus' -count=1
go test ./internal/minimal -run 'BackupRestoreDrillHTTP' -count=1
gofmt -l internal/migratecli/*.go internal/minimal/backup_restore_drill_http_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`: read-only `persistence-status-status` inspects retained
`GET /local/persistence/status` JSON, `backup-restore-drill` prints an ungated
before-status-status redirect plus a gated after-status-status
(`--require-ok --require-drained`) redirect, and the hermetic HTTP proof asserts
`$BASE/persistence-status-before-status.json` plus
`$BASE/persistence-status-after-status.json`. Matching `migration-run-retention`
printer + hermetic curl-stub expansion is now GREEN — see
[CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md).
Follow-up owned separately: freeze-only read-only `status-status` for
retained `post-apply-status.json` / `post-rollback-status.json` — see
[CLI status-status contract freeze](2026-09-07-cli-status-status-contract-freeze.md).

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / live
  no-format inner shape / `ok`+drained recompute / printer wiring /
  hermetic filenames / migration-run-retention exclusion
- Track E / migration-contract point at this freeze; GREEN is now landed
- freeze commit stayed docs-only; this GREEN commit adds the inspector
- tree stays green after the GREEN commit

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not add a live inner `format` marker in GREEN.
- Do not import `internal/minimal` from `migratecli`.
- Do not wire `migration-run-retention` in this GREEN; curl-stub expansion is owned separately — see [CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md).
- Do not restore / cleanup / open a database from the status command.
- Do not auto-run `backup-restore-drill` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not add a working CLI example that claims `migration-run-retention`
  already emits `persistence-status-*-status.json` unless that follow-up GREEN
  actually produced them.
