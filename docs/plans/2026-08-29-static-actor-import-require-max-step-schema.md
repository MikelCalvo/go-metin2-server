# Static-Actor Import Require Max-Step Schema — 2026-08-29

## Objective

Close the remaining Track E operator-docs gap after world-lane migration
`0019_static_actor_combat_profile_max_step` landed: document and pin that
`ImportStaticActorContentState` fail-closes with
`ErrStaticActorContentStateImportSchemaRequired` when tip-`0013` static-actor
SQL import targets a ledger that owns homeward-delay `0018` but **not** additive
`0019` max-step, instead of a raw `no such column: max_step` driver error.

The hermetic gate and INSERT binding already landed with the world-lane
max-step GREEN (`7cc42de5`). This slice is the persistence-lane ops/docs
sync that keeps Track E / migration-contract / development runbooks honest about
catalog tip `0019` and the five-boundary import preflight.

## Why now

- Tip-`0013` export/quarantine/import already round-trip `max_step` on
  combat-profile rows because live FileStore / content-bundle owns authored
  max-step beside chase / return / homeward delay.
- Migration `0019` adds the SQL column; `requireStaticActorContentStateSchema`
  already requires ledger versions `13` + `16` + `17` + `18` + `19` and
  `TestSQLiteHarnessStaticActorContentStateImportRejectsTip0018WithoutMaxStepSchema`
  is green under `-tags=sqlite_harness`.
- Track E / migration-contract follow-ups still stop at the homeward-delay
  `0018` docs sync and treat max-step `0019` import-schema tip sync as unspoken
  even though the catalog tip and hermetic gate already own `0019`.
- Those contradictions are production-ops hazards for import/backfill cutovers
  that stop at tip-`0018`.

## Contract frozen by this slice

1. Keep tip-`0013` as the export / quarantine / import-result migration identity
   (`migration_version=13`, `migration_name=static_actor_combat_profile_state`).
2. `ImportStaticActorContentState` still inserts `chase_delay_ms`,
   `return_delay_ms`, `homeward_delay_ms`, and `max_step` into
   `static_actor_combat_profiles`.
3. Schema preflight must require **all** of:
   - ledger version `13` / `static_actor_combat_profile_state`
   - ledger version `16` / `static_actor_combat_profile_chase_delay`
   - ledger version `17` / `static_actor_combat_profile_return_delay`
   - ledger version `18` / `static_actor_combat_profile_homeward_delay`
   - ledger version `19` / `static_actor_combat_profile_max_step`
4. Missing tip-`0013` → `SchemaRequired` naming version `13`.
5. Missing chase-delay after tip-`0013` → `SchemaRequired` naming version `16`.
6. Missing return-delay after tip-`0016` → `SchemaRequired` naming version `17`
   (wrapped with observed tip) **before** any INSERT.
7. Missing homeward-delay after tip-`0017` → `SchemaRequired` naming version `18`
   (wrapped with observed tip) **before** any INSERT.
8. Missing max-step after tip-`0018` → `SchemaRequired` naming version `19`
   (wrapped with observed tip) **before** any INSERT.
9. Existing empty-DB / tip-`0013`-only / tip-`0016`-only / tip-`0017`-only /
   tip-`0018`-only / apply-to-`19` import proofs stay green.
10. Catalog tip reported by operator docs is
    `0019_static_actor_combat_profile_max_step` (export tip remains `0013`;
    safebox money export tip remains `0015`).
11. Upsert / auto-run / stock production driver remain explicitly deferred.
12. No new Go production code in this slice: the gate already exists; this owns
    the Track E docs/contract sync plus any residual pointer fixes.

## What this is not yet

- retipping static-actor exports to `migration_version=16`, `17`, `18`, or `19`
- inventing upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- profile-authored `reaction_delay_ms` / a future `0020` migration (world/combat
  lane ownership until GREEN, then a matching Track E tip sync)
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint / remote admin / secrets in git
- claiming DB-backed live static-actor loading

## Likely files to change

- `docs/plans/2026-08-28-static-actor-import-require-homeward-delay-schema.md` (pointer)
- `docs/plans/2026-08-28-static-actor-import-require-return-delay-schema.md` (pointer)
- `docs/plans/2026-08-28-static-actor-import-require-chase-delay-schema.md` (pointer)
- `docs/plans/2026-08-27-static-actor-content-state-sql-import-backfill.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer to this plan)
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

- catalog tip is `0019_static_actor_combat_profile_max_step`
- static-actor import preflight names additive `0016`, `0017`, `0018`, **and** `0019`
- tip-`0018`-only reject is documented beside tip-`0017`-only homeward-delay reject
- upsert / stock production driver stay deferred

## Exit criteria

- operator docs no longer treat catalog tip `0018` as the unfinished max-step gap
- max-step import schema gate is marked done on Track E / migration-contract
- homeward-delay plan marks the deferred `0019` follow-up done via this plan
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not retip export identity away from version `13` in this slice.
- Do not invent `reaction_delay_ms` migration / authorship on this lane.
- Do not register a production driver in stock binaries.
- Do not push `origin/main`; push only `origin/lane/persistence`.
