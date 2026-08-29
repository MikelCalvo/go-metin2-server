# Static-Actor Import Require Return-Delay Schema — 2026-08-28

## Objective

Close the remaining Track E operator-docs gap after world-lane migration
`0017_static_actor_combat_profile_return_delay` landed: document and pin that
`ImportStaticActorContentState` fail-closes with
`ErrStaticActorContentStateImportSchemaRequired` when tip-`0013` static-actor
SQL import targets a ledger that owns chase-delay `0016` but **not** additive
`0017` return_delay, instead of a raw `no such column: return_delay_ms` driver
error.

The hermetic gate and INSERT binding already landed with the world-lane return
delay GREEN (`fe7ad63d`). This slice is the persistence-lane ops/docs sync that
keeps Track E / migration-contract / development runbooks honest about catalog
tip `0017` and the three-boundary import preflight.

## Why now

- Tip-`0013` export/quarantine/import already round-trip `return_delay_ms` on
  combat-profile rows because live FileStore / content-bundle owns authored
  return delay beside chase delay.
- Migration `0017` adds the SQL column; `requireStaticActorContentStateSchema`
  already requires ledger versions `13` + `16` + `17` and
  `TestSQLiteHarnessStaticActorContentStateImportRejectsTip0016WithoutReturnDelaySchema`
  is green under `-tags=sqlite_harness`.
- Track E / migration-contract / `docs/development.md` still narrate catalog tip
  `0016` and treat return-delay `0017` as deferred after the chase-delay gate.
- Those contradictions are production-ops hazards for import/backfill cutovers
  that stop at tip-`0016`.

## Contract frozen by this slice

1. Keep tip-`0013` as the export / quarantine / import-result migration identity
   (`migration_version=13`, `migration_name=static_actor_combat_profile_state`).
2. `ImportStaticActorContentState` still inserts both `chase_delay_ms` and
   `return_delay_ms` into `static_actor_combat_profiles`.
3. Schema preflight must require **all** of:
   - ledger version `13` / `static_actor_combat_profile_state`
   - ledger version `16` / `static_actor_combat_profile_chase_delay`
   - ledger version `17` / `static_actor_combat_profile_return_delay`
4. Missing tip-`0013` → `SchemaRequired` naming version `13`.
5. Missing chase-delay after tip-`0013` → `SchemaRequired` naming version `16`.
6. Missing return-delay after tip-`0016` → `SchemaRequired` naming version `17`
   (wrapped with observed tip) **before** any INSERT.
7. Existing empty-DB / tip-`0013`-only / tip-`0016`-only / apply-to-`17` import
   proofs stay green.
8. Catalog tip reported by this return-delay slice was
   `0017_static_actor_combat_profile_return_delay` (export tip remains `0013`;
   safebox money export tip remains `0015`). Current catalog tip after
   homeward-delay is `0018` — see
   [static-actor import require homeward-delay schema](2026-08-28-static-actor-import-require-homeward-delay-schema.md).
9. Upsert / auto-run / stock production driver remain explicitly deferred.
   ~~Homeward-delay / `0018` import schema gate~~ Done — see
   [static-actor import require homeward-delay schema](2026-08-28-static-actor-import-require-homeward-delay-schema.md).
10. No new Go production code in this slice: the gate already exists; this owns
    the Track E docs/contract sync plus any residual pointer fixes.

## What this is not yet

- retipping static-actor exports to `migration_version=16` or `17`
- inventing upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- ~~homeward-delay `homeward_delay_ms` / migration `0018`~~ Done on `main` plus
  the import schema gate docs sync — see
  [static-actor import require homeward-delay schema](2026-08-28-static-actor-import-require-homeward-delay-schema.md)
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint / remote admin / secrets in git
- claiming DB-backed live static-actor loading

## Likely files to change

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

- catalog tip for this slice was `0017_static_actor_combat_profile_return_delay`
  (later advanced to `0018` by the homeward-delay docs sync)
- static-actor import preflight names additive `0016` **and** `0017`
- tip-`0016`-only reject is documented beside tip-`0013`-only chase-delay reject
- upsert / stock production driver stay deferred

## Exit criteria

- operator docs no longer claim catalog tip `0016` as current tip
- return-delay import schema gate is marked done on Track E / migration-contract
- chase-delay plan marks the deferred `0017` follow-up done via this plan
- homeward-delay / `0018` follow-up is owned by
  [static-actor import require homeward-delay schema](2026-08-28-static-actor-import-require-homeward-delay-schema.md)
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not retip export identity away from version `13` in this slice.
- Do not invent homeward-delay migration / authorship on this lane (owned later
  by the homeward-delay docs sync).
- Do not register a production driver in stock binaries.
- Do not push `origin/main`; push only `origin/lane/persistence`.
