# Gamed File-Store Persistence Ops Registration Helper — 2026-08-24

## Objective

Extract the duplicated loopback file-store persistence ops wiring shared by
`cmd/gamed` and the hermetic
[backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md)
into one registration helper so the eight-store validate / crash-temp cleanup /
backup / backup-validate / restore surface plus `runtime-config` /
`persistence-status` cannot drift between production daemon bootstrap and the
drained drill proof.

## Why now

- Track E just proved the printed `backup-restore-drill` script against a
  drained loopback mux, but the proof rebuilt ~40 `RegisterLocal*` calls by
  hand beside the same block in `cmd/gamed/main.go`.
- The hermetic plan already named this as the remaining optional follow-up:
  extract a shared gamed ops registration helper so `cmd/gamed` and the drill
  stop duplicating endpoint wiring.
- Without a single owner, the next store fold-in (or path rename) can leave the
  drill green while `gamed` silently omits a restore route, or the reverse.

## Contract frozen by this slice

1. `internal/minimal` owns
   `RegisterGamedFileStorePersistenceOps(mux *http.ServeMux, runtime *gameRuntime) *http.ServeMux`.
2. The helper registers exactly the drained backup/restore drill surface:
   - per-store validate + crash-temps/cleanup for accounts, login tickets, item
     templates, static actors, interactions, quest state, ground items, safebox
   - per-store backup + backup/validate + restore for those eight stores
   - `GET /local/runtime-config`
   - `GET /local/persistence/status`
3. The helper does **not** register:
   - world introspection / notice / relocate / transfer
   - login-ticket issued-before preview/cleanup
   - quest-state transition / overview / character / quest / flag mutation reads
   - migration catalog/status/plan/ledger endpoints
   - quarantine / export endpoints
   - spawn-group / static-actor respawn / content-bundle surfaces
4. `cmd/gamed` calls the helper immediately after
   `ops.NewPprofMuxWithLocalRuntimeIntrospection(...)`, then continues to
   register the non-shared surfaces above.
5. The hermetic drill proof builds `ops.NewPprofMux("gamed")` and then calls the
   same helper (no duplicated `RegisterLocal*` list).
6. Focused coverage proves the helper serves `/local/persistence/status` and at
   least one store backup route; the existing
   `TestBackupRestoreDrillHTTPExecutesAgainstDrainedGamedOps` remains the
   end-to-end curl-script gate.
7. Docs mark the hermetic-plan follow-up done and name the helper as the single
   registration owner for the eight-store drill surface.

## What this is not yet

- extracting the remaining `cmd/gamed` ops surface (quest mutation / spawn /
  content-bundle / world introspection)
- automatic / scheduled execution of printed triage scripts
- ~~`rm` of aside-renamed trees~~ Done for confirmation-gated print-only `artifact-gc-aside-purge` — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution remains deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).
- FreeBSD port / `pkg` enable defaults
- remote log shipping / metrics exporters
- SQL import/backfill or a driver-backed harness
- inventing a daemon `/local/...` log-download endpoint
- remote admin authentication

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution remains deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).
3. Keep FreeBSD port / `pkg` enable defaults deferred.
4. Keep remote log shipping / metrics exporters deferred.
5. Keep SQL import/backfill deferred until a driver-backed harness exists.
6. ~~Optional later: extract the inline migration + tip-`0015` quarantine/export
   registration into a shared helper beside this file-store owner.~~ Done: see
   [gamed migration + quarantine/export ops registration helper](2026-08-24-gamed-migration-quarantine-export-ops-registration-helper.md).
7. Keep extracting quest / spawn / content-bundle / world ops deferred.

## Likely files to change

- `internal/minimal/gamed_ops.go` (new)
- `internal/minimal/gamed_ops_test.go` (new)
- `internal/minimal/backup_restore_drill_http_test.go`
- `cmd/gamed/main.go`
- `docs/plans/2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md`
- `docs/workflow/file-store-backup-restore-drill.md`
- `docs/development.md` (brief pointer only)

## TDD and validation

Focused coverage in `internal/minimal`:

- helper registers persistence-status + runtime-config on a drained runtime
- helper registers an eight-store backup route (account-store backup accepts
  loopback POST shape / rejects non-loopback)
- existing `BackupRestoreDrillHTTP` still passes after call-site rewrite

Validation for this slice:

- `go test ./internal/minimal -run 'GamedFileStorePersistenceOps|BackupRestoreDrillHTTP' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- one shared helper owns the eight-store drill registration list
- `cmd/gamed` and the hermetic drill both call it
- focused + HTTP drill tests green
- hermetic plan follow-up marked done

## Anti-goals / ordering constraints

- Do not widen the helper to the full gamed ops mux in this slice.
- Do not change endpoint paths, request bodies, or restore live-session gates.
- Do not auto-run printed backup/restore scripts from CLI.
- Do not push `origin/main`; push only `origin/lane/persistence`.
