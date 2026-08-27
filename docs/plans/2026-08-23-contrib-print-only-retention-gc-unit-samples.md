# Contrib Print-Only Retention / GC Unit Samples — 2026-08-23

## Objective

Ship the already-frozen print-only retention / GC unit samples as **disabled-by-default**
tree fragments under `contrib/lab-retention-gc/` so FreeBSD / systemd lab hosts can
`install(1)` / `cp` reviewable `.sample` files without inventing packaging that enables
timers, pipes printed scripts into a shell, or auto-runs aside-rename / `rm`.

The operator contract already lives in
[`docs/workflow/lab-retention-gc-unit-samples.md`](../workflow/lab-retention-gc-unit-samples.md).
This slice only moves those samples into the tree as installable fragments and proves
the hard rules with a focused Go test.

## Why now

- Track E print-only unit samples landed as docs-only; the explicit follow-up was
  "packaging / `pkg` / port fragments that install these samples as `.sample` files
  only (still disabled by default)".
- Lab topology still says packaging that installs **enabled** units by default is
  deferred — that remains true. This slice is narrower: tree-owned `.sample`
  fragments operators copy by hand.
- Automatic GC execution, `rm` of aside trees, SQL import, and remote admin stay
  deferred.

## Contract frozen by this slice

1. New tree path `contrib/lab-retention-gc/` owns:
   - `metin2-print-retention-gc.sh` (print-only helper; mode guidance `0750`)
   - `systemd/metin2-artifact-retention-gc-print.service.sample`
   - `systemd/metin2-artifact-retention-gc-print.timer.sample`
   - `cron.d/metin2-artifact-retention-gc-print.sample`
   - `README.md` install / non-goals note pointing at the workflow doc
2. Samples invoke only printer / helper paths; never `| /bin/sh`, never `ExecStart`
   of printed triage scripts, never embed DSN / SQL, never `rm` retention trees.
3. Focused `internal/migratecli` (or sibling) test reads the contrib fragments from
   the repo root and fail-closes if hard-rule markers regress.
4. Workflow / development / Track E follow-ups mark the packaging-fragments option
   done while automatic enablement / execution remains deferred.

## What this is not yet

- FreeBSD port / `pkg` that installs or enables units by default
- `systemctl enable --now` / cron activation from CI or packaging
- automatic execution of printed aside-rename scripts
- `rm` / unlink of `.gc-aside-*` trees
- SQL import/backfill or driver-backed harness
- remote admin authentication
- README churn beyond operator docs already required

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution remains deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Optional later: FreeBSD port / `pkg` that installs these as `.sample` only
   (still disabled by default; no `ENABLE` defaults).
5. ~~Optional later: FreeBSD `periodic(8)` weekly print-only fragment gated on
   `weekly_metin2_artifact_retention_gc_print_enable="NO"`.~~ Done: see
   [contrib FreeBSD periodic retention / GC print sample](2026-08-23-contrib-freebsd-periodic-retention-gc-print-sample.md).
6. ~~Optional later: fold companion `migration-run-retention` /
   `backup-restore-drill` prints into the helper.~~ Done: see
   [contrib companion print retention printers](2026-08-23-contrib-companion-print-retention-printers.md).
   Live scheduled `curl` of runtime-config remains deferred.
