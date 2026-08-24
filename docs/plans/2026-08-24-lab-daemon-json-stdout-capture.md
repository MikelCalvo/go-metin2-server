# Lab Daemon JSON Stdout Capture — 2026-08-24

## Objective

Close the production-ops gap between the owned JSON process-logger contract
(`authd` / `gamed` write redacted JSON to stdout) and the disabled-by-default
lab daemon unit samples, which currently drop FreeBSD stdout via
`daemon -f` and leave systemd on the journal without a tree-owned file path
operators can retain beside backup / migration evidence.

## Why now

- Track E already owns lab topology, retention/GC print samples, daemon unit
  samples, and production observability conventions, but reconnect/restart
  operators still have no reviewable place to keep daemon JSON logs under
  `/var/log/metin2/` with rotation hooks that preserve the loopback-only /
  no-DSN / no-auto-GC hard rules.
- FreeBSD `daemon -f` alone redirects stdout/stderr to `/dev/null`, so the
  current `rc.d` samples silently discard the observability contract unless an
  operator invents a host-local wrapper.
- systemd `Type=simple` already keeps stdout in the journal; this slice adds an
  explicit append path without inventing remote log shipping.

## Contract frozen by this slice

1. FreeBSD `rc.d` samples append JSON stdout/stderr with
   `daemon -f -H -o /var/log/metin2/{authd,gamed}.log` (override via
   `*_logfile` rc knobs defaulting to those absolute paths).
2. `-H` stays set so `newsyslog` / operator `SIGHUP` can reopen the log file
   without restarting the PvE vertical.
3. systemd unit samples set:
   - `StandardOutput=append:/var/log/metin2/{authd,gamed}.log`
   - `StandardError=append:/var/log/metin2/{authd,gamed}.log`
4. New tree fragments:
   - `contrib/lab-daemons/newsyslog.conf.d/metin2-daemons.conf.sample`
   - `contrib/lab-daemons/logrotate.d/metin2-daemons.conf.sample`
5. FreeBSD `newsyslog` sample rotates daily (`@T00`), keeps 7 copies, uses
   `JH` (bzip2 + SIGHUP), and signals `/var/run/{authd,gamed}.pid` with
   signal `1` so `daemon -H` reopens the file.
6. Linux `logrotate` sample is weekly, `rotate 7`, `copytruncate`, and never
   shells printer / migrate / GC commands.
7. README / workflow / observability docs name `/var/log/metin2/` beside the
   data / backup trees and keep enable defaults `NO`.
8. Focused `internal/migratecli` coverage fail-closes if samples regress the
   hard rules (no DSN, no enable-YES, no stdout pipe to a shell, required
   `-o` / `StandardOutput=append` / rotation fragments present).

## What this is not yet

- FreeBSD port / `pkg` that installs or enables units / newsyslog by default
- flipping `*_enable` defaults to `YES`
- remote log shipping / SIEM / OpenTelemetry
- embedding DSNs, passwords, login keys, or executable SQL
- daemon startup auto-migration / auto-restore / automatic artifact GC
- claiming journald capture is removed (append is additive evidence)

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabDaemon' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep FreeBSD port / `pkg` enable defaults deferred.
2. Keep remote log shipping / metrics exporters deferred.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. ~~Optional later: fold `/var/log/metin2/{authd,gamed}.log` into the offline
   backup / migration-run retention printers.~~ Done: see
   [CLI daemon log retention correlation](2026-08-24-cli-daemon-log-retention-correlation.md).
5. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`.~~
   Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
