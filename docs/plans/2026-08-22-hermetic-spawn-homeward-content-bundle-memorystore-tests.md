# Hermetic Spawn-Homeward Content-Bundle MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `staticstore.MemoryStore` and `interactionstore.MemoryStore` into `internal/minimal/spawn_homeward_content_bundle_test.go` so content-bundle import prune / restore / replace proofs for pending spawn-group homeward-step schedules no longer allocate disposable path-isolated static/interaction FileStores.

This closes the remaining content-lane content-bundle hermetic follow-up after [hermetic content-bundle runtime MemoryStore tests](2026-08-22-hermetic-content-bundle-runtime-memorystore-tests.md).

## Why now

- Track D bootstrap quest/NPC/regen authoring is otherwise closed.
- `content_bundle_runtime_test.go` already injects MemoryStores for export/import/summary proofs.
- The neighboring homeward-schedule content-bundle suites still seeded static / interaction JSON FileStores even though constructor injection and MemoryStore seams already exist.
- These proofs only assert live runtime schedule prune/restore/replace behavior through the same `Store` injection path; they do not need filesystem-backed content snapshots.

## Contract frozen by this slice

1. Focused tests in `internal/minimal/spawn_homeward_content_bundle_test.go` construct content stores with MemoryStores:
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

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameRuntime(FailedContentBundleImportRestoresSpawnGroupHomewardStepSchedule|NoOpContentBundleImportPrunesStaleSpawnGroupHomewardStepSchedule|SuccessfulContentBundleReplacementClearsStaleSpawnGroupHomewardStepSchedule)$' -count=1
gofmt -w internal/minimal/spawn_homeward_content_bundle_test.go
git diff --check
```

## Follow-up options

1. ~~Optionally convert the twin return-step content-bundle prune/restore/replace suites.~~ Done: see [hermetic spawn-return content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-return-content-bundle-memorystore-tests.md).
2. Optionally convert shared `newInteractionDefinitionStore` / `newItemTemplateStore` helpers to MemoryStore once neighboring item gameplay suites are ready for the same coupling reduction.
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
4. Keep branching quest scripts and multi-count regen deferred.
5. Keep durable safebox persistence / password load deferred.
