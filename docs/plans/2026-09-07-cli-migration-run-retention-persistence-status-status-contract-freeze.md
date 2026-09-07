# CLI migration-run-retention persistence-status-status contract freeze — 2026-09-07

## Objective

Freeze printer + hermetic curl-stub wiring so read-only
`metin2-migrate persistence-status-status` inspects retained
`$RUN/persistence-status-before.json` / `$RUN/persistence-status-after.json`
inside printed `migration-run-retention` scripts.

Operators can then re-check eight-store live FileStore health evidence during a
DB apply / rollback window **without** curling the live ops mux again, restoring,
walking FileStores, or treating an incomplete curl stub as a valid snapshot.

This freeze does **not** invent automatic apply/rollback execution, a stock
production driver, loopback ops mutation, remote admin, gated
`--require-ok` / `--require-drained` / `--require-no-crash-temps` on these
retention redirects, a live inner `go-metin2-persistence-status-v1` format
marker, or any claim that a present valid status-status file proves live
FileStore / DB row state after a later apply.

## Why docs-first

Track E tip chain through `persistence-status-status` +
`backup-restore-drill` printer/HTTP wiring is Done
([CLI persistence-status-status contract freeze](2026-09-07-cli-persistence-status-status-contract-freeze.md)).

That GREEN explicitly deferred `migration-run-retention`: the printer already
retains `$RUN/persistence-status-before.json` before catalog/ledger/plan and
`$RUN/persistence-status-after.json` after post-apply / post-rollback status,
but:

1. there is still no matching `persistence-status-status` redirect beside those
   files (unlike `backup-restore-drill`);
2. the hermetic SQLite curl stub still emits
   `{"ok":true,"live_selected_character_count":0}`, which fails this inspector's
   eight-store consistency rules.

Ungated `jq` of a retained file still works for browsing. Operators still
reassemble inspect-by-hand when they need fail-closed exit semantics after a
migration window. Printed scripts use `set -eu`, so wiring the inspector
without expanding the stub would turn today's green hermetic apply/rollback
proofs red for a missing-store reason, not a real FileStore failure.

Opening RED without freezing printer placement, ungated vs gated semantics,
stub JSON shape, hermetic filenames, leftover-lock ordering, and the
no-new-live-endpoint rule would invent those operator-facing exit semantics
mid-implementation. Freeze first; GREEN is now landed on `lane/persistence`.

Working CLI (inspect a retained migration-run file):

```bash
metin2-migrate persistence-status-status \
  --persistence-status /var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/persistence-status-after.json
```

Printed `migration-run-retention` companions (ungated):

```sh
curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"
metin2-migrate persistence-status-status \
  --persistence-status "$RUN/persistence-status-before.json" \
  > "$RUN/persistence-status-before-status.json"
```

```sh
curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"
metin2-migrate persistence-status-status \
  --persistence-status "$RUN/persistence-status-after.json" \
  > "$RUN/persistence-status-after-status.json"
```

## Contract to freeze (before RED)

### A. Printer wiring (same GREEN as the stub expansion)

`metin2-migrate migration-run-retention` already retains ungated
`persistence-status-before.json` during correlation and
`persistence-status-after.json` after post-apply / post-rollback status.
GREEN must add a matching `persistence-status-status` redirect
**immediately after** each of those lines. Printer remains print-only and
still does not execute status / apply / rollback / backup / restore itself.

Forward and `--allow-rollback` share `renderMigrationRunRetentionScript`;
both directions get the same two redirects. Do **not** invent a second
command or rollback-only inspect path.

1. **Before-status** stays ungated live capture, then ungated retained
   inspection:

```sh
curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"
metin2-migrate persistence-status-status \
  --persistence-status "$RUN/persistence-status-before.json" \
  > "$RUN/persistence-status-before-status.json"
```

2. **After-status** stays ungated live capture, then ungated retained
   inspection:

```sh
curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"
metin2-migrate persistence-status-status \
  --persistence-status "$RUN/persistence-status-after.json" \
  > "$RUN/persistence-status-after-status.json"
```

Do **not** print `--require-ok`, `--require-drained`, or
`--require-no-crash-temps` on these redirects in this GREEN.

Why this differs from `backup-restore-drill`:

- drill after-status is post-**restore** stop/go (`--require-ok --require-drained`)
  because restore is blocked by live selected-character sessions;
- migration-run-retention after-status is **correlation** around a CLI-only
  ledger apply/rollback that does not restore FileStores;
- leftover-lock triage is printed **after** persistence-status-after;
  a gated inspect that failed because a lab `gamed` still had a selected
  character would abort `set -eu` before that triage ran.

Operators who want fail-closed FileStore stop/go during a migration window
already have the inspector and may re-run it by hand with require flags.
Do **not** change successful-apply script exit semantics in this GREEN.

Printed scripts still use `set -eu`, so a malformed / incomplete retained
snapshot still fails the run — that is why the curl stub must expand.

Do **not** invent a new retained filename beyond those two `*-status.json`
companions.

### B. Hermetic curl-stub expansion (same GREEN)

`mustInstallMigrationRunRetentionCurlStub` currently answers
`*/local/persistence/status` with an incomplete aggregate body. GREEN must
replace that body with a compact one-line JSON snapshot that passes ungated
`persistence-status-status` eight-store consistency.

Rules:

1. Keep the stub a `PATH` `curl` shim. Do **not** start `gamed`, open
   FileStores, or call `internal/minimal`.
2. Body is the **live** no-format `/local/persistence/status` object (no wrapping
   `go-metin2-persistence-status-status-v1` `format` marker).
3. All eight store objects are present with the inspector's exact keys.
4. Empty valid stores are allowed and preferred for this stub (zero counts,
   empty identity slices, `backup_manifest.present: false`,
   `restore_blocked_by_live_sessions: false`,
   `live_selected_character_count: 0`, recomputed `ok: true`).
5. Each store `path` is a non-empty NUL-free placeholder; GREEN does not
   require those paths to exist on disk.
6. Do **not** change other stub URL bodies (`/local/build-info`,
   `/local/runtime-config`, `/local/db/migrations/status`) except as needed to
   keep today's proofs green.
7. GREEN may share a compact helper with
   `internal/migratecli/persistence_status_status_test.go` or inline the
   compact JSON in the stub writer. Keep it migratecli-local.

### C. Hermetic `/bin/sh` assertions (same GREEN)

Existing tagged proofs already put `metin2-migrate` + the curl stub on `PATH`
and assert `$RUN/persistence-status-before.json` (forward tip) plus
`$RUN/persistence-status-after.json` (all four proofs). GREEN **must** keep
those proofs green under the extra redirects and assert on **every** tagged
proof (forward tip, rollback-to-zero, intermediate forward `7`, intermediate
rollback `8`):

- `$RUN/persistence-status-before-status.json` is
  `go-metin2-persistence-status-status-v1` with `present: true`
- `$RUN/persistence-status-after-status.json` is the same format with
  inner `ok: true` and `live_selected_character_count: 0`

Do **not** require leftover-lock `apply-lock-aside-status.json` on the
successful path (still expected absent).

Do **not** change untagged `go test ./internal/migratecli` into a SQLite
harness test. Untagged coverage owns printer stdout shape + ordering only.

### D. Ordering (untagged printer tests)

Keep today's mkdir → authd/runtime/status-before → daemon logs → notes →
catalog → preflight → apply → post-status → status-after → conditional lock
triage order, with the new inspect lines **immediately after** their matching
JSON retains:

1. `> "$RUN/persistence-status-before.json"` then before-status-status
   **before** daemon-log copies / `notes.md`
2. `> "$RUN/persistence-status-after.json"` then after-status-status
   **before** leftover-lock triage

Forward, rollback-to-zero, and intermediate printer tests that already pin
the curl retain lines must also pin the new inspect lines and must **not**
contain `--require-ok`, `--require-drained`, or `--require-no-crash-temps`
on those inspect lines.

### E. Explicit non-goals

- changing `persistence-status-status` inspector flags / envelope / recompute
  rules
- changing `backup-restore-drill` printer/HTTP wiring
- printing require-gates on migration-run-retention persistence-status
  redirects
- expanding `migration-run-retention` into FileStore backup/restore
- walking / hashing live FileStores or backup-tree store subdirs
- starting `gamed` from the SQLite harness curl stub
- importing `internal/minimal` from `migratecli`
- adding a live inner `format` marker on `GET /local/persistence/status`
- leftover-lock auto-delete / auto-run `apply-lock-aside`
- opening a database from the status command
- a loopback `GET /local/persistence-status-status` endpoint
- remote admin / secrets in git / metrics
- stock production DB driver registration
- automatic / scheduled execution of printed apply / rollback scripts
- claiming a retained after-status proves live FileStores currently match the
  applied ledger (operators still compare `post-apply-status` /
  `ledger-snapshot-status` plus `/local/persistence/status`)
- changing default (ungated) missing-file outer `present: false` exit `0`
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/migration_run_retention.go` (before/after redirects)
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
  (curl stub + `*-status.json` assertions)
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md` (tree listing + printer sentence
  only once GREEN produces the companions)
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused untagged coverage in `internal/migratecli`:

- forward printer emits ungated before-status-status immediately after
  `persistence-status-before.json` and ungated after-status-status immediately
  after `persistence-status-after.json`
- rollback printer emits the same two ungated inspect lines beside rollback
  artifact names
- inspect lines omit `--require-ok` / `--require-drained` /
  `--require-no-crash-temps`
- leftover-lock triage still follows after-status-status
- stdout still omits DSNs / executable SQL and still does not auto-run
  `apply-lock-aside`

Hermetic tagged coverage:

- curl stub `/local/persistence/status` body passes ungated
  `persistence-status-status`
- all four printed-script proofs stay exit `0` under `set -eu`
- retained before/after status-status JSON exists with the frozen envelopes
- after-status-status inner snapshot is `ok: true` and drained

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'MigrationRunRetention' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/migratecli/migration_run_retention.go internal/migratecli/migration_run_retention_test.go internal/migratecli/migration_run_retention_sqlite_harness_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`: `migration-run-retention` prints ungated
`persistence-status-status` companions immediately after
`$RUN/persistence-status-before.json` / `$RUN/persistence-status-after.json`,
and the hermetic SQLite curl stub emits a compact empty eight-store
`GET /local/persistence/status` body that those inspect lines accept.
~~Follow-up owned separately: freeze-only read-only `status-status` for
retained `$RUN/post-apply-status.json` / `$RUN/post-rollback-status.json`.~~
Done — see [CLI status-status contract freeze](2026-09-07-cli-status-status-contract-freeze.md).
Printed scripts now emit gated `$RUN/post-apply-status-status.json` /
`$RUN/post-rollback-status-status.json` companions.

## Exit criteria for this freeze

- this plan exists and names exact printer placement / ungated vs gated
  semantics / stub JSON rules / hermetic filenames / leftover-lock ordering
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not print require-gates on these retention redirects in GREEN.
- Do not start `gamed` from the curl stub.
- Do not import `internal/minimal` from `migratecli`.
- Do not change `backup-restore-drill` in GREEN.
- Do not restore / cleanup / open a database from the status command.
- Do not auto-run `migration-run-retention` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not add a working CLI example that claims the printer already emits
  `persistence-status-*-status.json` unless GREEN actually produced them.
