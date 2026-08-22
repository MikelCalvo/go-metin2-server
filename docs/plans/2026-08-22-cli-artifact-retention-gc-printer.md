# CLI Artifact-Retention GC Printer — 2026-08-22

## Objective

Add a read-only `metin2-migrate artifact-retention-gc` command so operators can turn a lab retention root (`/var/metin2/backups` or `/var/metin2/migration-runs`) plus a keep-days policy into a path-aware shell script that triages and aside-renames aged `YYYYMMDDTHHMMSSZ-<commit12>/` trees — without deleting trees, opening a database, writing files itself, embedding DSNs, or inventing a cron/daemon GC job.

`docs/workflow/lab-deployment-topology.md` already freezes retention naming and explicitly deferred automatic artifact GC. Retention printers already create those trees. Operators still lack a fail-closed offline printer for age-based aside triage. This slice closes that deferred printer gap after backup/migration retention correlation.

## Contract frozen by this slice

```bash
metin2-migrate artifact-retention-gc \
  --retention-base </var/metin2/backups|/var/metin2/migration-runs|other absolute path> \
  [--keep-days 14] \
  [--now <RFC3339|/YYYYMMDDTHHMMSSZ>]
```

Rules:

1. `--retention-base` is required and must be an absolute cleaned path (same normalization style as `--migration-runs-base` / `--backup-base`). Relative / empty / whitespace fails closed.
2. `--keep-days` defaults to `14`. It must parse as an integer `>= 1`. Zero / negative / non-integer fails closed.
3. `--now` is optional. When omitted, the printer uses the inspecting host's UTC wall clock. When present it must decode as either:
   - RFC3339 / RFC3339Nano (for example `2026-08-22T12:00:00Z`), or
   - the lab compact UTC stamp `YYYYMMDDTHHMMSSZ`
   Invalid / empty `--now` fails closed. Tests pin `--now` for determinism.
4. On success, stdout is a plain-text shell script that:
   - sets `RETENTION_BASE`, `KEEP_DAYS`, `NOW_UTC` (`YYYYMMDDTHHMMSSZ` from the resolved `--now` / wall clock), and `ASIDE_SUFFIX` (`gc-aside-<NOW_UTC>`);
   - documents that matching children are only directory names shaped `YYYYMMDDTHHMMSSZ-<suffix>` where the 16-character UTC prefix parses as a real UTC timestamp;
   - prints a triage loop that skips non-matching names, non-directories, and trees whose UTC prefix is younger than `KEEP_DAYS` relative to `NOW_UTC`;
   - for each aged candidate, prints an aside-rename to `"$RETENTION_BASE/<name>.$ASIDE_SUFFIX"` and refuses to overwrite if the aside destination already exists;
   - never prints `rm`, `rmdir`, `unlink`, `find -delete`, or truncate;
   - never executes the rename itself, never opens a database, never prints executable SQL / concrete DSNs.
5. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
6. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists `artifact-retention-gc`.
7. Top-level `metin2-migrate` usage lists the new command beside the other read-only printers.

## What this is not yet

- automatic / scheduled artifact GC or lifecycle daemons
- `rm` / unlink / truncate helpers
- recursive scanning below the immediate retention-base children
- deleting aside-renamed trees
- DB dumps / engine-specific backup GC
- ground-item restart durability or SQL import/backfill
- remote admin auth
- systemd / cron unit shipping that invokes this printer

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for `/var/metin2/backups` with `--keep-days 14` and pinned `--now` includes `RETENTION_BASE`, `KEEP_DAYS`, `NOW_UTC`, `ASIDE_SUFFIX`, the matching-name triage comment, aside-rename destination shape, and destination-collision guard
- custom `--retention-base /var/metin2/migration-runs` is honored
- `--keep-days 0` / negative / non-integer → exit `1`, no stdout
- relative `--retention-base` → exit `1`
- invalid `--now` → exit `1`
- missing `--retention-base` / unexpected args / unknown flag → exit `2`
- usage text lists `artifact-retention-gc`
- stdout omits `rm` / SQL / concrete DSN markers and does not claim to perform GC itself

Validation for this slice:

- `go test ./internal/migratecli -run 'ArtifactRetentionGC|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled artifact GC deferred; any future cron/unit must require explicit operator confirmation beyond this printer.
2. Keep `rm` of aside-renamed trees deferred.
3. ~~Keep ground-item restart durability deferred until operators decide quarantined `0010` exports drive recovery.~~ Done for FileStore rematerialize + backup/restore: see [ground-item process-restart durability](2026-08-22-ground-item-process-restart-durability.md) and [ground-item file-store backup/restore](2026-08-22-ground-item-file-store-backup-restore.md). SQL import/backfill from quarantined `0010` exports remains deferred.
4. Keep SQL import/backfill deferred until a driver-backed harness exists.
5. Optional later: systemd/unit samples that only print (never auto-run) this triage script.
6. ~~Prove the printed `/bin/sh` triage script actually aside-renames aged trees under FreeBSD/GNU date helpers.~~ Done: see [CLI artifact-retention GC script execution proof](2026-08-22-cli-artifact-retention-gc-script-execution-proof.md).
