# Hermetic Spawn-Return Content-Bundle MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `staticstore.MemoryStore` and `interactionstore.MemoryStore` into the three content-bundle return-step schedule suites in `internal/minimal/shared_world_test.go` so import prune / restore / replace proofs for pending spawn-group return-step schedules no longer allocate disposable path-isolated static/interaction FileStores.

This closes the return-step twin of [hermetic spawn-homeward content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-homeward-content-bundle-memorystore-tests.md).

## Why now

- Track D bootstrap quest/NPC/regen authoring is otherwise closed.
- Homeward content-bundle prune/restore/replace suites already inject MemoryStores.
- The neighboring return-step schedule suites still seeded static / interaction JSON FileStores even though constructor injection and MemoryStore seams already exist.
- These proofs only assert live runtime schedule prune/restore/replace behavior through the same `Store` injection path; they do not need filesystem-backed content snapshots.

## Contract frozen by this slice

1. Focused tests construct content stores with MemoryStores:
   - `TestGameRuntimeFailedContentBundleImportRestoresSpawnGroupReturnStepSchedule`
   - `TestGameRuntimeNoOpContentBundleImportPrunesStaleSpawnGroupReturnStepSchedule`
   - `TestGameRuntimeSuccessfulContentBundleReplacementClearsStaleSpawnGroupReturnStepSchedule`
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()`
2. Runtime construction continues through `newGameRuntimeWithAccountStoreAndContentStores(...)`.
3. Failed-import restore, no-op prune, and successful-replacement clear proofs keep the same live schedule assertions.
4. Login-ticket FileStores remain intentional for session bootstrap.
5. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for the broader item/interaction suites.

## What this is not yet

- global MemoryStore conversion of `newInteractionDefinitionStore` / `newItemTemplateStore`
- broader filesystem decoupling of account / login-ticket stores
- production `NewGameRuntime` accepting injected content MemoryStores
- branching quest scripts / new NPC service kinds
- SQL import/backfill execution
- converting the larger spawn-lifecycle FileStore suites that still intentionally exercise path-isolated persistence

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameRuntime(FailedContentBundleImportRestoresSpawnGroupReturnStepSchedule|NoOpContentBundleImportPrunesStaleSpawnGroupReturnStepSchedule|SuccessfulContentBundleReplacementClearsStaleSpawnGroupReturnStepSchedule)$' -count=1
gofmt -w internal/minimal/shared_world_test.go
git diff --check
```

## Follow-up options

1. Optionally convert neighboring content-bundle import suites that still seed disposable static/interaction FileStores when those proofs do not require filesystem rematerialize.
2. Optionally convert shared `newInteractionDefinitionStore` / `newItemTemplateStore` helpers to MemoryStore once neighboring item gameplay suites are ready for the same coupling reduction.
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exists.
4. Keep branching quest scripts and multi-count regen deferred.
