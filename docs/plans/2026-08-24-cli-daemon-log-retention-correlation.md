# CLI Daemon Log Retention Correlation — 2026-08-24

## Objective

Close the remaining operator-correlation gap after lab daemon JSON stdout
capture: the read-only `metin2-migrate backup-restore-drill` and
`migration-run-retention` printers already retain both-daemon build-info,
runtime-config, persistence status, and a `notes.md` stub, but they still do
not print the optional `/var/log/metin2/{authd,gamed}.log` retain steps that
[lab deployment topology](../workflow/lab-deployment-topology.md) and
[production observability](../workflow/production-observability.md) now name
beside backup / migration evidence.

## Why now

- Track E just shipped disabled-by-default unit samples that append redacted
  JSON process logs under `/var/log/metin2/`.
- The topology correlation checklist already asks operators to retain matching
  stdout JSON logs with the same `service` / `version` / `commit` /
  `build_date` attrs.
- Without printer updates, operators still invent host-local copy steps (or
  skip log evidence) during reconnect/restart and migration windows.

## Contract frozen by this slice

1. Both printers gain optional absolute log path flags with lab defaults:
   - `--gamed-log-path` defaults to `/var/log/metin2/gamed.log`
   - `--authd-log-path` defaults to `/var/log/metin2/authd.log`
2. Paths must be absolute after trim/clean; empty / relative / `.` fail closed
   with exit `1` and no stdout script.
3. Printed scripts set `GAMED_LOG` / `AUTHD_LOG` from those validated paths.
4. After identity / runtime / persistence-status-before retain (and before the
   `notes.md` stub), both scripts print an optional retain block:
   - `if [ -f "$GAMED_LOG" ]; then cp -p "$GAMED_LOG" "$BASE/gamed.log"; fi`
   - `if [ -f "$AUTHD_LOG" ]; then cp -p "$AUTHD_LOG" "$BASE/authd.log"; fi`
   - migration-run printer uses `$RUN/...` instead of `$BASE/...`
5. Missing log files stay non-fatal (`set -eu` + `[ -f ]`) so hosts that have
   not renamed the unit samples still run the rest of the drill/runbook.
6. Printers still never execute HTTP, never open a database, never embed DSNs /
   executable SQL, never wipe live data trees, and never auto-ship / rotate
   logs themselves.
7. Lab topology retention trees list `gamed.log` / `authd.log` beside the
   existing correlation artifacts.
8. Docs (`development.md`, backup/restore drill, migration apply runbook,
   production observability, lab daemon unit samples, Track E wording) name the
   optional retain steps and flags.

## What this is not yet

- remote log shipping / SIEM / OpenTelemetry
- treating retained log copies as proof that restore / apply succeeded
- automatic / scheduled artifact GC deletion
- FreeBSD port / `pkg` enable defaults
- SQL import/backfill or a driver-backed harness
- inventing a daemon `/local/...` log-download endpoint

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful `backup-restore-drill` / `migration-run-retention` printers include
  default `GAMED_LOG` / `AUTHD_LOG`, the conditional `cp -p` retains, and
  ordering after persistence-status-before / before `notes.md`
- custom `--gamed-log-path` / `--authd-log-path` are honored
- relative / blank log paths fail closed with no stdout
- usage text lists the new flags
- stdout still omits SQL / concrete DSN markers

Validation for this slice:

- `go test ./internal/migratecli -run 'BackupRestoreDrill|MigrationRunRetention|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep FreeBSD port / `pkg` enable defaults deferred.
2. Keep remote log shipping / metrics exporters deferred.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. Keep SQL import/backfill deferred until a driver-backed harness exists.
5. ~~Optional later: fold the same `--gamed-log-path` / `--authd-log-path`
   flags into the tree-owned `contrib/lab-retention-gc` print helper.~~ Done:
   see [contrib retention helper daemon log paths](2026-08-24-contrib-retention-helper-daemon-log-paths.md).
6. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`.~~
   Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
