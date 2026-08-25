# Print-Only Retention / GC Unit Samples — 2026-08-23

## Objective

Freeze the first **print-only** systemd (and FreeBSD cron-style) unit samples for
the already-shipped lab retention / GC printers so operators can schedule a
reviewable script dump without inventing automatic aside-rename, `rm`, scheduled
deletion, remote admin, or in-tree unit shipping that auto-executes triage.

The printers (`backup-restore-drill`, `migration-run-retention`,
`artifact-retention-gc`) already emit path-aware shell scripts. Multiple Track E
follow-ups still listed "systemd/unit samples that only print (never auto-run)"
as the next ops maturity step after the GC execution proof. This slice closes
that documentation gap.

## Why now

- `artifact-retention-gc` printer + hermetic `/bin/sh` execution proof already
  own age-based aside-rename triage when an operator runs the printed script.
- Lab topology still says "Kubernetes / systemd unit shipping in-tree" is not
  yet owned, which over-blocks the narrower print-only sample contract.
- Safebox money (`0015`) rematerialize / export already landed; lab path comments
  that still say "safebox cells" alone under-describe the eighth store for
  reconnect/restart operators.
- SQL import, automatic GC deletion, and multi-host orchestration remain
  correctly deferred — this slice must not open those doors.

## Contract frozen by this slice

1. New operator doc `docs/workflow/lab-retention-gc-unit-samples.md` owns:
   - print-only systemd `.service` samples that invoke
     `metin2-migrate artifact-retention-gc` (and optionally the backup /
     migration-run printers) and write stdout to a dated review path under
     `/var/metin2/ops-prints/`;
   - matching `.timer` samples that only start those print services;
   - a FreeBSD `/etc/cron.d`-style sample with the same print-only semantics;
   - hard rules: never `ExecStart` / cron-pipe the printed triage script into
     `/bin/sh`, never `rm` / unlink aside trees, never embed DSNs, never bind
     ops listeners off loopback.
2. `docs/workflow/lab-deployment-topology.md` links the new doc and narrows
   "systemd unit shipping" so automatic execution stays deferred while
   print-only samples are owned.
3. Lab path / drill wording for safebox names durable warehouse gold beside
   cells (FileStore already owns both; `0015` tips export).
4. Recent Track E follow-ups that still deferred print-only unit samples mark
   that option done and point here; automatic / scheduled deletion remains
   deferred.

## What this is not yet

- installing or enabling units from CI / packaging
- `OnCalendar` / cron jobs that auto-run the printed aside-rename script
- `rm` / unlink of `.gc-aside-*` trees
- SQL import/backfill or driver-backed harness
- mall / TMP4 CG `SAFEBOX_MONEY`
- remote admin authentication
- README churn beyond these operator docs

## TDD and validation

Docs / sample-contract slice; no new Go tests required.

Validation:

- `git diff --check`
- spot-check that every sample invokes a printer and writes/prints a script,
  never pipes the script into a shell
- spot-check lab topology / drill safebox wording mentions warehouse money
- confirm related plan follow-ups no longer list print-only unit samples as
  open without pointing here

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. ~~Optional later: packaging / `pkg` / port fragments that install these samples
   as `.sample` files only (still disabled by default).~~ Done: see
   [contrib print-only retention / GC unit samples](2026-08-23-contrib-print-only-retention-gc-unit-samples.md)
   and [`contrib/lab-retention-gc/`](../../contrib/lab-retention-gc/). FreeBSD port /
   `pkg` enable defaults remain deferred.
5. ~~Optional later: fold companion `migration-run-retention` /
   `backup-restore-drill` prints into the contrib helper.~~ Done: see
   [contrib companion print retention printers](2026-08-23-contrib-companion-print-retention-printers.md).
   Live scheduled `curl` of runtime-config remains deferred.
