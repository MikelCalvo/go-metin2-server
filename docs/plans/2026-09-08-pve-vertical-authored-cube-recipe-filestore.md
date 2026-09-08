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
git diff --check
```
