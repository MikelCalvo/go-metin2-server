# Lab Daemon Unit Samples — 2026-08-23

## Objective

Freeze **disabled-by-default** FreeBSD `rc.d` and systemd unit samples for
`authd` / `gamed` so operators can start the lab PvE vertical against the
absolute store layout in [lab deployment topology](lab-deployment-topology.md)
without packaging that enables services, embeds DSNs, or invents remote admin.

See also:

- [contrib lab daemon rc.d / systemd samples plan](../plans/2026-08-23-contrib-lab-daemon-rc-systemd-samples.md)
- [lab daemon JSON stdout capture plan](../plans/2026-08-24-lab-daemon-json-stdout-capture.md)
- tree fragments under [`contrib/lab-daemons/`](../../contrib/lab-daemons/)
- sibling print-only retention/GC samples in [lab retention / GC unit samples](lab-retention-gc-unit-samples.md)
- [production observability conventions](production-observability.md)

## Hard rules

1. Units stay `.sample` until an operator renames them after review.
2. FreeBSD `rc.conf` knobs default to `authd_enable="NO"` and
   `gamed_enable="NO"`. Packaging / ports must not flip these to `YES`.
3. Do not `systemctl enable --now` or `sysrc …_enable=YES` from packaging.
4. Ops listeners stay loopback-only (`127.0.0.1:6061` / `127.0.0.1:6060`).
5. Never embed DSNs, passwords, login keys, or executable SQL.
6. Never `ExecStart` `metin2-migrate` apply / backup / GC from daemon units.
7. Never pipe unit stdout into a shell; never wipe `/var/metin2/data` on stop.
8. File-backed stores keep dedicated parents under `/var/metin2/data/`.
9. Capture redacted JSON process logs under `/var/log/metin2/` (never under
   live data / backup trees); rotation samples must not shell migrate / GC.

## Tree layout

```text
contrib/lab-daemons/
  README.md
  env/metin2-authd.env.sample
  env/metin2-gamed.env.sample
  rc.d/authd.sample
  rc.d/gamed.sample
  rc.d/rc.conf.sample
  systemd/authd.service.sample
  systemd/gamed.service.sample
  systemd/authd.service.d/lab-store.conf.sample
  systemd/gamed.service.d/lab-store.conf.sample
  newsyslog.conf.d/metin2-daemons.conf.sample
  logrotate.d/metin2-daemons.conf.sample
```

## Env contract (no secrets)

Shared / authd (`env/metin2-authd.env.sample`):

- `METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/data/login-tickets`
- `METIN2_ACCOUNT_STORE_DIR=/var/metin2/data/accounts`
- `METIN2_AUTHD_PPROF_ADDR=127.0.0.1:6061`
- `METIN2_AUTHD_LEGACY_ADDR=:11002`

gamed (`env/metin2-gamed.env.sample`) adds dedicated file stores:

- `METIN2_GAMED_STATIC_ACTOR_STORE_PATH=/var/metin2/data/static-actors/static-actors.json`
- `METIN2_GAMED_INTERACTION_STORE_PATH=/var/metin2/data/interactions/interaction-definitions.json`
- `METIN2_GAMED_ITEM_TEMPLATE_STORE_PATH=/var/metin2/data/item-templates/item-templates.json`
- `METIN2_GAMED_QUEST_STATE_STORE_PATH=/var/metin2/data/quest-state/quest-state.json`
- `METIN2_GAMED_GROUND_ITEM_STORE_PATH=/var/metin2/data/ground-items/ground-items.json`
- `METIN2_GAMED_SAFEBOX_STORE_PATH=/var/metin2/data/safebox/safebox.json`
- `METIN2_GAMED_PPROF_ADDR=127.0.0.1:6060`
- `METIN2_GAMED_LEGACY_ADDR=:13000`

`authd` and `gamed` must share the same login-ticket and account-store dirs.
Samples intentionally omit every `METIN2_*_DB_DSN` / `METIN2_*_DB_DRIVER`.

## FreeBSD rc.d

Preferred on FreeBSD lab hosts:

```bash
install -d -m 0755 /usr/local/etc/rc.d
install -m 0755 contrib/lab-daemons/rc.d/authd.sample \
  /usr/local/etc/rc.d/authd.sample
install -m 0755 contrib/lab-daemons/rc.d/gamed.sample \
  /usr/local/etc/rc.d/gamed.sample
# rename without .sample only after review
install -m 0644 contrib/lab-daemons/rc.d/rc.conf.sample \
  /usr/local/etc/rc.conf.metin2.sample
# merge reviewed knobs into /etc/rc.conf; keep *_enable="NO"
install -d -m 0750 /var/log/metin2
install -d -m 0755 /usr/local/etc/newsyslog.conf.d
install -m 0644 \
  contrib/lab-daemons/newsyslog.conf.d/metin2-daemons.conf.sample \
  /usr/local/etc/newsyslog.conf.d/metin2-daemons.conf.sample
# rename without .sample only after review
```

Contract:

1. Scripts source `/etc/rc.subr`, set `rcvar`, default `: "${…_enable:=NO}"`, and
   launch `/usr/local/bin/{authd,gamed}` through `/usr/sbin/daemon`.
2. Optional env file `/etc/metin2/metin2-{authd,gamed}.env` is sourced in
   `start_precmd` when present.
3. Stop uses `rc.subr` / `SIGTERM` only; scripts never wipe store trees.
4. JSON stdout/stderr append with
   `daemon -f -H -o /var/log/metin2/{authd,gamed}.log` (override via
   `*_logfile`, defaulting to those absolute paths). `-H` lets
   `newsyslog` reopen the file with signal `1` against `/var/run/{authd,gamed}.pid`.

## systemd

```bash
install -m 0644 contrib/lab-daemons/systemd/*.sample /etc/systemd/system/
install -d -m 0755 /etc/systemd/system/authd.service.d
install -d -m 0755 /etc/systemd/system/gamed.service.d
install -m 0644 \
  contrib/lab-daemons/systemd/authd.service.d/lab-store.conf.sample \
  /etc/systemd/system/authd.service.d/lab-store.conf.sample
install -m 0644 \
  contrib/lab-daemons/systemd/gamed.service.d/lab-store.conf.sample \
  /etc/systemd/system/gamed.service.d/lab-store.conf.sample
# rename without .sample only after review; do NOT systemctl enable --now yet
install -d -m 0750 /var/log/metin2
install -d -m 0755 /etc/logrotate.d
install -m 0644 \
  contrib/lab-daemons/logrotate.d/metin2-daemons.conf.sample \
  /etc/logrotate.d/metin2-daemons.conf.sample
# rename without .sample only after review
```

Contract:

1. `Type=simple`, `ExecStart=/usr/local/bin/{authd,gamed}`,
   `RequiresMountsFor=/var/metin2`, `KillSignal=SIGTERM`.
2. Main unit files have no inline `Environment=` lines.
3. Lab store / ops env comes only from
   `EnvironmentFile=-/etc/metin2/metin2-{authd,gamed}.env` drop-ins.
4. Units append JSON stdout/stderr with
   `StandardOutput=append:/var/log/metin2/{authd,gamed}.log` and matching
   `StandardError=append:…` (journal capture remains additive).

## Log rotation samples

FreeBSD `newsyslog.conf.d/metin2-daemons.conf.sample`:

- daily `@T00`, keep 7, flags `JH`, pidfiles `/var/run/{authd,gamed}.pid`,
  signal `1`
- never shells `metin2-migrate`, backup, apply, or GC

Linux `logrotate.d/metin2-daemons.conf.sample`:

- weekly, `rotate 7`, `copytruncate`, `create 0640 metin2 metin2`
- no `postrotate` shell that invokes migrate / GC / apply

## What this is not yet

- FreeBSD port / `pkg` that installs **enabled** `rc.d` / systemd units
- flipping `authd_enable` / `gamed_enable` to `YES` by default
- DB driver/DSN embedding or daemon startup auto-migration
- remote admin, metrics exporters, or multi-host orchestration
- automatic / scheduled artifact GC deletion
- remote log shipping / SIEM sinks

Operators can still fold the optional `/var/log/metin2/{authd,gamed}.log`
files into backup / migration retention trees through the offline printers
documented in [CLI daemon log retention correlation](../plans/2026-08-24-cli-daemon-log-retention-correlation.md).
