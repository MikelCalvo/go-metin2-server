# Contrib Migration-Run Retention Target Env — 2026-08-28

## Objective

Extend the already-shipped print-only `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
helper so operators can request intermediate/forward and rollback-direction
`migration-run-retention` scripts via env knobs that mirror the CLI flags proven
by the hermetic intermediate-target SQLite proofs — without auto-running apply,
registering a stock production driver, inventing upsert policy, or enabling
timers by default.

Today the helper always prints `migration-run-retention.sh` with only
`--build-info` plus optional daemon log paths. Staged lab cutovers that need
`--target-version 7` or `--allow-rollback --target-version 8` still require
hand-editing the printed script or invoking `metin2-migrate` outside the
reviewed print helper.

## Why now

- Track E / migration-contract tips still name production-engine selection /
  operator runbook hardening beyond the SQLite harness as the remaining gap
  after hermetic intermediate-target retention (empty→`7`, tip→`8`).
- Hermetic CLI proofs already own intermediate targets; the contrib print helper
  and unit samples still always emit the default `latest` forward script.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (no auto-run, no stock driver, no upsert).

## Contract frozen by this slice

1. Keep the existing always-on `migration-run-retention.sh` print and all other
   helper gates unchanged when the new env knobs are unset.
2. Optional `METIN2_MIGRATION_TARGET_VERSION`:
   - when unset / empty → omit `--target-version` (CLI default remains `latest`);
   - when set to a non-empty trimmed value → pass `--target-version "$VALUE"`;
   - whitespace-only values are treated as empty (omit the flag).
3. Optional `METIN2_MIGRATION_ALLOW_ROLLBACK`:
   - only `YES` (case-insensitive) requests rollback-direction printing;
   - when `YES` **and** `METIN2_MIGRATION_TARGET_VERSION` is a non-empty
     non-`latest` value → also pass `--allow-rollback`;
   - when `YES` but target is missing / empty / `latest` → print without
     `--allow-rollback` and record a fail-closed note that rollback requires a
     non-empty non-`latest` target (still never auto-runs apply);
   - any other non-empty value → treat as not enabled (no `--allow-rollback`)
     and note that the knob must be `YES` to print rollback-direction flags.
4. `notes.md` gains an explicit `migration-run-retention=...` line describing
   the effective target / rollback posture for the printed script.
5. Tree-owned samples document the knobs as commented optional overrides:
   - `contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`
   - `contrib/lab-retention-gc/periodic/periodic.conf.sample`
   - `contrib/lab-retention-gc/README.md`
   - embedded helper copy in `docs/workflow/lab-retention-gc-unit-samples.md`
6. Hermetic helper execution coverage proves:
   - unset env → no `--target-version` / no `--allow-rollback` in stub argv;
   - `METIN2_MIGRATION_TARGET_VERSION=7` → `--target-version 7`, no rollback;
   - `METIN2_MIGRATION_ALLOW_ROLLBACK=YES` + `METIN2_MIGRATION_TARGET_VERSION=8`
     → both flags;
   - `METIN2_MIGRATION_ALLOW_ROLLBACK=YES` without a non-`latest` target → no
     `--allow-rollback`, notes record the skip reason;
   - helper / samples still never embed DSNs, never pipe printed scripts into a
     shell, and never execute apply.
7. Docs mark this contrib target-env follow-up done on the intermediate-target /
   Track E / migration-contract tips; upsert / auto-run / stock production
   driver remain explicitly deferred.

## What this is not yet

- automatic / scheduled execution of the printed apply / rollback script
- flipping any timer / cron / `periodic` enable flag to YES by default
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- FreeBSD port / `pkg` enable defaults
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- claiming DB-backed runtime stores replace FileStores

## Likely files to change

- `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
- `contrib/lab-retention-gc/README.md`
- `contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`
- `contrib/lab-retention-gc/periodic/periodic.conf.sample`
- `docs/workflow/lab-retention-gc-unit-samples.md` (embedded helper copy)
- `internal/migratecli/contrib_lab_retention_gc_test.go`
- `docs/plans/2026-08-28-hermetic-migration-run-retention-intermediate-target-sqlite.md`
- `docs/plans/2026-08-28-hermetic-migration-run-retention-sqlite-apply.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md`
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- static sample assertions mention the new env knobs
- hermetic helper:
  - default omit target/rollback flags
  - forward intermediate target `7`
  - rollback intermediate target `8` with `--allow-rollback`
  - invalid rollback combo notes skip and omits `--allow-rollback`
- printer stdout / helper notes never embed a concrete DSN value

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1
gofmt -l internal/migratecli/contrib_lab_retention_gc_test.go
git diff --check
```

## Exit criteria

- contrib helper forwards optional target / allow-rollback env to the printer
- invalid rollback combo fails closed without printing `--allow-rollback`
- samples / README / unit-samples doc document the knobs
- prior deferred runbook-hardening tips mark this contrib follow-up done
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed apply/rollback from CLI / contrib / cron.
- Do not embed DSN values in helper stdout / notes / samples.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
