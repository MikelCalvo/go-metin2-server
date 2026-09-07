# CLI status-status contract freeze — 2026-09-07

## Objective

Freeze a read-only `metin2-migrate status-status` inspector for retained
`metin2-migrate status` / `GET /local/db/migrations/status` Plan JSON
(`post-apply-status.json` / `post-rollback-status.json` /
`daemon-migrations-status.json`) so operators can re-check metadata-only
schema-boundary evidence after a CLI apply/rollback window **without**
reopening the target database, curling the live ops mux, or treating a
hand-edited Plan file as valid.

This freeze does **not** invent automatic apply/rollback execution, a stock
production driver, loopback ops mutation, remote admin, a live inner `format`
marker on `status` / `GET /local/db/migrations/status`, FileStore restore,
or any claim that a present valid status-status file proves live
`schema_migrations` rows after a later apply.

## Why docs-first

Track E tip chain through `migration-run-retention` persistence-status-status
is Done
([CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`catalog-status`, `plan-artifact-status`,
`ledger-snapshot-status`, `apply-preflight-status`, `apply-lock-status`,
`apply-lock-aside-status`, `apply-audit-status`, `import-export-status`,
`synthesize-wipe-export-status`, `export-tree-status`,
`export-tree-status-status`, `backup-tree-status`,
`backup-tree-status-status`, `persistence-status-status`).
`migration-run-retention` already retains:

- `$RUN/post-apply-status.json` (forward) / `$RUN/post-rollback-status.json`
  (`--allow-rollback`) from live `metin2-migrate status --driver "$DRIVER"
  --dsn "$DSN" --target-version "$TARGET_VERSION"`
- `$RUN/daemon-migrations-status.json` from optional
  `GET /local/db/migrations/status`

Those files are the same metadata-only `db/migrations.Plan` shape
(`current_version`, `latest_version`, `up_to_date`, `pending[]`) with **no**
`format` marker. There is still no small command to re-validate that Plan
JSON by itself after the original binary is archived, the tree is copied, or
the file is hand-edited.

`plan-artifact-status` is the wrong inspector: it requires
`go-metin2-migration-plan-artifact-v1` (`format` + `plan_sha256` + wrapped
`plan`). Feeding it a live `status` Plan fails closed on missing `format`.

Ungated `jq` of a retained file still works for browsing. Operators still
reassemble stop/go by hand when they need fail-closed exit semantics after
apply (`up_to_date: true` toward the printed `$TARGET_VERSION`) without
reopening the DSN.

Printed `migration-run-retention` scripts use `set -eu`. Wiring a gated
inspect after post-apply/post-rollback status is the honest fail-closed
companion for the CLI apply window. The optional daemon-status curl stays
ungated: a lab `gamed` with DB preflight disabled reports an empty-ledger
plan (`current_version: 0`, pending catalog) even after a successful CLI
apply, and leftover-lock triage still follows post-status.

The hermetic SQLite curl stub currently answers
`*/local/db/migrations/status` with
`{"current_version":0,"latest_version":0,"up_to_date":false,"pending":[]}`.
That body is internally inconsistent (`latest_version` must be positive;
`up_to_date` must equal `len(pending)==0` against a positive catalog
latest). GREEN must **not** fail the successful hermetic proofs on that
optional curl: either leave the daemon inspect ungated (malformed daemon
status still fails closed if operators later inspect it by hand) **or**
expand the stub to a valid empty-ledger Plan against the inspecting
catalog. Prefer expanding the stub in the same GREEN as printer wiring so
operators can later inspect `$RUN/daemon-migrations-status.json` with the
new command without a second stub slice. Do **not** print a gated
`--require-up-to-date` on the daemon curl.

Opening RED without freezing command / flag names, the live no-format inner
shape, Plan-shape reuse, require-gate semantics, printer placement, stub
rules, hermetic filenames, leftover-lock ordering, and the
no-new-live-endpoint rule would invent those operator-facing exit
semantics mid-implementation. Freeze first; GREEN stays follow-on.

Working CLI (inspect a retained post-apply file after GREEN):

```bash
metin2-migrate status-status \
  --status /var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/post-apply-status.json \
  --require-up-to-date
```

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate status-status \
  --status <path> \
  [--require-up-to-date] \
  [--require-matches-embedded-latest]
```

Rules:

1. Requires `--status`; extra positional arguments are usage errors
   (exit `2`).
2. The path is a retained live no-format `db/migrations.Plan` JSON file
   (`post-apply-status.json` / `post-rollback-status.json` /
   `daemon-migrations-status.json`), **not** a
   `go-metin2-migration-plan-artifact-v1` file, **not**
   `ledger-snapshot.json`, and **not** stdin. Relative paths are allowed
   (same file-inspector policy as `catalog-status` /
   `plan-artifact-status` / `persistence-status-status`). stdin (`-`) is
   **not** accepted: a path whose name is `-` is a missing/regular file
   named `-`.
3. `--require-up-to-date` and `--require-matches-embedded-latest` are
   independently opt-in (boolean; default false) and may be combined
   freely.
4. Usage text lists the command beside `status` / `plan-artifact-status` /
   `migration-run-retention` and lists both require flags beside `--status`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, HTTP, apply, rollback, lock
   reservation, FileStore walk, or daemon mutation.
7. Never emits DSNs or executable SQL.

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `plan`) when
   the path is absent.
2. Rejects symlink or non-regular paths, oversized files over **64 KiB**
   (same cap as catalog / ledger-snapshot inspectors; live Plan JSON is
   small), invalid UTF-8, empty files, malformed JSON, unknown fields,
   trailing JSON, or a wrapping `format` marker with exit `1`, a short
   stderr reason, and **no** stdout status JSON.
3. Input must be the live no-format Plan object. A
   `go-metin2-migration-status-status-v1` file (this command's own output),
   a `go-metin2-migration-plan-artifact-v1` file, or any other wrapped
   envelope is the wrong format and must fail closed.
4. `status_sha256` is computed over the **exact retained file bytes** so
   operators can correlate the inspected file with lab notes / retention
   trees.

### C. Inner Plan consistency (no SQL / DB open)

When the file is present, decode into `db/migrations.Plan` with unknown
fields disallowed and fail closed unless **all** of the following hold,
reusing `validateMigrationPlanShape` (same contiguous pending-step replay
already used by `plan-artifact-status`):

1. `latest_version` is positive.
2. `current_version` is inside `0..latest_version`.
3. `up_to_date == (len(pending) == 0)`.
4. Each pending step has a positive version `<= latest_version`, a
   non-empty name, a non-empty path, a lowercase hex SHA-256, and
   direction `up` or `down`.
5. Pending steps do not mix directions.
6. Pending up steps continue from `current_version+1`; pending down steps
   continue from `current_version` downward.
7. Replayed pending steps land on a version inside `0..latest_version`.

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove live `schema_migrations` rows.

After consistency succeeds, set `matches_embedded_latest` by comparing
`plan.LatestVersion` to `len(dbmigrations.Catalog())` from the inspecting
binary. Do **not** reopen embedded SQL files to re-hash pending step
checksums. A valid older Plan whose `latest_version` drifted from the
inspecting catalog is still ungated success with
`matches_embedded_latest: false`.

Then apply selected require-gates **in this order**, each failing closed
with exit `1`, a short stderr reason that names the failed gate, and
**no** stdout JSON:

1. `--require-up-to-date`
   - absent path → `--require-up-to-date failed: status is absent`
   - `up_to_date: false` → `--require-up-to-date failed: up_to_date=false`
2. `--require-matches-embedded-latest`
   - absent path → `--require-matches-embedded-latest failed: status is absent`
   - `matches_embedded_latest: false` →
     `--require-matches-embedded-latest failed: matches_embedded_latest=false`

Do **not** invent `--require-current-version` or `--require-pending-empty`
as separate flags in this GREEN. `up_to_date` already means pending is
empty toward the Plan's own current/latest pair.

### D. Successful outer envelope

```json
{
  "format": "go-metin2-migration-status-status-v1",
  "present": true,
  "status_sha256": "...",
  "matches_embedded_latest": true,
  "plan": {
    "current_version": 29,
    "latest_version": 29,
    "up_to_date": true,
    "pending": []
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-migration-status-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `plan` is the decoded
retained no-format Plan object (same field names as live
`metin2-migrate status` / `GET /local/db/migrations/status` stdout).

A present valid Plan that is not up-to-date is still ungated success with
`plan.up_to_date: false`. That lets operators inspect a pre-apply or
intermediate retained file without the inspector failing closed.

### E. Printer wiring (same GREEN as the inspector)

`metin2-migrate migration-run-retention` already retains ungated
`post-apply-status.json` / `post-rollback-status.json` after apply and
optional `daemon-migrations-status.json` during correlation. GREEN must
add a matching `status-status` redirect **immediately after** the
post-apply / post-rollback retain line. Printer remains print-only and
still does not execute status / apply / rollback itself.

Forward and `--allow-rollback` share `renderMigrationRunRetentionScript`;
both directions get the same inspect placement against their existing
`$postStatus` filename.

1. **Post-apply / post-rollback** (gated stop/go for the CLI window):

```sh
metin2-migrate status \
  --driver "$DRIVER" \
  --dsn "$DSN" \
  --target-version "$TARGET_VERSION" \
  > "$RUN/post-apply-status.json"
metin2-migrate status-status \
  --status "$RUN/post-apply-status.json" \
  --require-up-to-date \
  --require-matches-embedded-latest \
  > "$RUN/post-apply-status-status.json"
```

Rollback uses `$RUN/post-rollback-status.json` /
`$RUN/post-rollback-status-status.json` with the same two require flags.

Why gated here: the printed script invokes the same `metin2-migrate`
binary that just applied toward `$TARGET_VERSION`. After a successful
apply/rollback, `status --target-version "$TARGET_VERSION"` must be
`up_to_date: true`. `--require-matches-embedded-latest` fail-closes when
the inspecting binary's catalog latest drifted from the retained Plan
(mixed-version window). For rollback-to-zero the Plan is still
`up_to_date: true` toward target `0` even though `current_version` is `0`
and `latest_version` remains the catalog tip.

2. **Optional daemon-status** stays ungated live capture. Do **not** print
   a `status-status` redirect beside `$RUN/daemon-migrations-status.json`
   in this GREEN.

Why: leftover-lock triage is printed **after** persistence-status-after
and post-status. A gated daemon inspect that failed because lab `gamed`
had DB preflight disabled (empty-ledger Plan, `up_to_date: false`) would
abort `set -eu` before leftover-lock triage. Operators who want
fail-closed daemon-status inspection already have the inspector and may
re-run it by hand.

GREEN **should** still expand the hermetic curl stub
`*/local/db/migrations/status` body to a compact one-line JSON Plan that
passes ungated `status-status` Plan-shape consistency:

- `latest_version` equals `len(Catalog())` of the inspecting binary
- `current_version: 0`
- `up_to_date: false`
- `pending` is the full catalog up-plan (or GREEN may emit a minimal
  internally consistent empty-ledger Plan produced by
  `PlanUpToLatest(nil)` / `PlanToVersion(nil, 0)`). Prefer
  `PlanUpToLatest(nil)` so the stub matches a real disabled-preflight
  daemon.
- Keep the stub a `PATH` `curl` shim. Do **not** start `gamed` or open a
  database from the stub.

If emitting the full pending catalog makes the stub awkward, GREEN may
leave the daemon curl body unchanged **only if** the printer still does
not inspect that file. Do not wire a daemon `status-status` redirect
against today's `latest_version: 0` body.

Do **not** invent a new retained filename beyond
`post-apply-status-status.json` / `post-rollback-status-status.json`.

Do **not** change `backup-restore-drill`. That printer does not retain
`metin2-migrate status` Plan JSON.

### F. Hermetic `/bin/sh` assertions (same GREEN)

Existing tagged proofs already put `metin2-migrate` + the curl stub on
`PATH` and assert `$RUN/post-apply-status.json` (forward / intermediate
forward) plus `$RUN/post-rollback-status.json` (rollback proofs) via
`assertPostStatusCurrentVersion`. GREEN **must** keep those proofs green
under the extra redirects and assert on **every** tagged proof (forward
tip, rollback-to-zero, intermediate forward `7`, intermediate rollback
`8`):

- `$RUN/post-apply-status-status.json` or
  `$RUN/post-rollback-status-status.json` is
  `go-metin2-migration-status-status-v1` with `present: true`
- inner `plan.up_to_date` is `true`
- inner `plan.current_version` matches today's
  `assertPostStatusCurrentVersion` expectation
- `matches_embedded_latest` is `true`

Do **not** require leftover-lock `apply-lock-aside-status.json` on the
successful path (still expected absent).

Do **not** change untagged `go test ./internal/migratecli` into a SQLite
harness test. Untagged coverage owns printer stdout shape + ordering plus
inspector unit tests only.

### G. Ordering (untagged printer tests)

Keep today's mkdir → authd/runtime/status-before → daemon logs → notes →
catalog → preflight → apply → post-status → status-after → conditional
lock triage order, with the new inspect line **immediately after** the
matching post-status retain and **before** persistence-status-after:

1. `> "$RUN/post-apply-status.json"` then post-apply-status-status
   **before** `persistence-status-after.json`
2. leftover-lock triage still follows persistence-status-after-status

Forward, rollback-to-zero, and intermediate printer tests that already pin
the post-status retain lines must also pin the new inspect lines and must
contain `--require-up-to-date` and `--require-matches-embedded-latest` on
those inspect lines.

### H. Explicit non-goals

- changing live `metin2-migrate status` / `GET /local/db/migrations/status`
  to wrap a `format` marker
- a loopback `GET /local/db/migrations/status-status` endpoint
- printing a gated inspect beside `daemon-migrations-status.json`
- changing `backup-restore-drill` / `persistence-status-status`
- opening a database from `status-status`
- importing `internal/minimal` from `migratecli`
- leftover-lock auto-delete / auto-run `apply-lock-aside`
- stock production DB driver registration
- automatic / scheduled execution of printed apply / rollback scripts
- claiming a gated clean post-apply-status-status proves live FileStores
  currently match the applied ledger (operators still compare
  `persistence-status-status` plus `/local/persistence/status`)
- changing default (ungated) missing-file outer `present: false` exit `0`
- `--require-current-version` / `--require-pending-empty` extra flags
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/status_status.go` (new; migratecli-local Plan
  decode, reuse `validateMigrationPlanShape`)
- `internal/migratecli/status_status_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/migration_run_retention.go` (post-status redirect)
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
  (optional curl-stub Plan expansion + `*-status-status.json` assertions)
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md` (tree listing + printer
  sentence only once GREEN produces the companions)
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused untagged coverage in `internal/migratecli`:

- missing status path → outer `present: false`, no DB open
- valid up-to-date Plan → outer `present: true` + checksum +
  `matches_embedded_latest: true` + inner plan, no DB open
- valid not-up-to-date empty-ledger Plan → ungated `up_to_date: false`
- valid older/internally consistent Plan whose `latest_version` drifted
  from the inspecting catalog → ungated `matches_embedded_latest: false`
- `--require-up-to-date` with absent path or `up_to_date: false` → exit `1`,
  empty stdout
- `--require-matches-embedded-latest` with absent path or drifted latest →
  exit `1`, empty stdout
- wrapping `format` / plan-artifact envelope / unknown-field / symlink /
  oversized / `latest_version: 0` / `up_to_date` vs pending mismatch →
  exit `1`, empty stdout
- usage / unknown-command text lists `status-status`
- unknown extra flag still exit `2`
- stdout omits DSNs / executable SQL
- `migration-run-retention` forward printer emits gated
  `--require-up-to-date --require-matches-embedded-latest`
  `status-status` immediately after `post-apply-status.json` and before
  `persistence-status-after.json`
- rollback printer emits the same gated inspect beside
  `post-rollback-status.json`
- leftover-lock triage still follows persistence-status-after-status
- stdout still omits DSNs / executable SQL and still does not auto-run
  `apply-lock-aside`
- daemon-migrations-status retain stays ungated (no `status-status`
  redirect beside it)

Hermetic tagged coverage:

- all four printed-script proofs stay exit `0` under `set -eu`
- retained post-apply / post-rollback status-status JSON exists with the
  frozen envelopes
- inner plan is `up_to_date: true` with the expected `current_version`

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'StatusStatus|MigrationRunRetentionPrints|RejectsUnknownCommandMentionsStatusStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/migratecli/status_status.go internal/migratecli/status_status_test.go internal/migratecli/migratecli.go internal/migratecli/migration_run_retention.go internal/migratecli/migration_run_retention_test.go internal/migratecli/migration_run_retention_sqlite_harness_test.go
git diff --check
```

## Status

Freeze-only on `lane/persistence`. GREEN stays follow-on; printed scripts
still do not emit `post-apply-status-status.json` /
`post-rollback-status-status.json`.

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / live
  no-format inner shape / Plan-shape reuse / require-gate order / printer
  wiring / hermetic filenames / leftover-lock ordering / daemon-status
  exclusion
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not add a live inner `format` marker in GREEN.
- Do not print a gated inspect beside `daemon-migrations-status.json`.
- Do not change `backup-restore-drill` in GREEN.
- Do not restore / apply / open a database from the status-status command.
- Do not auto-run `migration-run-retention` from CLI.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not add a working CLI example that claims the printer already emits
  `post-*-status-status.json` unless GREEN actually produced them.
