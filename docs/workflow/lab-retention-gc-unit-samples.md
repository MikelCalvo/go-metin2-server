# Lab Retention / GC Print-Only Unit Samples — 2026-08-23

## Objective

Freeze **print-only** host unit / cron samples for the already-shipped
`metin2-migrate` retention printers. Operators may schedule a reviewable script
dump; they must still inspect and deliberately run (or discard) the printed
shell. This is not automatic artifact GC and not in-tree packaging that enables
timers by default.

See also:

- [lab deployment topology](lab-deployment-topology.md)
- [CLI artifact-retention GC printer plan](../plans/2026-08-22-cli-artifact-retention-gc-printer.md)
- [CLI artifact-retention GC script execution proof](../plans/2026-08-22-cli-artifact-retention-gc-script-execution-proof.md)
- [print-only retention / GC unit samples plan](../plans/2026-08-23-print-only-retention-gc-unit-samples.md)
- [contrib print-only retention / GC unit samples](../plans/2026-08-23-contrib-print-only-retention-gc-unit-samples.md)
- [contrib companion print retention printers](../plans/2026-08-23-contrib-companion-print-retention-printers.md)
- [contrib print helper hermetic execution](../plans/2026-08-23-contrib-print-helper-hermetic-execution.md)
- [contrib runtime-config EnvironmentFile sample](../plans/2026-08-23-contrib-runtime-config-envfile-sample.md)
- [contrib FreeBSD periodic retention / GC print sample](../plans/2026-08-23-contrib-freebsd-periodic-retention-gc-print-sample.md)
- tree fragments under [`contrib/lab-retention-gc/`](../../contrib/lab-retention-gc/) (disabled-by-default `.sample` install copies; never enable from packaging)
- sibling daemon start samples (not print-only) in [lab daemon unit samples](lab-daemon-unit-samples.md) / [`contrib/lab-daemons/`](../../contrib/lab-daemons/)

## Hard rules

1. Units / cron entries may only invoke printer commands (`artifact-retention-gc`,
   `backup-restore-drill`, `migration-run-retention`, optionally `version`).
2. Stdout must land in a dated review path under `/var/metin2/ops-prints/` (or an
   operator-chosen absolute sibling outside live data / retention trees).
3. Never pipe printer stdout into `/bin/sh`, `bash`, `csh`, or `zsh` from the
   unit / cron line.
4. Never `ExecStart` / cron-run `rm`, `rmdir`, `unlink`, `find -delete`, or any
   aside-rename of retention trees without a separate, interactive operator
   confirmation step outside these samples.
5. Never embed DSNs, passwords, login keys, or executable SQL in unit files,
   Environment=, or printed notes.
6. Ops listeners stay loopback-only; these samples do not change bind policy.

## Review directory

```text
/var/metin2/ops-prints/
  YYYYMMDDTHHMMSSZ-<commit12>/
    build-info.json
    artifact-retention-gc-backups.sh
    artifact-retention-gc-migration-runs.sh
    migration-run-retention.sh
    backup-restore-drill.sh          # only when METIN2_RUNTIME_CONFIG is set
    notes.md
```

Create the parent once on the lab host:

```bash
install -d -m 0750 /var/metin2/ops-prints
```

Shared helper used by the samples below. The same script ships as
[`contrib/lab-retention-gc/metin2-print-retention-gc.sh`](../../contrib/lab-retention-gc/metin2-print-retention-gc.sh);
install it as `/usr/local/libexec/metin2-print-retention-gc.sh`, mode `0750`,
owner root:

```bash
#!/bin/sh
# Print-only retention/GC dump. Never executes the printed triage scripts.
set -eu
BIN="${METIN2_MIGRATE_BIN:-/usr/local/bin/metin2-migrate}"
PRINTS_ROOT="${METIN2_OPS_PRINTS_ROOT:-/var/metin2/ops-prints}"
KEEP_DAYS="${METIN2_RETENTION_KEEP_DAYS:-14}"
RUNTIME_CONFIG="${METIN2_RUNTIME_CONFIG:-}"
GAMED_LOG="${METIN2_GAMED_LOG_PATH:-/var/log/metin2/gamed.log}"
AUTHD_LOG="${METIN2_AUTHD_LOG_PATH:-/var/log/metin2/authd.log}"

test -x "$BIN"
test -d "$PRINTS_ROOT"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
TMP_BUILD=$(mktemp)
trap 'rm -f "$TMP_BUILD"' EXIT INT TERM
"$BIN" version >"$TMP_BUILD"
COMMIT=$("$BIN" version | sed -n 's/.*"commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
COMMIT12=$(printf %s "${COMMIT:-unknown}" | cut -c1-12)
OUT="${PRINTS_ROOT}/${STAMP}-${COMMIT12}"
mkdir -p "$OUT"
cp "$TMP_BUILD" "$OUT/build-info.json"

"$BIN" artifact-retention-gc \
  --retention-base /var/metin2/backups \
  --keep-days "$KEEP_DAYS" \
  >"$OUT/artifact-retention-gc-backups.sh"
"$BIN" artifact-retention-gc \
  --retention-base /var/metin2/migration-runs \
  --keep-days "$KEEP_DAYS" \
  >"$OUT/artifact-retention-gc-migration-runs.sh"

"$BIN" migration-run-retention \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path "$GAMED_LOG" \
  --authd-log-path "$AUTHD_LOG" \
  >"$OUT/migration-run-retention.sh"

DRILL_NOTE="backup-restore-drill=skipped (set METIN2_RUNTIME_CONFIG to a retained runtime-config JSON snapshot)"
if [ -n "$RUNTIME_CONFIG" ]; then
  if [ -L "$RUNTIME_CONFIG" ]; then
    DRILL_NOTE="backup-restore-drill=skipped (METIN2_RUNTIME_CONFIG must not be a symlink)"
  elif [ -f "$RUNTIME_CONFIG" ]; then
    "$BIN" backup-restore-drill \
      --runtime-config "$RUNTIME_CONFIG" \
      --build-info "$OUT/build-info.json" \
      --gamed-log-path "$GAMED_LOG" \
      --authd-log-path "$AUTHD_LOG" \
      >"$OUT/backup-restore-drill.sh"
    DRILL_NOTE="backup-restore-drill=printed from METIN2_RUNTIME_CONFIG"
  else
    DRILL_NOTE="backup-restore-drill=skipped (METIN2_RUNTIME_CONFIG is not a regular file)"
  fi
fi

{
  printf 'printed %s\ncommit=%s\nkeep_days=%s\n' "$OUT" "${COMMIT:-unknown}" "$KEEP_DAYS"
  printf '%s\n' "$DRILL_NOTE"
} >"$OUT/notes.md"
chmod 0640 "$OUT"/*.sh "$OUT/build-info.json" "$OUT/notes.md"
printf '%s\n' "$OUT"
```

The helper's only `rm` is the temporary `mktemp` build-info copy via `trap` —
never retention trees or printed scripts. Companion printers are owned by this
helper: `migration-run-retention.sh` always, `backup-restore-drill.sh` only
when `METIN2_RUNTIME_CONFIG` points at a retained non-symlink regular
runtime-config snapshot. Both companion printers receive
`--gamed-log-path` / `--authd-log-path` from `METIN2_GAMED_LOG_PATH` /
`METIN2_AUTHD_LOG_PATH` (defaults `/var/log/metin2/gamed.log` /
`/var/log/metin2/authd.log`) so printed retain scripts can optionally copy
daemon JSON logs when present. The helper never live-fetches ops JSON.
## systemd samples (print-only)

Place as `.sample` files under `/etc/systemd/system/` (or a lab overlay). Do
**not** `systemctl enable --now` until an operator has reviewed the unit text
and accepts that only printers run.

### `metin2-artifact-retention-gc-print.service`

```ini
[Unit]
Description=Print metin2-migrate artifact-retention-gc triage scripts (no execution)
Documentation=file:///opt/metin2/go-metin2-server/docs/workflow/lab-retention-gc-unit-samples.md
After=local-fs.target
RequiresMountsFor=/var/metin2

[Service]
Type=oneshot
User=metin2
Group=metin2
ExecStartPre=/usr/bin/test -x /usr/local/libexec/metin2-print-retention-gc.sh
ExecStart=/usr/local/libexec/metin2-print-retention-gc.sh
# Explicit non-goals: no shell of the printed scripts, no Environment=DSN.
Nice=10

[Install]
WantedBy=multi-user.target
```

### `metin2-artifact-retention-gc-print.timer`

```ini
[Unit]
Description=Weekly print of metin2 artifact-retention-gc triage scripts

[Timer]
OnCalendar=Sun *-*-* 04:15:00
Persistent=true
Unit=metin2-artifact-retention-gc-print.service

[Install]
WantedBy=timers.target
```

### Optional companion print notes

The tree-owned helper already prints `migration-run-retention.sh` on every run
and forwards `--gamed-log-path` / `--authd-log-path` from
`METIN2_GAMED_LOG_PATH` / `METIN2_AUTHD_LOG_PATH` (defaults
`/var/log/metin2/gamed.log` / `/var/log/metin2/authd.log`). To also print
`backup-restore-drill.sh`, point `METIN2_RUNTIME_CONFIG` at a retained
runtime-config JSON snapshot before the unit/cron fires. Reviewable fragments
live under
[`contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`](../../contrib/lab-retention-gc/env/metin2-runtime-config.env.sample)
and
[`contrib/lab-retention-gc/systemd/metin2-artifact-retention-gc-print.service.d/runtime-config.conf.sample`](../../contrib/lab-retention-gc/systemd/metin2-artifact-retention-gc-print.service.d/runtime-config.conf.sample)
(an `EnvironmentFile=` drop-in). Never embed DSNs. Do **not** schedule live
`curl` of `http://127.0.0.1:6060/local/runtime-config` from the unit; prefer a
file retained during an earlier drained-session window.

Manual one-off companion prints (outside the helper) remain allowed for
triage, still print-only:

```bash
metin2-migrate version >"$OUT/build-info.json"
metin2-migrate backup-restore-drill \
  --runtime-config /var/metin2/ops-prints/retained-runtime-config.json \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path /var/log/metin2/gamed.log \
  --authd-log-path /var/log/metin2/authd.log \
  >"$OUT/backup-restore-drill.sh"

metin2-migrate migration-run-retention \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path /var/log/metin2/gamed.log \
  --authd-log-path /var/log/metin2/authd.log \
  >"$OUT/migration-run-retention.sh"
```
Those companions still must not pipe the resulting scripts into a shell from
the unit. Loopback `curl` to `127.0.0.1:6060` / `:6061` is allowed only for
interactive operator capture of retained JSON; the scheduled helper itself
never live-fetches.

## FreeBSD cron-style sample (print-only)

`/etc/cron.d/metin2-artifact-retention-gc-print.sample` remains an optional
Linux-style companion:

```cron
# Print-only: dumps artifact-retention-gc scripts under /var/metin2/ops-prints.
# Do NOT append "| /bin/sh" to the helper output or invoke the printed *.sh files here.
15 4 * * 0  metin2  /usr/local/libexec/metin2-print-retention-gc.sh >/dev/null
```

## FreeBSD periodic(8) weekly sample (print-only)

Preferred on FreeBSD lab hosts that already schedule `periodic weekly` from
`/etc/crontab`. Tree fragments:

- [`contrib/lab-retention-gc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample`](../../contrib/lab-retention-gc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample)
- [`contrib/lab-retention-gc/periodic/periodic.conf.sample`](../../contrib/lab-retention-gc/periodic/periodic.conf.sample)

Install (manual, review first):

```bash
install -d -m 0755 /usr/local/etc/periodic/weekly
install -m 0755 \
  contrib/lab-retention-gc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample \
  /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample
# rename without .sample only after review:
#   cp /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample \
#      /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print

install -m 0644 \
  contrib/lab-retention-gc/periodic/periodic.conf.sample \
  /usr/local/etc/periodic.conf.sample
# merge reviewed knobs into /etc/periodic.conf or /usr/local/etc/periodic.conf;
# keep weekly_metin2_artifact_retention_gc_print_enable="NO" until deliberately flipped
```

Contract:

1. The weekly script sources `periodic.conf`, gates on
   `weekly_metin2_artifact_retention_gc_print_enable` matching `YES`
   (case-insensitive), and otherwise exits `0` without running the helper.
2. Default remains `"NO"` in `periodic.conf.sample`. Packaging / ports must not
   flip this to `YES`.
3. When enabled, the script runs only
   `/usr/local/libexec/metin2-print-retention-gc.sh` (or
   `METIN2_PRINT_RETENTION_GC_HELPER`). It never pipes printer stdout into a
   shell, never `curl`s ops JSON, never embeds DSN / SQL, and never `rm`s
   retention / `.gc-aside-*` trees.

See also [contrib FreeBSD periodic retention / GC print sample](../plans/2026-08-23-contrib-freebsd-periodic-retention-gc-print-sample.md).

## Operator review after a print

1. Open `$OUT/notes.md` and `$OUT/build-info.json`; confirm stamp / commit match
   the intended lab binary (`metin2-migrate version` / `GET /local/build-info`).
2. Read the printed `.sh` files. Confirm they still only aside-rename aged
   trees and never contain `rm` / DSN / SQL markers.
3. If triage is desired, run **interactively** under an operator shell after
   draining sessions / confirming no concurrent retention writers — never from
   the timer / cron line.
4. Retain the printed scripts beside migration-run / backup evidence when the
   print coincides with a reconnect/restart or apply window.

## What this is not yet

- packaging / FreeBSD port / `pkg` that installs **enabled** timers, cron, or
  `periodic` entries by default (disabled-by-default `.sample` fragments under
  [`contrib/lab-retention-gc/`](../../contrib/lab-retention-gc/) are owned,
  including FreeBSD `periodic(8)` weekly + `periodic.conf.sample`)
- flipping `weekly_metin2_artifact_retention_gc_print_enable` to `YES` by default
- automatic execution of printed aside-rename / backup / apply scripts
- `rm` of `.gc-aside-*` trees
- scheduled live `curl` of ops JSON from the print helper / unit
- multi-host orchestration or remote admin
- SQL import/backfill from quarantined exports
