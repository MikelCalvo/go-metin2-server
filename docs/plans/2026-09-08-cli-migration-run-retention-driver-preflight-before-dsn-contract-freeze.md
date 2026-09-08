# CLI migration-run-retention driver preflight before DSN contract freeze — 2026-09-08

## Objective

Freeze a small ordering hardening for the printed
`metin2-migrate migration-run-retention` apply and rollback scripts: after the
script has created its timestamped retention tree and completed its existing
non-DB evidence retains, it must verify that the importing CLI binary links
`$DRIVER` **before** it expands or requires `$DSN`.

The existing script already retains a successful linked-driver envelope as
`$RUN/sql-drivers.json` and gates it before `ledger-snapshot`; this follow-up
moves that same gate ahead of the target-credential shell expansion. It aligns
the migration-window printer with the landed `import-export-drill` preflight:
an operator using a binary that lacks the selected driver should learn that
without having to provide a target DSN.

This is not production-engine selection, a new retained-file status format,
a DB-backed runtime-store migration, a daemon mutation surface, or a change to
individual direct SQL command gates.

## Why this follow-up is needed

The current rendered ordering is:

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

That already prevents `ledger-snapshot` from opening a DSN under an unlinked
driver, but it still demands an otherwise unused DSN environment value before
the binary-capability failure. The `drivers` command never opens a target and
can make this decision from `$DRIVER` alone.

`import-export-drill` now demonstrates the safer bootstrap ordering: check the
same CLI binary's linked driver first, then expand its import DSN only after
that check succeeds. The migration retention printer should give apply and
rollback operators the same fail-fast property without changing the actual
migration behavior after a successful preflight.

## Contract to freeze before GREEN

### A. Printed ordering

Both normal forward output and `--allow-rollback` output share
`renderMigrationRunRetentionScript`; both must render this exact sequence after
the existing catalog / catalog-status retains and before `ledger-snapshot`:

```sh
: "${DRIVER:?export DRIVER to the database/sql driver name}"
metin2-migrate drivers \
  --require-driver "$DRIVER" \
  > "$RUN/sql-drivers.json"
: "${DSN:?export DSN to the operator-managed database/sql DSN}"
metin2-migrate ledger-snapshot \
  --driver "$DRIVER" \
  --dsn "$DSN" \
  > "$RUN/ledger-snapshot.json"
```

The generated script remains `set -eu`. The `drivers` command must remain the
CLI inspector, not `curl "$OPS/local/db/drivers"`: the `metin2-migrate` binary
that passes the gate is the one that immediately performs
`ledger-snapshot`, `apply`, and post-apply `status`.

`$RUN` is intentionally created earlier, because it also contains identity,
runtime-config, persistence-status, daemon-plan, log, notes, catalog, and
catalog-status retention evidence. This follow-up does not reorder or remove
those non-DB retains.

### B. Success behavior remains unchanged

For a linked driver and a set DSN:

- `sql-drivers.json` remains the existing successful
  `go-metin2-sql-drivers-v1` envelope containing the complete sorted linked
  driver list;
- the subsequent ledger snapshot, plan artifact, apply-preflight, explicit
  `apply`, audit status, post-apply/post-rollback status, and local
  persistence retains keep their existing names, order, flags, and direction
  behavior;
- normal forward-to-tip, rollback-to-zero, intermediate forward, and
  intermediate rollback tagged SQLite drills remain executable with
  `DRIVER=sqlite`;
- no daemon `/local/db/drivers` curl, `sql-drivers-status.json`, driver alias,
  stock driver registration, or new flag is introduced.

A linked driver proves only that this CLI binary has the requested
`database/sql` name. It still does not prove that the eventual DSN is set,
reachable, authorized, schema-compatible, backed up, or safe to mutate.

### C. Failure behavior and partial-run boundary

The printed script must fail in this order:

1. An unset `DRIVER` still stops at the existing DRIVER parameter expansion.
2. A malformed or unlinked `DRIVER` reaches `drivers --require-driver` and
   stops before `$DSN` is expanded, before `ledger-snapshot`, and before every
   plan, preflight, lock, audit, `apply`, or post-apply SQL command.
3. A linked driver with an unset DSN writes its successful
   `sql-drivers.json`, then stops at the existing DSN parameter expansion
   before `ledger-snapshot`.

Shell redirection happens before the `drivers` subprocess runs. Therefore an
unlinked/malformed-driver failure can leave an empty or incomplete
`$RUN/sql-drivers.json` next to the earlier non-DB retained files. That is an
expected **failed partial run tree**, not a valid linked-driver artifact or
migration-window evidence. The script's nonzero exit is authoritative; an
operator must discard/restart the partial tree after correcting the binary or
driver selection. GREEN must not silently add cleanup, retry, automatic
artifact removal, or a new status schema merely to make that partial failure
look successful.

The driver gate must continue to write its existing short `drivers:` error to
stderr and never receive or echo a DSN. `migration-run-retention` itself stays
a print-only command; rendering the script still does not inspect local driver
linkage, open a database, contact an ops endpoint, or require a DSN.

### D. Test plan for GREEN

1. Untagged printer tests assert the exact DRIVER requirement → gated
   `drivers` redirect → DSN requirement → `ledger-snapshot` ordering for both
   forward and rollback rendering. Existing tests continue to assert no
   concrete DSN or executable SQL is printed.
2. The four tagged SQLite success drills (forward tip, rollback zero,
   intermediate forward, intermediate rollback) remain green and retain a
   valid `sql-drivers.json` containing `sqlite`.
3. Add a tagged hermetic negative case that renders a forward script with an
   opaque unlinked driver, executes it with that driver set and `DSN` absent,
   and observes the existing unavailable-driver stderr rather than the missing
   DSN parameter-expansion error.
4. That negative execution proves no `ledger-snapshot.json`, plan, preflight,
   audit, post-status, or migration lock is produced. It may observe an empty
   or incomplete `sql-drivers.json` as documented above; it must not treat it
   as a valid drivers envelope.
5. Existing `drivers`, direct `ledger-snapshot`/`status`/`apply` linkage-gate,
   and `import-export-drill` tests remain green. Their independent contracts
   are not changed by this renderer-only follow-up.

Expected validation after GREEN:

```bash
go test ./internal/migratecli -run 'Drivers|MigrationRunRetention' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Likely files for GREEN

- `internal/migratecli/migration_run_retention.go`
- `internal/migratecli/migration_run_retention_test.go`
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
- `docs/workflow/migration-apply-runbook.md`
- `docs/workflow/lab-deployment-topology.md`
- this plan (flip freeze → Done)

## Status

Implemented in the follow-up GREEN commit. The print-only renderer now emits
the linked-driver gate before the DSN expansion for both forward and rollback
scripts; untagged ordering tests and tagged SQLite positive/negative execution
proofs cover the contract. No stock driver registration, target connection,
daemon mutation endpoint, or retained-file status format was introduced.

## Exit criteria for this freeze

- the exact forward/rollback command order is explicit;
- successful retained-artifact behavior is distinguished from a failed partial
  preflight redirect;
- unlinked-driver-before-DSN and linked-driver/missing-DSN outcomes are named;
- the CLI-binary ownership boundary and no-daemon-curl rule are explicit;
- tagged positive and negative proof boundaries are named;
- no production driver, target DSN, remote admin path, or SQL runtime-store
  claim is introduced.

## Anti-goals / ordering constraints

- Do not implement GREEN before this freeze is committed.
- Do not change `import-export-drill`, `backup-restore-drill`, direct SQL
  command gates, or the loopback SQL-driver endpoint in this slice.
- Do not select/register SQLite, MySQL, PostgreSQL, or any production engine
  in stock binaries.
- Do not add automatic cleanup/retry, stale-lock removal, automatic script
  execution, token authentication, proxy trust, or a remote admin API.
- Do not treat a valid `sql-drivers.json` as proof of target health or backup
  restoreability.
- Do not push `origin/main`; push only `origin/lane/persistence`.
