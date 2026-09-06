# CLI backup-tree-status no-crash-temps require-gate contract freeze — 2026-09-06

## Objective

Freeze an opt-in fail-closed `--require-no-crash-temps` gate on the already-landed
`metin2-migrate backup-tree-status` and `backup-tree-status-status` inspectors so
operators can stop a backup-restore window when retained eight-store FileStore
backup evidence still reports hidden crash-temp residue — without hand-parsing
JSON, restoring, emptying live stores, or opening a database.

This freeze does **not** invent new inner `go-metin2-backup-tree-status-v1`
fields, crash-temp cleanup from the status commands, a stock production driver,
loopback ops mutation, remote admin, or any claim that a gated clean tree proves
live gamed FileStores currently have no crash temps.

## Why docs-first

Track E tip chain through `backup-tree-status-status` is Done
([CLI backup-tree-status-status contract freeze](2026-09-06-cli-backup-tree-status-status-contract-freeze.md)).

Live `backup-tree-status` already:

- walks the eight lab store subdirs through `ValidateBackupFrom`
- treats hidden crash-temps as **non-fatal** (`crash_temp_count` only; filenames
  stay forbidden)
- owns opt-in `--require-stores-complete`

`backup-tree-status-status` already re-validates retained
`backup-tree-status.json` and reuses `enforceBackupTreeStatusRequireGates` for
`--require-stores-complete`.

Ungated inspection remaining exit `0` with `crash_temp_count: 1` is correct for
browsing a retained tree. Operators still reassemble stop/go by hand (or via
`jq`) when they need fail-closed exit semantics before aside-rename / restore.

`backup-restore-drill` already:

- prints live-store `validate` + `crash-temps/cleanup` triage **before** backup
- copies only committed snapshots via `BackupTo` (crash temps are not backup
  payload)
- gates `$BASE/backup-tree-status.json` with `--require-stores-complete`

A freshly printed drill tree should therefore report omitted / zero
`crash_temp_count` on every present store. Residue can still appear later if an
operator copies a dirty tree, drops hidden crash temps into `$BASE/<store>/`, or
re-inspects an archived tree that was never cleaned. Opening RED without freezing
the flag name, absent-tree behavior, stderr mapping, helper reuse, and printer
wiring would invent those operator-facing exit semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default inspector behavior stays ungated

1. With **no** `--require-no-crash-temps`, both inspectors keep today's
   semantics:
   - hidden crash-temps remain non-fatal
   - present valid stores still report `crash_temp_count` when non-zero
   - crash-temp **filenames** remain forbidden stdout
   - missing children stay ungated `present: false` / `stores_complete: false`
   - invalid present children still fail closed before JSON
2. Existing `--require-stores-complete` stays unchanged.
3. No silent upgrade of inspection into a crash-temp gate.
4. No new status JSON fields. GREEN gates on the already-landed per-store
   `crash_temp_count` integers (omitted / zero means none).

### B. Opt-in flag (exact name frozen)

```bash
metin2-migrate backup-tree-status \
  --backup-tree <absolute-path> \
  [--require-stores-complete] \
  [--require-no-crash-temps]

metin2-migrate backup-tree-status-status \
  --backup-tree-status <path> \
  [--require-stores-complete] \
  [--require-no-crash-temps]
```

Rules:

1. `--require-no-crash-temps` is independently opt-in (boolean; default false)
   on **both** commands, using the **same name and aggregate mapping**.
2. It may be combined freely with `--require-stores-complete`.
3. Usage text lists `--require-no-crash-temps` beside
   `--require-stores-complete` for both commands.
4. Unknown flags / unexpected args still exit `2`.
5. Still no database open, SQL execution, HTTP, backup, restore, aside-rename,
   crash-temp cleanup, lock reservation, artifact deletion, or daemon mutation.
6. `backup-tree-status-status` still does not walk the original backup-tree or
   call `ValidateBackupFrom`.
7. Never emit DSNs, executable SQL, snapshot payloads, login names, tickets,
   vnums, vids, actor ids, or crash-temp **filenames**.

### C. Require failure semantics

When `--require-no-crash-temps` is selected:

1. If the inspected path is absent (`present: false` for live
   `backup-tree-status`, or outer missing-file `present: false` /
   inner `present: false` for `backup-tree-status-status`), exit `1` with a
   short stderr reason that names the failed gate and the absent tree, and
   **no** stdout status JSON.
2. If the tree / inner snapshot is present and **any** store entry has
   `crash_temp_count > 0`, exit `1` with a short stderr reason that names the
   failed gate, `crash_temp_count`, and the first store `kind` with a positive
   count, and **no** stdout status JSON.
3. Omitted `crash_temp_count` (JSON `omitempty` zero) counts as zero.
4. Missing store entries (`present: false` / `valid: false`, no count fields)
   do **not** fail this gate by themselves — that remains
   `--require-stores-complete`.
5. When the selected crash-temp gate is satisfied on a present tree (every
   store entry has omitted / zero `crash_temp_count`), emit the same
   `go-metin2-backup-tree-status-v1` /
   `go-metin2-backup-tree-status-status-v1` JSON as today.
6. Invalid present children / inconsistent retained snapshots still fail closed
   **before** require-gating (unchanged exit `1` / no stdout JSON path).
7. GREEN must extend the existing `enforceBackupTreeStatusRequireGates` helper
   (or extract an equivalent shared helper) rather than duplicating the mapping
   across live inspect and status-status. Evaluate `--require-stores-complete`
   **first**, then `--require-no-crash-temps`, so a missing tree with both flags
   still reports the stores-complete absent-tree reason already frozen.

Suggested stderr shapes (GREEN may wrap with the command prefix already used
by both inspectors):

```text
--require-no-crash-temps failed: backup-tree is absent
--require-no-crash-temps failed: crash_temp_count>0 on accounts
```

### D. Drill printer wiring (same GREEN as the flags)

`metin2-migrate backup-restore-drill` already retains gated
`backup-tree-status --require-stores-complete` and matching
`backup-tree-status-status --require-stores-complete` snapshots after
backup/validate and before aside-rename. GREEN must add
`--require-no-crash-temps` to **both** redirects, immediately after
`--require-stores-complete`:

```sh
echo '== backup-tree status =='
metin2-migrate backup-tree-status --backup-tree "$BASE" \
  --require-stores-complete \
  --require-no-crash-temps \
  > "$BASE/backup-tree-status.json"
metin2-migrate backup-tree-status-status --backup-tree-status "$BASE/backup-tree-status.json" \
  --require-stores-complete \
  --require-no-crash-temps \
  > "$BASE/backup-tree-status-status.json"
```

The extra flag is correct here: `BackupTo` does not copy hidden crash temps, so
a successful printed drill still writes residue-free status JSON. Operators
should not aside-rename live stores when the retained tree later shows crash-temp
residue (hand-copied / edited trees). Printer remains print-only and still does
not execute status / backup / restore / crash-temp cleanup itself.

Printed scripts still use `set -eu`, so a failed crash-temp require inspect
fails the drill.

Hermetic `/bin/sh` proof already puts `metin2-migrate` on `PATH` and asserts
`$BASE/backup-tree-status.json` plus `$BASE/backup-tree-status-status.json`.
GREEN of this gate **must** keep that proof green under the extra flags (fresh
drill backups have omitted / zero `crash_temp_count`). Do **not** invent a new
retained filename.

Do **not** change lab topology listings or add a working CLI example that
claims the flag already exists until GREEN actually produces it.

### E. Explicit non-goals

- new inner `go-metin2-backup-tree-status-v1` fields (`crash_temp_count_total`,
  `no_crash_temps`, crash-temp filenames)
- changing default ungated crash-temp acceptance (exit `0` + `crash_temp_count`)
- crash-temp cleanup / restore / aside-rename from either status command
- walking live gamed FileStores or requiring live `/local/*/crash-temps/cleanup`
  as part of this gate
- copying crash temps into `BackupTo` output
- opening a database or emitting DSNs / executable SQL
- accepting stdin (`-`)
- a loopback `GET /local/backup-tree-status` endpoint
- remote admin / secrets in git / metrics
- stock production DB driver registration
- automatic / scheduled execution of printed backup / restore scripts
- claiming a gated clean tree proves live gamed FileStores currently have no
  crash temps (operators still compare `/local/persistence/status` and per-store
  validate)
- changing default (ungated) missing-tree outer `present: false` exit `0`
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/backup_tree_status.go` (flag + shared require helper)
- `internal/migratecli/backup_tree_status_test.go`
- `internal/migratecli/backup_tree_status_status.go` (flag + usage)
- `internal/migratecli/backup_tree_status_status_test.go`
- `internal/migratecli/backup_restore_drill.go` (both redirects)
- `internal/migratecli/backup_restore_drill_test.go`
- `internal/minimal/backup_restore_drill_http_test.go` (still green under the
  extra flags)
- `docs/development.md`
- `docs/workflow/file-store-backup-restore-drill.md`
- `docs/workflow/lab-deployment-topology.md` (working CLI examples only once
  GREEN produces the flag)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- ungated hidden crash-temps still exit `0` and report `crash_temp_count`
  (existing `TestRunBackupTreeStatusAcceptsHiddenCrashTemps` stays)
- `--require-no-crash-temps` + hidden crash-temp on a complete tree → exit `1`,
  empty stdout, stderr names the gate / `crash_temp_count` / store kind
- `--require-no-crash-temps` + absent tree / missing status path / inner
  `present: false` → exit `1`, empty stdout, stderr names the gate / absent tree
- `--require-no-crash-temps` + complete clean tree → exit `0`, JSON unchanged
- `--require-no-crash-temps` + incomplete clean tree → exit `0` (independent of
  `stores_complete`)
- both require flags + incomplete clean tree → still fail
  `--require-stores-complete` first
- `backup-tree-status-status` with a retained snapshot that has
  `crash_temp_count: 1` fails closed **after the original tree is deleted**
- usage / unknown-command text lists `--require-no-crash-temps`
- unknown extra flag still exit `2`
- stdout still omits crash-temp filenames, logins, tickets, DSNs, SQL
- `backup-restore-drill` printer emits `--require-no-crash-temps` on both
  status redirects immediately after `--require-stores-complete` and before
  aside-rename

Hermetic HTTP proof in `internal/minimal`:

- printed-script execution still round-trips seeded account + safebox
- retained status JSON still exists with `stores_complete: true` under the
  extra flags

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'BackupTreeStatus|BackupRestoreDrillPrints|RejectsUnknownCommandMentionsBackupTreeStatus' -count=1
go test ./internal/minimal -run 'BackupRestoreDrillHTTP' -count=1
gofmt -l internal/migratecli/*.go internal/minimal/backup_restore_drill_http_test.go
git diff --check
```

## Status

Docs/spec freeze on `lane/persistence`. GREEN is follow-on: opt-in
`--require-no-crash-temps` on live `backup-tree-status` and retained
`backup-tree-status-status`, matching `backup-restore-drill` redirects, and
hermetic HTTP proof kept green under the extra flags.

## Exit criteria for this freeze

- this plan exists and names exact flag / failure semantics / helper reuse /
  printer wiring / hermetic no-new-artifact rule
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not add new inner status JSON fields in GREEN.
- Do not change ungated crash-temp acceptance.
- Do not cleanup / restore / open a database from the status commands.
- Do not auto-run `backup-restore-drill` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not add a working CLI example that claims the flag already exists until
  GREEN actually produces it.
