# Static-Actor Import Require Chase-Delay Schema — 2026-08-28

## Objective

Fail closed before SQL execution when `ImportStaticActorContentState` targets a
ledger that owns tip-`0013` static-actor content-state but has **not** applied
additive `0016_static_actor_combat_profile_chase_delay`, so operators get
`ErrStaticActorContentStateImportSchemaRequired` instead of a raw
`no such column: chase_delay_ms` driver error.

## Why now

- Tip-`0013` export/quarantine/import already round-trip `chase_delay_ms` on
  combat-profile rows because the live FileStore / content-bundle surface owns
  authored chase delay.
- Migration `0016` is what actually adds the SQL column, but
  `requireStaticActorContentStateSchema` still only checks for ledger version
  `13` / `static_actor_combat_profile_state`.
- Hermetic probe against `ApplyToVersion(13)` then import currently fails with:
  `table static_actor_combat_profiles has no column named chase_delay_ms`
  and `errors.Is(..., ErrStaticActorContentStateImportSchemaRequired) == false`.
- Track E prefers explicit migration/preflight safety over opaque driver errors.
- Safe, lane-scoped, one-commit, and testable without inventing upsert policy or
  registering a stock production driver. Return-delay / `0017` later landed on
  `main` with the world-lane GREEN; the matching import schema gate + Track E
  docs sync is owned by
  [static-actor import require return-delay schema](2026-08-28-static-actor-import-require-return-delay-schema.md).

## Contract frozen by this slice

1. Keep tip-`0013` as the export / quarantine / import-result migration identity
   (`migration_version=13`, `migration_name=static_actor_combat_profile_state`).
2. `ImportStaticActorContentState` still inserts `chase_delay_ms` into
   `static_actor_combat_profiles`.
3. Schema preflight must require tip-`0013` plus additive chase-delay `0016`.
   Live import after return-delay landed also requires additive `0017`; that
   three-boundary gate is owned by
   [static-actor import require return-delay schema](2026-08-28-static-actor-import-require-return-delay-schema.md).
4. Missing either tip-`0013` or chase-delay `0016` returns
   `ErrStaticActorContentStateImportSchemaRequired` (wrapped with the missing
   version/name and observed tip) **before** any INSERT.
5. Existing empty-DB missing-schema coverage stays green.
6. Existing tip-apply import proofs for this chase-delay slice applied through
   catalog tip `16`; current catalog tip after return-delay is `17`.
7. Upsert / auto-run / stock production driver remain explicitly deferred.
   ~~Return-delay / `0017` import schema gate~~ Done — see
   [static-actor import require return-delay schema](2026-08-28-static-actor-import-require-return-delay-schema.md).

## What this is not yet

- retipping static-actor exports to `migration_version=16`
- inventing upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- ~~world-lane `return_delay_ms` / migration `0017`~~ Done on `main` plus the
  import schema gate docs sync — see
  [static-actor import require return-delay schema](2026-08-28-static-actor-import-require-return-delay-schema.md)
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint / remote admin / secrets in git

## Likely files to change

- `internal/staticstore/static_actor_content_state_import.go`
- `internal/staticstore/static_actor_content_state_import_sqlite_harness_test.go`
- `internal/staticstore/migration_export.go` (named `0016` constants, if kept beside tip-`0013`)
- `docs/plans/2026-08-27-static-actor-content-state-sql-import-backfill.md` (pointer)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- this plan

## TDD and validation

Focused coverage under `//go:build sqlite_harness`:

- `ApplyToVersion(13)` then import →
  `errors.Is(..., ErrStaticActorContentStateImportSchemaRequired)`
- error text names missing version `16` /
  `static_actor_combat_profile_chase_delay`
- no combat-profile rows inserted on that fail-closed path
- existing apply-to-`16` import / duplicate / empty / empty-DB proofs stay green

Validation for this slice:

```bash
go test -tags=sqlite_harness ./internal/staticstore -run 'StaticActorContentStateImport' -count=1
go test ./internal/staticstore -run 'ImportStaticActorContentState' -count=1
gofmt -l internal/staticstore/static_actor_content_state_import.go internal/staticstore/static_actor_content_state_import_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- schema gate requires tip-`0013` **and** chase-delay `0016`
- tip-`0013`-only ledger fails closed with `SchemaRequired`, not raw SQL
- prior tip-`0016` import proofs remain green
- upsert / auto-run / stock driver remain explicitly deferred
- return-delay / `0017` follow-up is owned by
  [static-actor import require return-delay schema](2026-08-28-static-actor-import-require-return-delay-schema.md)

## Anti-goals / ordering constraints

- Do not retip export identity to version `16` in this slice.
- Do not invent upsert / merge policy in this slice.
- Do not register a production driver in stock binaries.
- Do not push `origin/main`; push only `origin/lane/persistence`.
