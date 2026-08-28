# Hermetic Migration-Run Retention Intermediate-Target SQLite Proof — 2026-08-28

## Objective

Extend the already-shipped hermetic `migration-run-retention` SQLite proofs
(empty → catalog tip, tip → version `0`) with intermediate-target coverage so
operators can trust printed forward apply to a non-`latest` tip and printed
rollback to a non-zero intermediate ledger without inventing automatic CLI
execution, registering a stock production driver, upsert policy, or a daemon
mutation route.

Today the printer already accepts an arbitrary `--target-version` string and
the hermetic harness proves only the two extremes (`latest` and `0`). Staged
lab cutovers and partial-catalog drills still lack an executable printed-script
proof for mid-catalog targets.

## Why now

- Track E / migration-contract tips still name production-engine selection /
  operator runbook hardening beyond the SQLite harness as the remaining gap
  after hermetic tip/zero retention apply.
- Printer stdout-shape tests cover `latest` and rollback-to-`0`, but do not
  pin an intermediate forward target's artifact names / omitted
  `--allow-rollback` contract.
- Hermetic tagged binary + `/bin/sh` coverage does not yet prove:
  - empty ledger → intermediate version `N` via printed forward retention;
  - catalog tip → intermediate version `M` via printed rollback retention.
- The PvE durable-state / migration-window vertical needs confidence that
  staged apply/rollback windows (for example stop at `0007` before importing
  tickets, or roll tip back to `0008` without wiping the whole ledger) work
  through the same PATH / `DRIVER` / `DSN` runbook script operators already use.

## Contract frozen by this slice

1. Keep the existing printer command surface, tip/zero hermetic proofs, and
   conditional leftover-lock triage footer unchanged.
2. Untagged printer stdout-shape coverage adds one forward intermediate case:
   - `--target-version 7` without `--allow-rollback`;
   - prints `TARGET_VERSION='7'`;
   - keeps forward artifact names (`migration-plan-artifact.json`,
     `apply-preflight.json`, `migration-apply-audit.json`,
     `post-apply-status.json`, default `migration-apply.lock`);
   - omits `--allow-rollback` and rollback artifact names.
3. Build-tagged coverage under `internal/migratecli`
   (`//go:build sqlite_harness`) adds:
   - forward hermetic proof: print `migration-run-retention --target-version 7`
     against an absolute temp `--migration-runs-base`, execute under `/bin/sh`
     with tagged `PATH` + curl stub + `DRIVER=sqlite` + empty-temp `DSN`,
     assert retained forward artifacts and live ledger tip version `7` /
     `auth_login_ticket_handoff`;
   - rollback hermetic proof: apply catalog to tip first, print
     `migration-run-retention --allow-rollback --target-version 8`, execute,
     assert retained rollback artifacts and live ledger tip version `8` /
     `static_actor_content_state`.
4. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency.
5. Docs mark the intermediate-target hermetic follow-up done on the
   migration-run-retention / Track E / migration-contract tips; upsert /
   auto-run / stock production driver remain explicitly deferred.

## What this is not yet

- automatic / scheduled execution of the printed apply / rollback script from
  CLI / contrib / cron / periodic
- flipping any timer / cron / `periodic` enable flag to YES by default
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- FreeBSD port / `pkg` enable defaults
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- inventing distributed advisory locks beyond the local lock file
- claiming DB-backed runtime stores replace FileStores

## Likely files to change

- `internal/migratecli/migration_run_retention_test.go` (untagged intermediate
  forward stdout shape)
- `internal/migratecli/migration_run_retention_sqlite_harness_test.go`
  (hermetic intermediate forward + intermediate rollback)
- `docs/plans/2026-08-28-hermetic-migration-run-retention-sqlite-apply.md`
- `docs/plans/2026-08-21-cli-migration-run-retention.md`
- `docs/plans/2026-08-21-cli-migration-rollback-run-retention.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- this plan

## TDD and validation

Focused coverage:

- untagged printer: `--target-version 7` keeps forward artifacts and omits
  `--allow-rollback`
- hermetic tagged binary + printed-script `/bin/sh` forward apply reaches
  ledger version `7`
- hermetic tagged binary + printed-script `/bin/sh` rollback from tip reaches
  ledger version `8`
- printer stdout never embeds a concrete DSN value
- tip/`0` hermetic proofs remain green

Validation for this slice:

```bash
go test ./internal/migratecli -run 'MigrationRunRetention' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'MigrationRunRetentionSQLite' -count=1
gofmt -l internal/migratecli/migration_run_retention_test.go internal/migratecli/migration_run_retention_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- intermediate forward printer shape is pinned
- hermetic printed-script SQLite forward-to-`7` and rollback-to-`8` proofs are
  green under `-tags=sqlite_harness`
- prior deferred runbook-hardening follow-ups mark this intermediate proof done
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed apply/rollback from CLI / contrib / cron.
- Do not embed DSN values in printer stdout.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
