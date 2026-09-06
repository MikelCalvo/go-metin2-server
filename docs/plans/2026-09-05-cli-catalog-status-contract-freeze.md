# CLI catalog-status contract freeze — 2026-09-05

## Objective

Freeze a read-only `metin2-migrate catalog-status` inspector for retained
`migration-catalog.json` artifacts so operators can re-check the metadata-only
migration inventory during incident review or release packaging **without**
opening a database or trusting a hand-edited catalog file.

This freeze does **not** invent upsert / merge / tip-`0002` cascade-delete, a
stock production driver, automatic / scheduled script execution, loopback ops
mutation, catalog SQL text, or any claim that a present valid catalog file
proves live `schema_migrations` row state.

## Why docs-first

Track E tip chain through `export-tree-status-status` is Done
([CLI export-tree-status-status contract freeze](2026-09-05-cli-export-tree-status-status-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`plan-artifact-status`, `ledger-snapshot-status`,
`apply-preflight-status`, `apply-lock-status`, `apply-audit-status`,
`import-export-status`, `synthesize-wipe-export-status`,
`export-tree-status`, `export-tree-status-status`).
`migration-run-retention` and `export-quarantine-drill` already retain
`migration-catalog.json` beside the tree (CLI `catalog` or loopback
`GET /local/db/migrations/catalog`), but there is no small command to
re-validate that file by itself after the original binary is archived,
copied, or hand-edited.

Opening RED without freezing:

- exact command / flag names,
- outer status envelope + checksum field,
- size cap and fail-closed file policy,
- inner contiguous-version / path / checksum consistency,
- how `matches_embedded` compares against the inspecting binary,
- whether the original database or SQL bodies may be opened,
- which printed retention / export-quarantine redirects adopt the new inspector,

would invent operator-facing incident-review semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate catalog-status --catalog <path> [--require-matches-embedded]
```

Rules:

1. Requires `--catalog`; extra positional arguments are usage errors
   (exit `2`).
2. The path is a retained `go-metin2-migration-catalog-summary-v1` JSON file
   (`migration-catalog.json`), **not** a SQL catalog directory and **not**
   stdin. Relative paths are allowed (same file-inspector policy as
   `ledger-snapshot-status` / `plan-artifact-status`). stdin (`-`) is
   **not** accepted.
3. `--require-matches-embedded` is independently opt-in (boolean; default
   false).
4. Usage text lists the command beside the other `*-status` helpers and lists
   `--require-matches-embedded` beside `--catalog`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, import mutation, synthesize,
   quarantine rewrite, lock reservation, artifact deletion, daemon mutation, or
   filesystem walk of `db/migrations/*.sql`.
7. Never emits DSNs, executable SQL, runtime store rows, or live ledger
   payloads.

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `catalog`) when
   the path is absent.
2. Rejects symlink or non-regular paths, oversized files over **64 KiB**,
   invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON,
   or an unsupported `format` marker with exit `1`, a short stderr reason, and
   **no** stdout status JSON.
3. Input `format` must be `go-metin2-migration-catalog-summary-v1`. A
   `go-metin2-migration-catalog-status-v1` file (this command's own output)
   is the wrong format and must fail closed.
4. `catalog_sha256` is computed over the **exact retained file bytes** so
   operators can correlate the inspected file with lab notes / retention trees.

### C. Inner `present: false` retained snapshots

There is no valid inner `present: false` catalog-summary shape. Missing-path
handling is only the outer envelope in section E.

Any selected `--require-matches-embedded` against an absent path fails closed
with exit `1`, a short stderr reason that names the failed gate / absent
catalog, and **no** stdout JSON.

### D. Inner catalog consistency (no SQL / DB open)

When the file is present, decode into the existing
`db/migrations.CatalogSummaryPayload` shape
(`go-metin2-migration-catalog-summary-v1`) and fail closed unless **all** of
the following hold:

1. `latest_version >= 1`.
2. `latest_version == len(migrations)`.
3. `migrations[i].version == i+1` for every entry (contiguous from `1`).
4. `migrations[i].name` is non-empty and matches `^[a-z0-9_]+$`.
5. `migrations[i].up_path` equals
   `fmt.Sprintf("%04d_%s.up.sql", version, name)`.
6. `migrations[i].down_path` equals
   `fmt.Sprintf("%04d_%s.down.sql", version, name)`.
7. `up_sha256` and `down_sha256` are lowercase hex SHA-256 (exactly 64
   `[0-9a-f]` characters).
8. The decoded object contains **no** executable SQL text fields (the summary
   shape never includes `UpSQL` / `DownSQL`).

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove live `schema_migrations` rows.

After consistency succeeds, set `matches_embedded` by comparing the decoded
summary to `dbmigrations.BuiltInCatalogSummary()` from the inspecting binary
(exact `format`, `latest_version`, and every row's version / name / paths /
checksums). Do **not** reopen embedded SQL files to re-hash them.

Then apply `--require-matches-embedded` when selected: fail closed with exit
`1` and **no** stdout JSON when `matches_embedded` is false.

### E. Successful outer envelope

```json
{
  "format": "go-metin2-migration-catalog-status-v1",
  "present": true,
  "catalog_sha256": "...",
  "matches_embedded": true,
  "catalog": {
    "format": "go-metin2-migration-catalog-summary-v1",
    "latest_version": 29,
    "migrations": []
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-migration-catalog-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `catalog` is the decoded
retained `go-metin2-migration-catalog-summary-v1` object (same field names as
live `metin2-migrate catalog` / `GET /local/db/migrations/catalog` stdout).

A present valid catalog that drifted from the inspecting binary is still
ungated success with `matches_embedded: false`. That lets operators inspect an
older retained tree without the newer binary failing closed.

### F. Printer wiring (same GREEN as the inspector)

GREEN must add a matching `catalog-status` redirect **immediately after** each
existing catalog retain line. Printers remain confirmation-gated /
print-only and still do not execute status / apply / import themselves.

1. **`migration-run-retention`** (forward and `--allow-rollback`):
   ```sh
   metin2-migrate catalog > "$RUN/migration-catalog.json"
   metin2-migrate catalog-status --catalog "$RUN/migration-catalog.json" \
     --require-matches-embedded \
     > "$RUN/migration-catalog-status.json"
   ```
   The require flag is correct here: the printed script invokes the same
   `metin2-migrate` binary that just wrote the catalog.

2. **`export-quarantine-drill`**:
   ```sh
   curl -sS "$OPS/local/db/migrations/catalog" > "$BASE/migration-catalog.json"
   metin2-migrate catalog-status --catalog "$BASE/migration-catalog.json" \
     --require-matches-embedded \
     > "$BASE/migration-catalog-status.json"
   ```
   The require flag fail-closes when loopback `gamed` catalog inventory drifts
   from the inspecting `metin2-migrate` binary (mixed-version window).

Printed scripts still use `set -eu`, so a failed catalog-status inspect fails
the drill. Contrib `lab-retention-gc` forwarding of those printers stays
unchanged in this freeze (no new env vars required here).

### G. Explicit non-goals

- walking / hashing `db/migrations/*.sql` from the status command
- exposing executable SQL or DSNs
- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed catalog / apply / import scripts
- opening a database from `catalog-status`
- claiming `matches_embedded` proves live DB row presence or that a target is
  migrated
- changing default (ungated) missing-file outer `present: false` exit `0`
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/catalog_status.go` (new)
- `internal/migratecli/catalog_status_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/migration_run_retention.go` (catalog-status redirect)
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
- `internal/migratecli/export_quarantine_drill.go`
- `internal/migratecli/export_quarantine_drill_test.go`
- `internal/minimal/export_quarantine_drill_http_test.go`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/workflow/lab-deployment-topology.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing catalog path → outer `present: false`, no DB open
- valid current embedded catalog snapshot → outer `present: true` + checksum +
  `matches_embedded: true` + inner catalog, no DB open
- valid older/internally consistent catalog that drifted from the inspecting
  binary → ungated `matches_embedded: false`
- `--require-matches-embedded` with absent path, drifted catalog, or inner
  mismatch → exit `1`, empty stdout
- version gaps / `latest_version != len(migrations)` / path/name mismatch /
  uppercase checksum / unknown-field / wrong format (including this command's
  own outer envelope) / symlink / oversized → exit `1`, empty stdout
- usage / unknown-command mention `catalog-status`
- `migration-run-retention` forward + rollback printers emit the matching
  catalog-status redirect with `--require-matches-embedded`
- `export-quarantine-drill` printer emits the matching catalog-status redirect
  with `--require-matches-embedded`
- hermetic migration-run-retention SQLite proof retains a valid
  `migration-catalog-status.json` whose checksum matches the catalog JSON and
  `matches_embedded` is true
- hermetic export-quarantine drill HTTP proof retains the same status file

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'CatalogStatus|MigrationRunRetentionPrints|ExportQuarantineDrillPrints|RejectsUnknownCommandMentionsCatalogStatus' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
go test ./internal/minimal -run 'ExportQuarantineDrillHTTP' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Status

GREEN on `lane/persistence`.

- Read-only `metin2-migrate catalog-status --catalog <path> [--require-matches-embedded]`
  re-validates retained `migration-catalog.json` without opening a database or
  walking `db/migrations/*.sql`.
- Outer envelope is `go-metin2-migration-catalog-status-v1`; missing path is
  ungated `present: false`; present files re-check inner contiguous versions /
  paths / lowercase SHA-256 and set `matches_embedded` against
  `BuiltInCatalogSummary()`.
- `migration-run-retention` (forward + rollback) and `export-quarantine-drill`
  printers emit matching `catalog-status --require-matches-embedded` redirects
  to `migration-catalog-status.json`.
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.
- Follow-up owned separately and now GREEN: read-only `apply-lock-aside-status` for retained `apply-lock-aside.json` — see [CLI apply-lock-aside-status contract freeze](2026-09-05-cli-apply-lock-aside-status-contract-freeze.md).
- Follow-up owned separately: read-only `backup-tree-status` for retained backup-restore trees — see [CLI backup-tree-status contract freeze](2026-09-06-cli-backup-tree-status-contract-freeze.md).

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / consistency
  rules / `matches_embedded` / printer wiring
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not open a database or read migration SQL from the status command.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
