# PvE vertical authored cube-recipe FileStore — 2026-09-08

## Objective

Close the remaining honesty gap after
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` already
rebuilds a fresh `gamed` runtime from persisted static-actor / interaction /
item-template / quest-state FileStores: authored `cube_recipes` still lived in
a hermetic MemoryStore, so daemon restart could only prove closed-window
silence plus the lab fallback.

This slice wires `config.Service.CubeRecipeStorePath` /
`METIN2_CUBE_RECIPE_STORE_PATH` / `METIN2_GAMED_CUBE_RECIPE_STORE_PATH` and
persists imported recipes through `cubestore.FileStore`. A later simulated
daemon restart against that same path rematerializes `cubeRecipesAuthored`
and `/cube r_info` `cube r_list 20022 1 27001,1` after a fresh CubeMaster
`INTERACT`.

## Contract frozen by this slice

1. `gamed` owns a dedicated file-backed cube-recipe parent
   (`go-metin2-server-cube-recipes/cube-recipes.json` by default; lab sample
   `/var/metin2/data/cube-recipes/cube-recipes.json`).
2. Importing portable `cube_recipes` Save()s the authored snapshot instead of
   keeping it in MemoryStore. Empty imports Save an empty snapshot so the next
   process does not keep stale rows.
3. Missing / empty FileStore snapshots still fall back to the in-memory lab
   bootstrap and mark `cubeRecipesAuthored = false`.
4. The composed PvE restart proof shares one explicit `CubeRecipeStorePath`
   across both runtimes, asserts `cubeRecipesAuthored`, and after the closed
   `/cube r_info` smoke reopens CubeMaster to emit the authored `r_list`.
5. `GET /local/runtime-config` reports `persistence.cube_recipe_store_path`.
   The backup-restore drill decoder accepts that optional field so unknown-field
   rejection cannot break retained runtime-config JSON, but it does **not** add
   `/local/cube-recipe*` backup/restore endpoints.

## What this is not yet

- cube-recipe backup / validate / restore ops endpoints
- `/cube make all`, injected-roll `1..99`, or authored `percent = 0`
- merchant sell-back of leftover cube materials
- binary cube headers / OR-materials

## SQL catalog companion (`0031_cube_recipe_state`)

Authored cube recipes now have a first schema-only SQL catalog companion
beside already-owned `cubestore.FileStore`. FileStore remains the live
rematerialize / backup path. This companion does **not** invent extra
`/local/cube-recipe*` backup endpoints, CLI quarantine/import kinds, a stock
production driver, remote admin, or secrets in git.

Contract:

1. Embedded catalog adds `0031_cube_recipe_state` after
   `0030_bootstrap_ground_item_ownership_timer` (catalog tip moves to `31`).
2. `up` creates:
   - `cube_recipe_npcs` (`PRIMARY KEY (npc_vnum)`)
   - `cube_recipes` (`PRIMARY KEY (npc_vnum, position)`, FK to npcs)
   - `cube_recipe_materials` (`PRIMARY KEY (npc_vnum, recipe_position, material_position)`)
   - `cube_recipe_material_options` (`PRIMARY KEY (npc_vnum, recipe_position, option_index, material_position)`)
   with FileStore CHECKs: npc/reward/item vnum `1..4294967295`, counts
   `1..65535`, percent `0..100`, gold `0..9223372036854775807` (signed BIGINT).
3. `down` drops child tables before parents.
4. `cubestore` owns:
   - `ExportCubeRecipeState` / `FileStore.ExportCubeRecipeState`
   - `Validate` / `QuarantineCubeRecipeStateExport`
   - `ImportCubeRecipeState` (quarantine → require ledger `31` /
     `cube_recipe_state` → parameterized INSERT; opt-in scoped replace)
5. CLI `quarantine-export` / `import-export` kinds stay the closed ten-kind
   set. `cube-recipe-state` is a cubestore primitive only in this slice.
6. Upsert-by-default, stock production driver, DB-backed live cube loading,
   and extra `/local/cube-recipe*` backup routes remain deferred.

## Verification

```bash
gofmt -w \
  internal/config/service.go \
  internal/config/service_test.go \
  internal/minimal/factory.go \
  internal/minimal/factory_test.go \
  internal/minimal/pve_vertical_authoring_test.go \
  internal/migratecli/backup_restore_drill.go \
  internal/migratecli/backup_restore_drill_test.go \
  internal/migratecli/contrib_lab_daemons_test.go
go test ./internal/config ./internal/migratecli ./internal/minimal -count=1 \
  -run 'Test(LoadServiceUsesBootstrapPersistenceDefaultsWhenEnvIsMissing|LoadServiceUsesGlobalBootstrapPersistenceOverrides|LoadServicePrefersServiceSpecificBootstrapPersistenceOverrides|ValidatePersistenceConfigAcceptsDistinctExplicitPaths|GameRuntimeConfigSnapshotReportsPersistenceStoreLocations|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn|RunBackupRestoreDrillAcceptsOptionalCubeRecipeStorePathWithoutBackupEndpoints|ContribLabDaemons)$'
gofmt -w \
  db/migrations/catalog_test.go \
  db/migrations/plan_test.go \
  internal/cubestore/migration_export.go \
  internal/cubestore/migration_export_quarantine.go \
  internal/cubestore/migration_export_test.go \
  internal/cubestore/cube_recipe_state_import.go \
  internal/cubestore/cube_recipe_state_import_sqlite_harness_test.go \
  internal/migratecli/quarantine_export.go \
  internal/migratecli/cube_recipe_state_kind_test.go \
  internal/minimal/factory_test.go \
  internal/minimal/gamed_migration_ops_test.go
go test ./db/migrations ./internal/cubestore ./internal/migratecli -count=1 \
  -run 'Test(BuiltInCatalogIsValid|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn|ExportCubeRecipeState|QuarantineCubeRecipeState|ImportCubeRecipeState|FileStoreExportCubeRecipeState|ExportQuarantineKindsRemainClosedWithoutCubeRecipeState|RunQuarantineExportRejectsCubeRecipeStateKind|RunImportExportRejectsCubeRecipeStateKind|RunImportExportStatusRejectsCubeRecipeStateKind)$'
go test ./internal/minimal -count=1 -timeout=360s \
  -run 'Test(GameRuntimeMigrationStatusPlansBuiltInCatalogWithoutExecutingSQL|GameRuntimeMigrationCatalogSummaryReturnsMetadataOnlyCatalog|RegisterGamedMigrationQuarantineExportOpsServesCatalogExportQuarantineAndDrivers)$'
go test -tags=sqlite_harness ./db/migrations ./internal/cubestore -count=1 \
  -run 'TestSQLiteHarness(AppliesCatalogAndReadsLedgerSnapshot|CubeRecipeStateImport)'
git diff --check
```
