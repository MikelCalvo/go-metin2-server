# Contrib Lab Daemon rc.d / systemd Samples — 2026-08-23

## Objective

Ship **disabled-by-default** FreeBSD `rc.d` and systemd unit samples for
`authd` / `gamed` under `contrib/lab-daemons/`, matching the already-owned lab
topology absolute store paths and loopback ops binds — without packaging that
enables services, embeds DSNs, or invents remote admin / auto-migration.

## Why now

- Track E already owns lab topology, retention/GC print samples, and restart
  rematerialize for the PvE vertical, but operators still have no tree-owned
  daemon start fragments on FreeBSD (`rc.d`) or Linux (systemd).
- Recent follow-ups deferred FreeBSD port / `pkg` enable defaults. This slice
  stays narrower: reviewable `.sample` units operators copy by hand, with
  `*_enable="NO"` / no `systemctl enable --now` from packaging.
- Automatic GC execution, SQL import, remote admin, and DB DSN embedding stay
  deferred.

## Contract frozen by this slice

1. New tree under `contrib/lab-daemons/`:
   - `README.md`
   - `env/metin2-authd.env.sample`
   - `env/metin2-gamed.env.sample`
   - `rc.d/authd.sample`
   - `rc.d/gamed.sample`
   - `rc.d/rc.conf.sample`
   - `systemd/authd.service.sample`
   - `systemd/gamed.service.sample`
   - `systemd/authd.service.d/lab-store.conf.sample`
   - `systemd/gamed.service.d/lab-store.conf.sample`
2. FreeBSD `rc.conf.sample` defaults `authd_enable="NO"` and
   `gamed_enable="NO"`. Packaging / ports must not flip these to `YES`.
3. `rc.d` scripts use `rc.subr`, default enable `NO`, and
   `command=/usr/local/bin/{authd,gamed}`.
4. systemd units are `Type=simple`, `ExecStart=/usr/local/bin/{authd,gamed}`,
   `RequiresMountsFor=/var/metin2`, stop via `SIGTERM`, and carry **no** inline
   `Environment=` lines; lab store/ops env comes only from
   `EnvironmentFile=-/etc/metin2/metin2-{authd,gamed}.env` drop-ins.
5. Env samples document absolute `/var/metin2/data/...` paths and loopback
   pprof binds only. No `METIN2_*_DB_DSN`, passwords, login keys, or SQL.
6. Focused `internal/migratecli` coverage fail-closes if samples are missing or
   regress the hard rules.
7. Workflow / development docs point at the install path; retention/GC print
   samples remain under `contrib/lab-retention-gc/`.

## What this is not yet

- FreeBSD port / `pkg` that installs or enables `rc.d` / systemd by default
- flipping sample `*_enable` defaults to `YES`
- embedding DB driver/DSN in unit or env samples
- daemon startup auto-migration or auto-restore beyond owned rematerialize
- remote admin, metrics exporters, or multi-host orchestration
- automatic / scheduled artifact GC deletion

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabDaemon' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep FreeBSD port / `pkg` enable defaults deferred.
2. Keep SQL import/backfill deferred until a driver-backed harness exists.
3. Keep automatic / scheduled execution of printed triage scripts deferred.
4. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`.~~
   Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
