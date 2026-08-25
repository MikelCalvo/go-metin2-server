# CLI Artifact GC-Aside Purge Printer — 2026-08-25

## Objective

Add a confirmation-gated `metin2-migrate artifact-gc-aside-purge` command so
operators can turn a lab retention root plus a minimum aside-age policy into a
path-aware shell script that deletes only aged `.gc-aside-*` triage trees —
without auto-running deletion, touching live `YYYYMMDDTHHMMSSZ-<commit12>/`
trees, opening a database, embedding DSNs, folding purge into scheduled print
helpers, or inventing FreeBSD port / remote-admin surfaces.

`artifact-retention-gc` already aside-renames aged live trees to
`<name>.gc-aside-<NOW_UTC>` and repeatedly deferred `rm` of those aside trees.
Operators still lack a fail-closed offline printer for the second, destructive
half of lab retention triage. This slice closes that deferred printer gap after
the export-quarantine / retention-helper chain.

## Why now

- Track E / lab topology / retention-GC unit samples / artifact-retention-gc
  plans still list "`rm` of `.gc-aside-*` trees" as deferred.
- Aside-rename + hermetic execution proofs are green; leaving aside trees forever
  forces host-local destructive wrappers that bypass the reviewed printer
  contract.
- The PvE reconnect / migration-window vertical needs a reproducible, reviewed
  purge path for backups / migration-runs / exports retention roots without
  scheduled deletion daemons.

## Contract frozen by this slice

```bash
metin2-migrate artifact-gc-aside-purge \
  --retention-base </var/metin2/backups|/var/metin2/migration-runs|/var/metin2/exports|other absolute path> \
  --i-confirm-lab-gc-aside-purge \
  [--min-aside-age-days 7] \
  [--now <RFC3339|/YYYYMMDDTHHMMSSZ>]
```

Rules:

1. `--retention-base` is required and must be an absolute cleaned path (same
   normalization as `artifact-retention-gc`). Relative / empty / whitespace
   fails closed.
2. `--i-confirm-lab-gc-aside-purge` is required to emit any stdout script.
   Missing confirmation → exit `1`, stderr reason, **no** stdout. The CLI still
   never executes the printed purge itself.
3. `--min-aside-age-days` defaults to `7`. It must parse as an integer `>= 1`.
   Zero / negative / non-integer fails closed.
4. `--now` is optional. When omitted, the printer uses the inspecting host's UTC
   wall clock. When present it must decode as either RFC3339 / RFC3339Nano or
   the lab compact UTC stamp `YYYYMMDDTHHMMSSZ`. Invalid / empty `--now` fails
   closed. Tests pin `--now` for determinism.
5. On success, stdout is a plain-text shell script that:
   - sets `RETENTION_BASE`, `MIN_ASIDE_AGE_DAYS`, `NOW_UTC`
     (`YYYYMMDDTHHMMSSZ`), and `CUTOFF_EPOCH`;
   - documents that candidates are only immediate child directories whose names
     end with `.gc-aside-YYYYMMDDTHHMMSSZ`;
   - skips non-directories, non-matching names, and aside trees younger than
     `MIN_ASIDE_AGE_DAYS` relative to `NOW_UTC` (age measured from the
     `.gc-aside-<stamp>` suffix, not the original live-tree prefix);
   - for each aged aside candidate, prints `rm -rf -- "$path"` plus a
     `purged <basename>` echo;
   - never mutates live `YYYYMMDDTHHMMSSZ-<commit12>/` trees that lack the
     `.gc-aside-` marker;
   - never executes the purge itself, never opens a database, never prints
     executable SQL / concrete DSNs.
6. On contract failure, exit `1` with a short stderr reason and **no** stdout
   script.
7. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists
   `artifact-gc-aside-purge`.
8. Top-level `metin2-migrate` usage lists the new command beside
   `artifact-retention-gc`.
9. Hermetic `/bin/sh` coverage materializes a temp retention root, prints with
   pinned `--now` / `--min-aside-age-days` / confirmation, and proves:
   - aged `.gc-aside-*` directories are removed;
   - young aside trees, live retention trees, and non-matching residue remain;
   - printer stdout still omits SQL / DSN markers.
10. Docs mark the recurring "`rm` of `.gc-aside-*`" follow-up done for this
    print-only confirmation-gated surface; automatic / scheduled purge execution
    and folding into `contrib/lab-retention-gc` remain deferred.

## What this is not yet

- automatic / scheduled execution of the printed purge script
- folding `artifact-gc-aside-purge` into `contrib/lab-retention-gc` print-only
  samples / timers / cron / periodic (those dumps must stay free of `rm`)
- FreeBSD port / `pkg` enable defaults
- SQL import/backfill from quarantined exports
- recursive scanning below immediate retention-base children
- remote admin authentication
- metrics / tracing / remote log shipping

## Likely files to change

- `internal/migratecli/artifact_gc_aside_purge.go` (new)
- `internal/migratecli/artifact_gc_aside_purge_test.go` (new)
- `internal/migratecli/migratecli.go` (dispatch + usage)
- `docs/development.md`
- `docs/workflow/lab-deployment-topology.md`
- `docs/workflow/lab-retention-gc-unit-samples.md`
- `docs/plans/2026-08-22-cli-artifact-retention-gc-printer.md`
- `docs/plans/2026-08-22-cli-artifact-retention-gc-script-execution-proof.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for `/var/metin2/backups` with confirmation, `--min-aside-age-days 7`,
  and pinned `--now` includes `RETENTION_BASE`, `MIN_ASIDE_AGE_DAYS`, `NOW_UTC`,
  `.gc-aside-` matching comments, and `rm -rf --`
- missing confirmation → exit `1`, no stdout
- relative `--retention-base` / invalid age / invalid `--now` → exit `1`
- missing `--retention-base` / unexpected args / unknown flag → exit `2`
- usage text lists `artifact-gc-aside-purge`
- hermetic execution purges one aged aside tree and preserves young aside + live
  retention + non-matching residue
- stdout omits SQL / concrete DSN markers

Validation for this slice:

- `go test ./internal/migratecli -run 'ArtifactGCAsidePurge|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- confirmation-gated purge printer is green with hermetic `/bin/sh` proof
- prior deferred "`rm` of `.gc-aside-*`" follow-ups mark this print-only surface done
- docs name the command beside `artifact-retention-gc` and keep scheduled helpers
  free of `rm`

## Anti-goals / ordering constraints

- Do not auto-run the printed purge from CLI / unit / cron / periodic.
- Do not fold purge into `contrib/lab-retention-gc` this slice.
- Do not add SQL import/backfill.
- Do not push `origin/main`; push only `origin/lane/persistence`.
