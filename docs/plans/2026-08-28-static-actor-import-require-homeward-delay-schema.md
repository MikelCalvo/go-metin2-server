# Static-Actor Import Require Homeward-Delay Schema — 2026-08-28

## Objective

Close the remaining Track E operator-docs gap after world-lane migration
`0018_static_actor_combat_profile_homeward_delay` landed: document and pin that
`ImportStaticActorContentState` fail-closes with
`ErrStaticActorContentStateImportSchemaRequired` when tip-`0013` static-actor
SQL import targets a ledger that owns return-delay `0017` but **not** additive
`0018` homeward_delay, instead of a raw `no such column: homeward_delay_ms`
driver error.

The hermetic gate and INSERT binding already landed with the world-lane
homeward delay GREEN (`62b3ce50`). This slice is the persistence-lane ops/docs
sync that keeps Track E / migration-contract / development runbooks honest about
catalog tip `0018` and the four-boundary import preflight.

## Why now

- Tip-`0013` export/quarantine/import already round-trip `homeward_delay_ms` on
  combat-profile rows because live FileStore / content-bundle owns authored
  homeward delay beside chase / return delay.
- Migration `0018` adds the SQL column; `requireStaticActorContentStateSchema`
  already requires ledger versions `13` + `16` + `17` + `18` and
  `TestSQLiteHarnessStaticActorContentStateImportRejectsTip0017WithoutHomewardDelaySchema`
  is green under `-tags=sqlite_harness`.
- Track E / migration-contract / `docs/development.md` still narrate catalog tip
  `0017` and treat homeward-delay `0018` as deferred after the return-delay gate.
- Those contradictions are production-ops hazards for import/backfill cutovers
  that stop at tip-`0017`.

## Contract frozen by this slice

1. Keep tip-`0013` as the export / quarantine / import-result migration identity
   (`migration_version=13`, `migration_name=static_actor_combat_profile_state`).
2. `ImportStaticActorContentState` still inserts `chase_delay_ms`,
   `return_delay_ms`, and `homeward_delay_ms` into `static_actor_combat_profiles`.
3. Schema preflight must require **all** of:
   - ledger version `13` / `static_actor_combat_profile_state`
   - ledger version `16` / `static_actor_combat_profile_chase_delay`
   - ledger version `17` / `static_actor_combat_profile_return_delay`
   - ledger version `18` / `static_actor_combat_profile_homeward_delay`
4. Missing tip-`0013` → `SchemaRequired` naming version `13`.
5. Missing chase-delay after tip-`0013` → `SchemaRequired` naming version `16`.
6. Missing return-delay after tip-`0016` → `SchemaRequired` naming version `17`
   (wrapped with observed tip) **before** any INSERT.
7. Missing homeward-delay after tip-`0017` → `SchemaRequired` naming version `18`
   (wrapped with observed tip) **before** any INSERT.
8. Existing empty-DB / tip-`0013`-only / tip-`0016`-only / tip-`0017`-only /
   apply-to-`18` import proofs stay green.
9. Catalog tip reported by operator docs is
   `0018_static_actor_combat_profile_homeward_delay` (export tip remains `0013`;
   safebox money export tip remains `0015`).
10. Upsert / auto-run / stock production driver remain explicitly deferred.
11. No new Go production code in this slice: the gate already exists; this owns
    the Track E docs/contract sync plus any residual pointer fixes.

## What this is not yet

- retipping static-actor exports to `migration_version=16`, `17`, or `18`
- inventing upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- profile-authored `max_step` / a future `0019` migration (world-lane ownership)
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint / remote admin / secrets in git
- claiming DB-backed live static-actor loading

## Likely files to change

- `docs/plans/2026-08-28-static-actor-import-require-return-delay-schema.md` (pointer)
- `docs/plans/2026-08-28-static-actor-import-require-chase-delay-schema.md` (pointer)
- `docs/plans/2026-08-27-static-actor-content-state-sql-import-backfill.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- this plan

## TDD and validation

No new Go tests required when the hermetic gate is already green. Re-prove:

```bash
go test -tags=sqlite_harness ./internal/staticstore -run 'StaticActorContentStateImport' -count=1
go test ./internal/staticstore -run 'ImportStaticActorContentState' -count=1
go test ./db/migrations -count=1
git diff --check
```

Spot-check that Track E / migration-contract / development wording:

- catalog tip is `0018_static_actor_combat_profile_homeward_delay`
- static-actor import preflight names additive `0016`, `0017`, **and** `0018`
- tip-`0017`-only reject is documented beside tip-`0016`-only return-delay reject
- upsert / stock production driver stay deferred

## Exit criteria

- operator docs no longer claim catalog tip `0017` as current tip
- homeward-delay import schema gate is marked done on Track E / migration-contract
- return-delay plan marks the deferred `0018` follow-up done via this plan
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not retip export identity away from version `13` in this slice.
- Do not invent `max_step` migration / authorship on this lane.
- Do not register a production driver in stock binaries.
- Do not push `origin/main`; push only `origin/lane/persistence`.
