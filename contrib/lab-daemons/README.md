# Lab daemon rc.d / systemd unit samples

Tree-owned, **disabled-by-default** fragments matching
[`docs/workflow/lab-daemon-unit-samples.md`](../../docs/workflow/lab-daemon-unit-samples.md)
and the absolute store layout in
[`docs/workflow/lab-deployment-topology.md`](../../docs/workflow/lab-deployment-topology.md).

These files exist so FreeBSD / systemd lab hosts can copy reviewable `.sample`
units and env files without inventing packaging that enables services, embeds
DSNs, or auto-runs migration / backup / GC from daemon start. Units expect
installed binaries at `/usr/local/bin/authd` and `/usr/local/bin/gamed`
(from `make build` / operator install).

JSON process logs follow
[`docs/workflow/production-observability.md`](../../docs/workflow/production-observability.md):
FreeBSD `rc.d` samples append stdout/stderr with `daemon -f -H -o
/var/log/metin2/{authd,gamed}.log`; systemd samples append the same paths via
`StandardOutput=` / `StandardError=`. Rotation samples live under
`newsyslog.conf.d/` (FreeBSD) and `logrotate.d/` (Linux).

Retention / GC print-only samples remain under
[`contrib/lab-retention-gc/`](../lab-retention-gc/).

## Install (manual, review first)

```bash
install -d -m 0750 /var/metin2/data/login-tickets
install -d -m 0750 /var/metin2/data/accounts
install -d -m 0750 /var/metin2/data/static-actors
install -d -m 0750 /var/metin2/data/interactions
install -d -m 0750 /var/metin2/data/item-templates
install -d -m 0750 /var/metin2/data/quest-state
install -d -m 0750 /var/metin2/data/ground-items
install -d -m 0750 /var/metin2/data/safebox
install -d -m 0750 /var/log/metin2
install -d -m 0750 /etc/metin2

install -m 0640 contrib/lab-daemons/env/metin2-authd.env.sample \
  /etc/metin2/metin2-authd.env.sample
install -m 0640 contrib/lab-daemons/env/metin2-gamed.env.sample \
  /etc/metin2/metin2-gamed.env.sample
# review, then:
#   cp /etc/metin2/metin2-authd.env.sample /etc/metin2/metin2-authd.env
#   cp /etc/metin2/metin2-gamed.env.sample /etc/metin2/metin2-gamed.env

# FreeBSD rc.d (preferred on FreeBSD lab hosts; stays NO until reviewed)
install -d -m 0755 /usr/local/etc/rc.d
install -m 0755 contrib/lab-daemons/rc.d/authd.sample \
  /usr/local/etc/rc.d/authd.sample
install -m 0755 contrib/lab-daemons/rc.d/gamed.sample \
  /usr/local/etc/rc.d/gamed.sample
# rename without .sample only after review:
#   cp /usr/local/etc/rc.d/authd.sample /usr/local/etc/rc.d/authd
#   cp /usr/local/etc/rc.d/gamed.sample /usr/local/etc/rc.d/gamed
install -m 0644 contrib/lab-daemons/rc.d/rc.conf.sample \
  /usr/local/etc/rc.conf.metin2.sample
# merge reviewed knobs into /etc/rc.conf; keep authd_enable="NO" gamed_enable="NO"
install -d -m 0755 /usr/local/etc/newsyslog.conf.d
install -m 0644 \
  contrib/lab-daemons/newsyslog.conf.d/metin2-daemons.conf.sample \
  /usr/local/etc/newsyslog.conf.d/metin2-daemons.conf.sample
# rename without .sample only after review

# systemd (do NOT systemctl enable --now until reviewed)
install -m 0644 contrib/lab-daemons/systemd/*.sample /etc/systemd/system/
install -d -m 0755 /etc/systemd/system/authd.service.d
install -d -m 0755 /etc/systemd/system/gamed.service.d
install -m 0644 \
  contrib/lab-daemons/systemd/authd.service.d/lab-store.conf.sample \
  /etc/systemd/system/authd.service.d/lab-store.conf.sample
install -m 0644 \
  contrib/lab-daemons/systemd/gamed.service.d/lab-store.conf.sample \
  /etc/systemd/system/gamed.service.d/lab-store.conf.sample
# rename without .sample only after review
install -d -m 0755 /etc/logrotate.d
install -m 0644 \
  contrib/lab-daemons/logrotate.d/metin2-daemons.conf.sample \
  /etc/logrotate.d/metin2-daemons.conf.sample
# rename without .sample only after review
```

## Hard rules

1. Samples stay `.sample`; packaging / ports / pkg must not install **enabled**
   units or flip `authd_enable` / `gamed_enable` to `YES`.
2. Do not `systemctl enable --now` or `sysrc authd_enable=YES` until an
   operator has reviewed the unit text and env files.
3. Ops stay loopback-only (`127.0.0.1:6061` / `127.0.0.1:6060`); never bind
   `0.0.0.0`, `::`, or a public hostname for pprof/ops.
4. Never embed DSNs, passwords, login keys, or executable SQL in unit files,
   `Environment=`, or env samples.
5. Never `ExecStart` `metin2-migrate` apply / backup / GC / aside-rename from
   daemon units.
6. Never pipe unit output into `/bin/sh`, `bash`, `csh`, or `zsh`.
7. Stop path is `SIGTERM` only; units must not wipe `/var/metin2/data`.
8. File-backed stores keep dedicated parents under `/var/metin2/data/`.
9. Daemon JSON stdout stays under `/var/log/metin2/` (never under live data /
   backup trees); rotation samples must not shell migrate / GC / apply.

## What this is not

- packaging that installs **enabled** `rc.d` / systemd units by default
- FreeBSD port / `pkg` enable defaults
- flipping `authd_enable` / `gamed_enable` to `YES` by default
- DB driver/DSN embedding or daemon startup auto-migration
- remote admin, metrics exporters, or multi-host orchestration
- automatic / scheduled artifact GC deletion (see `contrib/lab-retention-gc/`)
- remote log shipping / SIEM sinks
