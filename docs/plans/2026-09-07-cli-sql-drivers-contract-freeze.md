# CLI sql-drivers / loopback GET /local/db/drivers contract freeze — 2026-09-07

## Objective

Freeze operator-visible **linked-driver discovery** so a lab or production
operator can tell which `database/sql` driver names are actually compiled
into the binary they are about to use — and fail closed when `$DRIVER` is
not linked — **without** registering a stock production engine, opening a
DSN, or turning FileStores into SQL repositories.

This is the remaining Track E production-engine / runbook-hardening gap
after GREEN `status-status`. It is **not** engine selection (SQLite vs
MySQL vs Postgres as a release default). It is the smallest honest
preflight that makes a later engine choice fail closed instead of dying
inside `sql.Open` / daemon `409`s.

This freeze does **not** invent automatic apply/rollback execution, a stock
production driver, loopback ops mutation, remote admin, token auth,
forwarded-header trust, proxy awareness, FileStore restore, or any claim
that a present driver name proves live `schema_migrations` rows or
DB-backed runtime stores.

## Why docs-first

Track E tip chain through `status-status` is Done
([CLI status-status contract freeze](2026-09-07-cli-status-status-contract-freeze.md)).

What operators still cannot answer from a retained tree or a running
binary:

1. Does **this** `metin2-migrate` have `$DRIVER` linked, before the printed
   script opens a DSN for `ledger-snapshot` / `apply`?
2. Does **this** `gamed` have the configured `METIN2_*_DB_DRIVER` linked,
   matching startup `ValidateDatabaseDriverAvailability`, without inferring
   it from `GET /local/runtime-config` (`database.driver` /
   `dsn_configured` only)?
3. Is the stock release still empty (no registered engine), vs a
   deliberately tagged `sqlite_harness` lab binary that registers
   `sqlite` via `modernc.org/sqlite`?

`sql.Open` already fails when the driver is unknown, and daemon startup
already rejects an unregistered configured driver. Those paths are not an
operator catalog: CLI stderr is an opaque `open database driver %q: %v`
after a DSN was supplied, and `GET /local/runtime-config` reports the
**configured** name even when operators have not yet distinguished
"configured" from "linked into this process".

`migration-run-retention` already fail-closes on unset `$DRIVER` /
`$DSN` (`: "${DRIVER:?...}"`) then immediately runs
`ledger-snapshot --driver "$DRIVER" --dsn "$DSN"`. There is still no
linked-driver gate **before** the DSN-touching command. Printed scripts
use `set -eu`, so wiring `--require-driver "$DRIVER"` is the honest
stop/go for "this binary cannot apply because it was not built with that
engine".

Opening RED without freezing command / flag names, the empty-list-is-valid
stock contract, the harness driver name (`sqlite`, not `sqlite3`),
loopback path / gamed-only registration, printer placement, hermetic
filenames, and the no-stock-driver rule would invent those operator-facing
exit semantics mid-implementation. Freeze first; GREEN stays follow-on.

Working CLI (inspect the current binary after GREEN):

```bash
metin2-migrate drivers
metin2-migrate drivers --require-driver "$DRIVER"
```

Working loopback (gamed only, after GREEN):

```bash
curl -sS http://127.0.0.1:6060/local/db/drivers
```

## Contract to freeze (before RED)

### A. Runtime primitive (exact names frozen)

Keep the action as a programmatic helper first, independent of HTTP/CLI:

- `config.RegisteredDatabaseDrivers() []string`
  - returns a **sorted** copy of `database/sql.Drivers()`
  - never returns a nil slice (empty stock binary → `[]string{}`)
  - never opens a DSN, pings, queries `schema_migrations`, or applies SQL
- `config.RequireRegisteredDatabaseDriver(name string) error`
  - trims `name`
  - empty → invalid (callers treat as usage or `ErrDatabaseConfigInvalid`)
  - NUL / whitespace in the raw name → `ErrDatabaseConfigInvalid` (same
    alphabet as `ValidateDatabaseConfig`)
  - not present in `RegisteredDatabaseDrivers()` →
    `ErrDatabaseDriverUnavailable` wrapping the trimmed name
  - present → nil

`ValidateDatabaseDriverAvailability` stays the daemon-startup preflight
and may reuse the same list helper. It still accepts disabled DB
preflight (`driver == ""` and `dsn == ""`) and still never opens a DSN.

Do **not** alias `sqlite` ↔ `sqlite3`. The build-tagged harness registers
`sqlite` (`modernc.org/sqlite`). Config examples that mention `sqlite3` as
an opaque configured string are not a registered driver in stock or
harness binaries.

### B. CLI command and flags (exact names frozen)

```bash
metin2-migrate drivers [--require-driver <database/sql-driver-name>]
```

Rules:

1. No positional arguments. Extra args / unknown flags → usage exit `2`.
2. `--require-driver` is independently opt-in (default unset).
3. Missing `--require-driver` always succeeds with the envelope below,
   including when `drivers` is an empty array (stock binary).
4. `--require-driver` with empty/whitespace value → usage exit `2`.
5. `--require-driver` with NUL / embedded whitespace → exit `1`, short
   stderr, **no** stdout JSON.
6. `--require-driver` with a name **not** in the linked list → exit `1`,
   short stderr that names `database driver is unavailable` and the
   requested name, **no** stdout JSON. Do not echo a DSN (the command
   does not accept `--dsn`).
7. `--require-driver` with a linked name → exit `0` and the **full**
   sorted list (not a one-element filter).
8. Usage text lists `drivers` beside `status` / `version` /
   `migration-run-retention` and lists `--require-driver`.
9. Performs no database open, SQL execution, HTTP, apply, rollback, lock
   reservation, FileStore walk, or daemon mutation.
10. Never emits DSNs or executable SQL.
11. Redact any accidental DSN substring if a future helper is reused;
    this command has no `--dsn` flag.

### C. Successful CLI / loopback envelope

```json
{
  "format": "go-metin2-sql-drivers-v1",
  "drivers": []
}
```

Stock `go build ./cmd/metin2-migrate` / stock `gamed` / stock `authd`:

```json
{
  "format": "go-metin2-sql-drivers-v1",
  "drivers": []
}
```

Build-tagged harness (`//go:build sqlite_harness`, blank-import
`modernc.org/sqlite`) includes at least:

```json
{
  "format": "go-metin2-sql-drivers-v1",
  "drivers": ["sqlite"]
}
```

Additional test-only registered names (CLI unit tests) appear in
lexicographic order. No extra JSON fields. `drivers` is always an array,
never `null`, never omitted.

This envelope is **not** a retained-file inspector. There is no
`drivers-status` companion and no `present: false` missing-path object.
The command inspects the **current process**, not a file.

### D. Loopback `GET /local/db/drivers` (gamed only)

Follow the local-only ops pattern:

1. Runtime primitive first (`RegisteredDatabaseDrivers`), not HTTP first.
2. Register through a narrow helper
   `ops.RegisterLocalSQLDriversEndpoint(mux, drivers func() []string)`
   that no-ops when `mux` or `drivers` is nil.
3. Wire it only from
   `minimal.RegisterGamedMigrationQuarantineExportOps` (already the
   single owner for `/local/db/migrations/*` on `gamed`).
4. `authd` keeps `service.Run(...)` / default mux and does **not**
   register this route.
5. Handler contract:
   - method must be `GET` → else `405`
   - non-loopback `RemoteAddr` → `403`
   - success → `200` + `Content-Type: application/json; charset=utf-8`
     + the same `go-metin2-sql-drivers-v1` envelope as the CLI
6. Never opens a DSN, never reads `schema_migrations`, never includes
   configured driver/DSN fields from runtime-config (those stay on
   `GET /local/runtime-config`).
7. Empty linked list is `200` with `"drivers": []`, not `404` / `409`.
8. No request body. Non-empty bodies are ignored (GET); do not add a
   `413` path on this read-only GET.
9. Do **not** add `GET /local/db/drivers/{name}` or a POST mutation.

This is a bootstrap operator path, not a remote admin API. No token
auth, no `X-Forwarded-For` trust, no proxy awareness in this slice.

### E. Printer wiring (same GREEN as the inspector)

`metin2-migrate migration-run-retention` already fail-closes on unset
`$DRIVER` / `$DSN` then runs DSN-touching `ledger-snapshot`. GREEN must
add a matching `drivers --require-driver "$DRIVER"` redirect
**immediately after** those `: "${DRIVER:?...}"` / `: "${DSN:?...}"`
lines and **before** `ledger-snapshot`.

Printer remains print-only and still does not execute apply / snapshot
itself.

Forward and `--allow-rollback` share `renderMigrationRunRetentionScript`;
both directions get the same inspect placement (driver linkage does not
depend on apply direction).

```sh
: "${DRIVER:?export DRIVER to the database/sql driver name}"
: "${DSN:?export DSN to the operator-managed database/sql DSN}"
metin2-migrate drivers \
  --require-driver "$DRIVER" \
  > "$RUN/sql-drivers.json"
metin2-migrate ledger-snapshot \
  --driver "$DRIVER" \
  --dsn "$DSN" \
  > "$RUN/ledger-snapshot.json"
```

Why gated here: the printed script invokes the same `metin2-migrate`
binary that will apply. If that binary was not built with `$DRIVER`,
`set -eu` must stop **before** opening the DSN.

Do **not** print a `curl "$OPS/local/db/drivers"` retain in this GREEN.
Apply is CLI-owned; the loopback surface is for a running `gamed` that
already passed startup driver availability. Adding a curl would force
hermetic stub expansion and would not prove the CLI binary used for
apply. Operators who want the daemon view already have the endpoint
after GREEN and may curl it by hand into `notes.md` without a new
filename in the printer.

Do **not** invent `sql-drivers-status.json`.
Do **not** change `backup-restore-drill` / `import-export-drill`
printers in this GREEN (`import-export-drill` still treats `--driver` as
an opaque literal and does not `sql.Open` at print time; a later slice
may add a matching `--require-driver` hint).

### F. Hermetic `/bin/sh` assertions (same GREEN)

Existing tagged proofs already put `metin2-migrate` built with
`-tags=sqlite_harness` on `PATH` and export `DRIVER=sqlite`. GREEN
**must** keep those proofs green under the extra redirect and assert on
**every** tagged proof (forward tip, rollback-to-zero, intermediate
forward `7`, intermediate rollback `8`):

- `$RUN/sql-drivers.json` exists as a regular file
- envelope is `go-metin2-sql-drivers-v1`
- `drivers` contains `"sqlite"`
- leftover-lock `apply-lock-aside-status.json` stays absent on the
  successful path

Untagged `go test ./internal/migratecli` stays free of the SQLite
dependency. Untagged coverage owns printer stdout shape + ordering plus
inspector unit tests only (including the empty stock list).

Do **not** change the hermetic curl stub unless a test would otherwise
break; printer GREEN does not curl `/local/db/drivers`.

### G. Ordering (untagged printer tests)

Keep today's mkdir → identity/runtime/status-before → daemon logs →
notes → catalog → `DRIVER`/`DSN` require → ledger-snapshot → … order,
with the new inspect line **immediately after** the DRIVER/DSN require
and **before** ledger-snapshot.

Forward, rollback-to-zero, and intermediate printer tests that already
pin the `ledger-snapshot --driver "$DRIVER"` line must also pin:

```
metin2-migrate drivers
  --require-driver "$DRIVER"
  > "$RUN/sql-drivers.json"
```

Stdout still omits SQL / concrete DSN markers and still does not
auto-run `apply-lock-aside`.

### H. Explicit non-goals

- registering `modernc.org/sqlite` (or any other engine) in stock
  `gamed` / `authd` / `metin2-migrate` release builds
- Makefile / CI `GO_TAGS=sqlite_harness` as a default
- choosing MySQL / Postgres / SQLite as **the** production engine
- aliasing `sqlite` and `sqlite3`
- opening a DSN from `drivers` / `GET /local/db/drivers`
- adding `database.linked_drivers` onto `GET /local/runtime-config`
  (keep that snapshot configured-name + `dsn_configured` only)
- a loopback POST / mutation / apply endpoint
- printing a curl retain of `/local/db/drivers`
- `sql-drivers-status` retained-file inspector
- changing `backup-restore-drill` / `import-export-drill` in this GREEN
- leftover-lock auto-delete / auto-run `apply-lock-aside`
- automatic / scheduled execution of printed apply / rollback scripts
- claiming a linked driver proves live FileStores or live ledger rows
- token auth, forwarded-header trust, proxy awareness, remote admin
- secrets in git, metrics/tracing
- broad README churn
- pushing `origin/main`

## Likely files for GREEN (not this freeze)

- `internal/config/service.go` (`RegisteredDatabaseDrivers` /
  `RequireRegisteredDatabaseDriver`; optional reuse from
  `ValidateDatabaseDriverAvailability`)
- `internal/config/service_test.go`
- `internal/migratecli/drivers.go` (new)
- `internal/migratecli/drivers_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage)
- `internal/migratecli/migration_run_retention.go`
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
  (`$RUN/sql-drivers.json` assertions; no curl-stub change required)
- `internal/ops/pprofmux.go` (`RegisterLocalSQLDriversEndpoint`)
- `internal/ops/pprofmux_test.go` (GET 200 / 403 / 405; empty list;
  registered names)
- `internal/minimal/gamed_migration_ops.go` (gamed-only registration)
- focused authd/default-mux coverage proving the route is absent
- `docs/development.md`
- `docs/debugging-and-profiling.md` (`GET /local/db/drivers` section
  once GREEN lands)
- `docs/workflow/lab-deployment-topology.md` (tree listing only once
  GREEN produces `sql-drivers.json`)
- `docs/workflow/migration-apply-runbook.md`
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused untagged coverage:

- `go test ./internal/config -run 'RegisteredDatabaseDrivers|RequireRegisteredDatabaseDriver|ValidateDatabaseDriverAvailability' -count=1`
- CLI `drivers` with no flags → empty `drivers` array, no `sql.Open`
- CLI after registering a test driver → sorted list includes that name,
  still no DSN open
- `--require-driver` matching a registered test driver → exit `0`
- `--require-driver go_metin2_missing_driver` → exit `1`, empty stdout
- `--require-driver` empty / extra args / unknown flag → exit `2`
- usage / unknown-command text lists `drivers`
- `migration-run-retention` forward + rollback printers emit gated
  `--require-driver "$DRIVER"` immediately after the DRIVER/DSN require
  and before `ledger-snapshot`
- stdout still omits DSNs / executable SQL

Loopback coverage:

- `GET /local/db/drivers` from `127.0.0.1` → `200` + envelope
- non-loopback → `403`
- `POST` → `405`
- gamed registration path exposes the route; default/authd mux does not

Hermetic tagged coverage:

- `go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1`
- retained `$RUN/sql-drivers.json` includes `"sqlite"`
- untagged `go test ./internal/migratecli` stays free of SQLite

Validation for GREEN:

```bash
go test ./internal/config -run 'RegisteredDatabaseDrivers|RequireRegisteredDatabaseDriver|ValidateDatabaseDriverAvailability' -count=1
go test ./internal/migratecli -run 'Drivers|MigrationRunRetentionPrints|RejectsUnknownCommandMentionsDrivers' -count=1
go test ./internal/ops -run 'LocalSQLDrivers' -count=1
go test ./internal/minimal -run 'GamedMigration.*Drivers|Authd.*Drivers' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/config/*.go internal/migratecli/*.go internal/ops/pprofmux.go internal/ops/pprofmux_test.go internal/minimal/gamed_migration_ops.go
git diff --check
```

## Status

GREEN on `lane/persistence` in `c19e5565` (`feat(persistence): inspect linked SQL drivers`).

- `metin2-migrate drivers [--require-driver <name>]` reports the sorted linked
  `database/sql` names without opening a target; `--require-driver` fails
  closed before a DSN-touching command when its name is absent.
- Gamed-only, loopback `GET /local/db/drivers` exposes the same
  `go-metin2-sql-drivers-v1` envelope. Default/authd muxes omit the route.
- Stock binaries remain empty (`drivers: []`); harness-only `sqlite` remains
  opt-in `//go:build sqlite_harness`.
- Printed `migration-run-retention` scripts retain `$RUN/sql-drivers.json`
  immediately after DRIVER/DSN require and before `ledger-snapshot`; they do
  not curl the daemon route or emit `sql-drivers-status.json`.
- Focused config/CLI/ops/minimal coverage, tagged SQLite hermetic retention,
  touched-package tests, vet, formatting, and direct stock/tagged CLI runs
  are GREEN.
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.

Follow-up owned separately after GREEN: choose and document a production
DB engine/driver only when repository or migrator work needs a stock
default; keep advisory-lock coverage and SQL-backed runtime stores
deferred until that choice exists.

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / empty
  stock list / harness `sqlite` name / loopback path / gamed-only
  registration / printer placement / hermetic filename / leftover-lock
  ordering / no-curl rule
- Track E / migration-contract point at this freeze as the next GREEN
  target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not register a production driver in the freeze commit or the GREEN
  commit.
- Do not open a database from `drivers` / `GET /local/db/drivers`.
- Do not print a curl retain of `/local/db/drivers` in GREEN.
- Do not change `backup-restore-drill` in GREEN.
- Do not auto-run `migration-run-retention` from CLI.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not add a working CLI example that claims the printer already emits
  `sql-drivers.json` unless GREEN actually produced them.
