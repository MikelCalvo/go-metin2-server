# Gamed Migration + Quarantine/Export Ops Registration Helper — 2026-08-24

## Objective

Extract the still-inline `cmd/gamed` loopback migration + migration-shaped
export/quarantine wiring into one `internal/minimal` helper beside
`RegisterGamedFileStorePersistenceOps`, so production bootstrap and future
hermetic migration/export proofs cannot drift across the five read-only
migration routes and nine tip-`0015` export/quarantine pairs.

## Why now

- Track E just shared the eight-store file-store drill registration helper, and
  that plan's explicit "not yet" was widening the helper — not forever leaving
  migration/quarantine duplicated at the call site.
- `cmd/gamed/main.go` still registers a cohesive persistence block inline:
  migration status / catalog / plan / ledger-snapshot / plan-from-ledger-snapshot
  plus export + quarantine for roster, item state, point state, login-ticket
  handoff, quest state, safebox state, item templates, static-actor content, and
  bootstrap ground items.
- Catalog tip is already `0015_character_safebox_money`; seams and quarantine
  kinds exist. The remaining ops gap is registration ownership for that surface.
- Safe, lane-scoped, one-commit, and testable without touching deferred dangerous
  work (auto-run, `rm`, remote admin, SQL import).

## Contract frozen by this slice

1. `internal/minimal` owns
   `RegisterGamedMigrationQuarantineExportOps(mux *http.ServeMux, runtime *gameRuntime) *http.ServeMux`.
2. The helper registers exactly:
   - `GET /local/db/migrations/status`
   - `GET /local/db/migrations/catalog`
   - `GET /local/db/migrations/plan`
   - `GET /local/db/migrations/ledger-snapshot`
   - `POST /local/db/migrations/plan-from-ledger-snapshot`
   - export + quarantine for:
     - account-character-roster
     - character-item-state
     - character-point-state
     - auth-login-ticket-handoff
     - character-quest-state
     - character-safebox-state
     - item-template-state
     - static-actor-content-state
     - bootstrap-ground-item-state
3. The helper does **not** register:
   - eight-store validate / crash-temp / backup / restore / runtime-config /
     persistence-status (already owned by `RegisterGamedFileStorePersistenceOps`)
   - quest transition / overview / character / quest / flag mutation reads
   - login-ticket issued-before preview/cleanup
   - spawn-group / static-actor respawn / content-bundle / world introspection
4. `cmd/gamed` calls the helper after the file-store helper (and after any
   still-inline login-ticket issued-before / quest mutation wiring), replacing
   the duplicated `RegisterLocalMigration*` / export / quarantine list.
5. Focused coverage proves the helper serves migration catalog, one export route,
   and one quarantine route on a drained runtime; it stays scoped (omits a
   file-store backup route and a quest-state mutation route).
6. Docs mark the file-store helper's "extract migration/quarantine" follow-up
   done and name this helper as the single registration owner for that surface.

## What this is not yet

- extracting quest / spawn / content-bundle / world ops into more helpers
- hermetic HTTP drill that drives export → offline `quarantine-export`
- automatic / scheduled execution of printed triage scripts
- `rm` of aside-renamed trees
- FreeBSD port / `pkg` enable defaults
- remote log shipping / metrics exporters
- SQL import/backfill or a driver-backed harness
- inventing a daemon `/local/...` log-download endpoint
- remote admin authentication
- changing export schemas, quarantine validators, or migration tip `0015`

## Likely files to change

- `internal/minimal/gamed_migration_ops.go` (new)
- `internal/minimal/gamed_migration_ops_test.go` (new)
- `internal/minimal/gamed_ops.go` (comment only: migration owned by sibling)
- `cmd/gamed/main.go`
- `docs/plans/2026-08-24-gamed-file-store-persistence-ops-registration-helper.md`
- `docs/development.md` (brief pointer only)
- this plan

## TDD and validation

Focused coverage in `internal/minimal`:

- helper serves `/local/db/migrations/catalog` with tip metadata through `0015`
- helper serves roster export for a seeded account
- helper serves roster quarantine for a valid tip-shaped payload
- helper omits `/local/account-store/backup` and quest-state transition
- non-loopback catalog caller gets `403`

Validation for this slice:

- `go test ./internal/minimal -run 'GamedMigrationQuarantineExportOps|GamedFileStorePersistenceOps' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- one shared helper owns the migration + tip-`0015` quarantine/export list
- `cmd/gamed` calls it instead of the duplicated inline registrations
- focused tests green
- file-store helper plan marks the follow-up done

## Anti-goals / ordering constraints

- Do not widen this helper to quest / spawn / content-bundle surfaces.
- Do not change endpoint paths, request bodies, export schemas, or tip version.
- Do not auto-run printed migration / quarantine scripts from CLI.
- Do not push `origin/main`; push only `origin/lane/persistence`.
