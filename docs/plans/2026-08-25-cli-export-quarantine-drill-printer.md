# CLI Export → Offline Quarantine Drill Printer — 2026-08-25

## Objective

Add a read-only `metin2-migrate export-quarantine-drill` command so operators can
turn a retained `GET /local/build-info` / `metin2-migrate version` snapshot into
the concrete shell steps for the already-proven loopback export → retained-file →
offline `quarantine-export` handoff across every tip-`0015` migration-shaped
export kind — without starting `gamed`, opening a database, executing the
printed script, or inventing SQL import/backfill.

The hermetic HTTP → offline CLI proof and shared registration helper already
landed. Operators still lacked the path-aware printer sibling that
`backup-restore-drill` and `migration-run-retention` already provide for their
runbooks.

## Why now

- [Hermetic export → offline quarantine-export CLI proof](2026-08-25-hermetic-export-quarantine-offline-cli-proof.md)
  explicitly deferred “a `metin2-migrate export-quarantine-drill` printer that
  emits curl scripts”.
- Migration windows and drained reconnect drills need a repeatable, reviewable
  script that retains all nine tip-`0015` export kinds beside offline quarantine
  summaries under the lab `/var/metin2/exports/YYYYMMDDTHHMMSSZ-<commit12>/` tree.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (auto-run, `rm`, remote admin, SQL import).

## Contract frozen by this slice

```bash
metin2-migrate export-quarantine-drill \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--authd-ops-base-url http://127.0.0.1:6061] \
  [--export-base /var/metin2/exports] \
  [--gamed-log-path /var/log/metin2/gamed.log] \
  [--authd-log-path /var/log/metin2/authd.log]
```

Behavior:

1. `--build-info` is required. `-` reads stdin; any other value opens a regular
   non-symlink file.
2. Input is capped at 64 KiB, must be valid UTF-8, non-empty after trim, not
   literal JSON `null`, and must decode with `DisallowUnknownFields` plus no
   trailing JSON.
3. Snapshot must expose a non-empty trimmed `commit` (same 12-char lab suffix
   contract as the other printers).
4. `--ops-base-url` defaults to `http://127.0.0.1:6060` and `--authd-ops-base-url`
   defaults to `http://127.0.0.1:6061`; both must be absolute `http`/`https` URLs
   with a host and no query/fragment.
5. `--export-base` defaults to `/var/metin2/exports` and must be an absolute
   cleaned path. Daemon log path flags keep the same absolute-path defaults as
   the sibling printers.
6. On success, stdout is a plain-text shell script that:
   - sets `OPS`, `AUTH_OPS`, `EXPORTS_BASE`, daemon log paths, and build identity;
   - creates `$EXPORTS_BASE/<UTC compact timestamp>-$COMMIT12` as `$BASE` with
     one subdirectory per export kind;
   - retains both-daemon build-info, `runtime-config.json`, `healthz`, migration
     catalog, optional daemon JSON logs (missing files non-fatal), and a
     `notes.md` stub;
   - for each tip-`0015` kind, prints:
     - `curl -sS "$OPS/<export-path>" > "$BASE/<kind>/export.json"`
     - `metin2-migrate quarantine-export --kind <kind> --export "$BASE/<kind>/export.json" > "$BASE/<kind>/quarantine.json"`
   - never executes HTTP, never writes files itself, never opens a database,
     never embeds a DSN / executable SQL.
7. Covered kinds / export paths (order fixed):
   - `account-character-roster` → `/local/account-store/exports/account-character-roster`
   - `character-item-state` → `/local/account-store/exports/character-item-state`
   - `character-point-state` → `/local/account-store/exports/character-point-state`
   - `auth-login-ticket-handoff` → `/local/login-tickets/exports/auth-login-ticket-handoff`
   - `character-quest-state` → `/local/quest-state/exports/character-quest-state`
   - `character-safebox-state` → `/local/safebox-store/exports/character-safebox-state`
   - `item-template-state` → `/local/item-templates/exports/item-template-state`
   - `static-actor-content-state` → `/local/static-actors/exports/static-actor-content-state`
   - `bootstrap-ground-item-state` → `/local/ground-items/exports/bootstrap-ground-item-state`
8. On contract failure, exit `1` with a short stderr reason and **no** stdout
   script.
9. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists
   `export-quarantine-drill`.

## What this is not yet

- automatic / scheduled execution of the printed script
- ~~hermetic `/bin/sh` execution proof against drained loopback ops muxes~~ Done: see
  [hermetic export-quarantine drill HTTP execution proof](2026-08-25-hermetic-export-quarantine-drill-http-execution-proof.md)
- ~~folding the printer into `contrib/lab-retention-gc` print-only samples~~ Done: see
  [contrib export-quarantine drill print helper](2026-08-25-contrib-export-quarantine-drill-print-helper.md)
- SQL import/backfill from quarantined exports
- ~~`rm` of aside-renamed trees~~ Done for confirmation-gated print-only `artifact-gc-aside-purge` — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
- FreeBSD port / `pkg` enable defaults
- remote log shipping / metrics exporters
- remote admin authentication
- changing export schemas, quarantine validators, or migration tip `0015`

## Likely files to change

- `internal/migratecli/export_quarantine_drill.go` (new)
- `internal/migratecli/export_quarantine_drill_test.go` (new)
- `internal/migratecli/migratecli.go` (dispatch + usage)
- `docs/plans/2026-08-25-hermetic-export-quarantine-offline-cli-proof.md`
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md`
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for a valid build-info snapshot includes all nine kinds,
  lab retention tree naming, both-daemon build-info retain, and offline
  `quarantine-export` lines
- blank commit / relative export-base / invalid ops URL → exit `1`, no stdout
- malformed / invalid UTF-8 / oversized build-info → exit `1`
- missing flags / unexpected args → exit `2`
- usage text lists `export-quarantine-drill`
- stdout omits SQL / DSN markers and does not claim to perform quarantine itself

Validation for this slice:

- `go test ./internal/migratecli -run 'ExportQuarantineDrill|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- `metin2-migrate export-quarantine-drill` prints the path-aware handoff script
- focused tests green
- docs/topology name the `/var/metin2/exports` tree and the printer
- hermetic proof plan marks the deferred printer follow-up done

## Anti-goals / ordering constraints

- Do not auto-run the printed script from CLI.
- Do not widen registration helpers or change endpoint paths/bodies.
- Do not add SQL import/backfill.
- Do not push `origin/main`; push only `origin/lane/persistence`.
