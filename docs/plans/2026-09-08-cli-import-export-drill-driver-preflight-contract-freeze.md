# CLI import-export-drill linked-driver preflight contract freeze — 2026-09-08

## Objective

Freeze a small, print-only **linked `database/sql` driver preflight** for
`metin2-migrate import-export-drill`. The generated recovery/backfill script
must stop on the exact `metin2-migrate` binary that will perform the imports
when its selected `$DRIVER` is not linked — before it requires a target DSN,
reads retained export artifacts, or invokes the mutating `import-export`
command.

This follows GREEN direct SQL-driver gates for `ledger-snapshot`, `status`,
`apply`, and `import-export`, plus the existing linked-driver inspector:

```bash
metin2-migrate drivers --require-driver "$DRIVER"
```

It does not select or register a production driver, execute imports from the
printer, add a daemon endpoint, or turn bootstrap FileStores into SQL
repositories.

## Why this follow-up is needed

`import-export` now validates driver linkage immediately before its existing
`sql.Open`. That protects every individual import, but an operator running a
multi-kind printed drill otherwise learns that the binary cannot import only
after the script has required the DSN and begun its retained-artifact walk.

The existing SQL-driver inspector contract deliberately left
`import-export-drill` unchanged until this follow-up. This is a narrow
consistency improvement for the PvE restart/recovery path: a retained export
tree for roster, item, point, quest, safebox, ground-item, ticket, template,
or static-content recovery should not require a live target credential merely
to discover that the recovery binary lacks its requested driver.

## Contract to freeze before GREEN

### A. Printer-only scope

Keep the command and all existing flags unchanged:

```bash
metin2-migrate import-export-drill \
  --export-tree <absolute-retained-tree> \
  --driver <database/sql-driver> \
  --i-confirm-print-sql-import-drill \
  [--dsn-env METIN2_IMPORT_DSN] \
  [--i-confirm-print-scoped-replace] \
  [--i-confirm-print-two-phase-wipe-roster-reimport]
```

The printer continues to accept `--driver` as an opaque literal. At generation
time it must not call `config.RequireRegisteredDatabaseDriver`, open or ping a
database, require the DSN environment variable, inspect the export tree, or
attempt an import. That preserves use of the printer on a build host distinct
from the later execution host.

The printed script remains `set -eu` and runs the standard CLI inspector on
its execution host. No new printer flag, retained artifact filename, or
`sql-drivers-status` format is introduced.

### B. Required printed ordering

After the script assigns its quoted `EXPORT_TREE`, `DRIVER`, and `DSN_ENV`
variables, it must emit this preflight before expanding `$DSN`:

```sh
# The same metin2-migrate binary that later executes import-export performs
# this check. Its nonzero exit stops the set -eu script before the DSN is read.
metin2-migrate drivers --require-driver "$DRIVER" > /dev/null
DSN="${METIN2_IMPORT_DSN:?METIN2_IMPORT_DSN must be set to the import target DSN}"
```

The real renderer substitutes the validated `--dsn-env` name in the expansion.
The preflight output is deliberately discarded rather than written into the
export tree: `drivers` describes the current executable process, not a durable
migration/export artifact, and the prior driver contract explicitly provides
no `drivers-status` companion.

With `set -eu`:

- an unlinked, malformed, or otherwise rejected `$DRIVER` stops before a DSN
  value is expanded, before `export-tree-status`, before a quarantine file is
  read, and before any `import-export` call;
- a linked driver proceeds to the existing DSN check and unchanged recovery
  flow;
- a missing DSN still produces the existing shell parameter-expansion failure,
  but only after driver linkage succeeds.

The command must remain exactly `metin2-migrate drivers --require-driver
"$DRIVER"` (with only its stdout redirected). It must not inspect the gamed
loopback endpoint: the CLI binary, rather than a potentially different daemon
binary, owns the later SQL imports.

### C. Compatibility and safety boundaries

The same preflight placement applies to all three rendering modes:

1. default insert-only import order;
2. opt-in scoped-replace order; and
3. opt-in two-phase wipe → roster → omit-roster reimport.

It does not change export-tree status files/gates, kind ordering, quarantine
requirements, scoped-replace confirmation, two-phase wipe artifacts, SQL
transaction behavior, DSN redaction, or individual `import-export` linkage
gates. A passing preflight proves only that the executing CLI has a linked
name; it does not prove target reachability, credentials, schema shape,
retained-export validity, backup restoreability, or safe replacement scope.

Do not add an engine alias (`sqlite` and `sqlite3` remain distinct), a stock
SQL-driver blank import, automatic import/retry/rollback, a remote admin API,
token authentication, proxy trust, or a daemon mutation endpoint.

## TDD plan for GREEN

1. Untagged renderer tests cover default, scoped-replace, and two-phase modes:
   each contains `drivers --require-driver "$DRIVER" > /dev/null` after
   variable assignment and before the DSN expansion / first import.
2. Untagged tests confirm printer generation succeeds with an opaque unlinked
   literal, proving the printer itself performs no linkage check or database
   action.
3. Tagged SQLite hermetic script tests execute every existing drill mode with
   `DRIVER=sqlite`; they stay green and thereby prove the preflight runs under
   the same PATH-selected harness binary as imports.
4. Add a hermetic negative test with an unlinked driver and an unset DSN
   environment variable: the generated script must fail at `drivers` with the
   existing unavailable-driver error, not the missing-DSN shell error, and
   must not create before/after export-tree status or import-result files.
5. Existing direct `import-export` tests remain green, preserving their
   independently enforced pre-`sql.Open` gate.

Expected validation after GREEN:

```bash
go test ./internal/migratecli -run 'ImportExportDrill|Drivers|ImportExport' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

## Likely files for GREEN

- `internal/migratecli/import_export_drill.go`
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/workflow/migration-apply-runbook.md`
- this plan (flip freeze → Done)

## Status

Contract frozen on `lane/persistence`; GREEN is intentionally deferred to the
next cohesive migration-CLI recovery slice. The existing `drivers` inspector
and individual `import-export` direct gate remain available for manual
preflight in the meantime.

## Exit criteria for this freeze

- exact command, placement, stdout handling, and all three printer modes are
  explicit;
- generation-time versus script-execution-time behavior is distinguished;
- pre-DSN failure and no-new-artifact semantics are explicit;
- tagged positive and negative hermetic proof boundaries are named;
- no production Go code, production driver registration, target DSN, or daemon
  surface is introduced by this freeze.

## Anti-goals / ordering constraints

- Do not open a GREEN implementation before this contract is committed.
- Do not change `migration-run-retention`, `backup-restore-drill`, or
  `export-quarantine-drill` in this slice.
- Do not treat a linked driver as proof that a target database or FileStore
  restore is healthy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
