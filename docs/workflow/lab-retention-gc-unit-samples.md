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
    backup-restore-drill.sh          # optional
    migration-run-retention.sh       # optional
    notes.md
```

Create the parent once on the lab host:

```bash
install -d -m 0750 /var/metin2/ops-prints
```

Shared helper used by the samples below (install as
`/usr/local/libexec/metin2-print-retention-gc.sh`, mode `0750`, owner root):

```bash
#!/bin/sh
# Print-only retention/GC dump. Never executes the printed triage scripts.
set -eu
BIN="${METIN2_MIGRATE_BIN:-/usr/local/bin/metin2-migrate}"
PRINTS_ROOT="${METIN2_OPS_PRINTS_ROOT:-/var/metin2/ops-prints}"
KEEP_DAYS="${METIN2_RETENTION_KEEP_DAYS:-14}"

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

printf 'printed %s\ncommit=%s\nkeep_days=%s\n' "$OUT" "${COMMIT:-unknown}" "$KEEP_DAYS" >"$OUT/notes.md"
chmod 0640 "$OUT"/*.sh "$OUT/build-info.json" "$OUT/notes.md"
printf '%s\n' "$OUT"
```

The helper's only `rm` is the temporary `mktemp` build-info copy via `trap` —
never retention trees or printed scripts.

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

### Optional companion print services

Operators who want the backup / migration-run **create** printers dumped on the
same cadence can extend the helper (or add parallel oneshots) that only write:

```bash
metin2-migrate version >"$OUT/build-info.json"
curl -sS http://127.0.0.1:6060/local/runtime-config \
  | metin2-migrate backup-restore-drill \
      --runtime-config - \
      --build-info "$OUT/build-info.json" \
  >"$OUT/backup-restore-drill.sh"

metin2-migrate version \
  | metin2-migrate migration-run-retention --build-info - \
  >"$OUT/migration-run-retention.sh"
```

Those companions still must not pipe the resulting scripts into a shell from
the unit. Loopback `curl` to `127.0.0.1:6060` / `:6061` is allowed only when
`gamed` / `authd` are up; otherwise prefer retained JSON snapshots on disk.

## FreeBSD cron-style sample (print-only)

`/etc/cron.d/metin2-artifact-retention-gc-print.sample`:

```cron
# Print-only: dumps artifact-retention-gc scripts under /var/metin2/ops-prints.
# Do NOT append "| /bin/sh" to the helper output or invoke the printed *.sh files here.
15 4 * * 0  metin2  /usr/local/libexec/metin2-print-retention-gc.sh >/dev/null
```

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

- packaging that installs enabled timers by default
- automatic execution of printed aside-rename / backup / apply scripts
- `rm` of `.gc-aside-*` trees
- multi-host orchestration or remote admin
- SQL import/backfill from quarantined exports
