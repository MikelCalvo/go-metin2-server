# Hermetic Shared-World Content-Bundle Import MemoryStore Tests — 2026-08-24

## Objective

Convert the neighboring content-bundle import suites still seeded in
`internal/minimal/shared_world_test.go` from disposable static / interaction /
item FileStores to hermetic MemoryStores so live import / reject / fanout proofs
no longer allocate path-isolated content JSON stores.

This closes the remaining documented content-lane hermetic follow-up from:

- [hermetic spawn-return content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-return-content-bundle-memorystore-tests.md)
- [hermetic spawn-group operator position MOVE MemoryStore tests](2026-08-24-hermetic-spawn-group-operator-position-move-memorystore-tests.md)

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring and negative fixtures
  are otherwise closed.
- Return-step / homeward / operator MOVE content-bundle suites already inject
  MemoryStores.
- These neighboring import proofs only assert live runtime import, reject, and
  visibility/target fanout behavior; they do not rematerialize from disk or
  exercise backup/restore paths.

## Contract owned by this slice

1. The following focused tests construct content stores with MemoryStores:
   - `TestGameRuntimeImportContentBundleQueuesSpawnGroupVisibilityForOnlinePlayers`
   - `TestGameRuntimeRejectsDuplicateStaticActorContentBundleWithoutMutatingRuntime`
   - `TestGameRuntimeImportsContentBundleCombatProfilesBeforeSpawnGroups`
   - `TestGameRuntimeImportsExampleFormulaCombatProfileBundleBeforeSpawnGroups`
   - `TestGameRuntimeImportsContentBundleDropTablesAsSpawnGroupRewardDescriptor`
   - `TestGameRuntimeImportsContentBundleRegenSpawnsAsSpawnGroups`
   - `TestGameRuntimeReimportsFormulaOnlyCombatProfileBundleIdempotently`
   - `TestGameRuntimeFailedContentBundleImportDoesNotLeakSpawnVisibilityFrames`
   - `TestGameRuntimeFailedContentBundleImportDoesNotLeakSelectedTargetClear`
2. Construction uses:
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()`
   - `itemcatalog.NewMemoryStore()` when item templates are required
3. Runtime construction continues through the existing
   `newGameRuntimeWithAccountStoreAndContentStores(...)`,
   `newGameRuntimeWithStoresAndTransferTriggers(...)`, and
   `newGameRuntimeWithStoresAndTransferTriggersAndItemStore(...)` seams.
4. Login-ticket / account FileStores remain intentional for session bootstrap
   and account persistence where those suites already use them.
5. Explicit FileStore construction for restart/backup/path-isolated proofs stays
   on those dedicated suites.

## Explicit non-goals

- converting remaining FileStores in item/restart suites that still assert
  filesystem rematerialize
- production `NewGameRuntime` accepting injected content MemoryStores by default
- branching quest scripts / pack AI / new NPC service kinds
- SQL import/backfill execution

## Validation

```bash
gofmt -w internal/minimal/shared_world_test.go
go test ./internal/minimal -run 'Test(GameRuntimeImportContentBundleQueuesSpawnGroupVisibilityForOnlinePlayers|GameRuntimeRejectsDuplicateStaticActorContentBundleWithoutMutatingRuntime|GameRuntimeImportsContentBundleCombatProfilesBeforeSpawnGroups|GameRuntimeImportsExampleFormulaCombatProfileBundleBeforeSpawnGroups|GameRuntimeImportsContentBundleDropTablesAsSpawnGroupRewardDescriptor|GameRuntimeImportsContentBundleRegenSpawnsAsSpawnGroups|GameRuntimeReimportsFormulaOnlyCombatProfileBundleIdempotently|GameRuntimeFailedContentBundleImportDoesNotLeakSpawnVisibilityFrames|GameRuntimeFailedContentBundleImportDoesNotLeakSelectedTargetClear)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Optionally convert remaining direct disposable static/interaction FileStore
   constructions in non-rematerialize suites when those proofs do not require
   filesystem coupling.~~ Done for the proximity aggro suppress suites: see
   [hermetic proximity aggro suppress MemoryStore tests](2026-08-24-hermetic-proximity-aggro-suppress-memorystore-tests.md).
2. Keep import/backfill execution deferred until a driver-backed harness and
   backup policy exist.
3. Keep branching quest scripts and pack AI / synchronized respawn deferred.
