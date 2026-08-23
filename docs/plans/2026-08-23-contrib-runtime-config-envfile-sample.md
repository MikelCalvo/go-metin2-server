# Contrib Runtime-Config EnvironmentFile Sample — 2026-08-23

## Objective

Ship a disabled-by-default systemd drop-in / env-file `.sample` under
`contrib/lab-retention-gc/` that only documents how operators may point
`METIN2_RUNTIME_CONFIG` at a retained runtime-config JSON snapshot for the
already-owned print helper — without live `curl`, DSN/SQL embedding, enabling
timers by default, or auto-running printed triage scripts.

## Why now

- The print helper already gates `backup-restore-drill.sh` on
  `METIN2_RUNTIME_CONFIG`.
- Workflow docs describe a reviewed drop-in env file, but the contrib tree has
  no reviewable fragment for that path.
- Hermetic helper execution is proven; operators still invent divergent env
  drop-ins for the drill printer.

## Contract frozen by this slice

1. New tree fragment(s) under `contrib/lab-retention-gc/`, for example:
   - `systemd/metin2-artifact-retention-gc-print.service.d/runtime-config.conf.sample`
     and/or
   - `env/metin2-runtime-config.env.sample`
2. Sample content may only set / document `METIN2_RUNTIME_CONFIG` to an absolute
   retained JSON path under `/var/metin2/ops-prints/` (or an operator-chosen
   absolute sibling outside live data / retention trees).
3. Hard rules unchanged: no `curl`, no `| /bin/sh`, no DSN/SQL/password markers,
   no `rm` / aside-rename, no `Environment=` DSN lines in the base service.
4. Focused `internal/migratecli` coverage fail-closes if the sample is missing
   or regresses those hard rules.
5. Workflow / contrib README point at the sample; automatic enablement remains
   deferred.

## What this is not yet

- live scheduled `curl` of `/local/runtime-config`
- packaging that enables the drop-in / timer by default
- automatic execution of printed backup / apply / GC scripts
- `rm` of `.gc-aside-*` trees
- SQL import/backfill or remote admin

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. Keep `rm` of aside-renamed trees deferred.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Keep FreeBSD port / `pkg` enable defaults deferred.
5. Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`.
