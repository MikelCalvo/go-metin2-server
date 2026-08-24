# Hermetic Spawn-Group Operator Position MOVE MemoryStore Tests — 2026-08-24

## Objective

Convert the two disposable content FileStores in
`internal/minimal/spawn_group_operator_position_move_test.go` to hermetic
MemoryStores so same-map operator/runtime position MOVE and presentation
delete/readd proofs no longer allocate path-isolated static/interaction JSON
stores.

This closes the remaining documented content-lane hermetic follow-up from:

- [hermetic shared interaction/item template helpers](2026-08-23-hermetic-shared-interaction-item-template-helpers.md)
- [hermetic spawn-homeward content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-homeward-content-bundle-memorystore-tests.md)
- [hermetic spawn-return content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-return-content-bundle-memorystore-tests.md)

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring and negative fixtures
  are otherwise closed.
- These two runtime proofs only assert live retained-viewer MOVE vs delete/readd
  choreography after `ImportContentBundle`; they do not rematerialize from disk
  or exercise backup/restore paths.
- Neighboring content-bundle import prune/restore suites already inject
  `staticstore.NewMemoryStore()` / `interactionstore.NewMemoryStore()`.

## Contract owned by this slice

1. `TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionUsesRetainedViewerMove`
   and `TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPresentationKeepsDeleteReadd`
   construct content stores with:
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()`
2. Runtime construction continues through
   `newGameRuntimeWithAccountStoreAndContentStores(...)`.
3. Login-ticket FileStores remain intentional for session bootstrap.
4. The pure shared-world engagement-clear proof in the same file stays
   FileStore-free (registry-only) and is unchanged.
5. Explicit FileStore construction for restart/backup/path-isolated proofs stays
   on those dedicated suites.

## Explicit non-goals

- converting remaining FileStores in item/restart/proximity suites that still
  assert filesystem rematerialize
- production `NewGameRuntime` accepting injected content MemoryStores by default
- branching quest scripts / pack AI / new NPC service kinds
- SQL import/backfill execution

## Validation

```bash
gofmt -w internal/minimal/spawn_group_operator_position_move_test.go
go test ./internal/minimal -run 'Test(GameRuntimeUpdateStaticActorSameMapSpawnGroupPositionUsesRetainedViewerMove|GameRuntimeUpdateStaticActorSameMapSpawnGroupPresentationKeepsDeleteReadd|SharedWorldRegistryUpdateStaticActorSameMapSpawnGroupPositionClearsEngagement)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Optionally convert remaining direct disposable static/interaction FileStore
   constructions in non-rematerialize suites when those proofs do not require
   filesystem coupling.~~ Done for the neighboring shared-world content-bundle
   import suites: see
   [hermetic shared-world content-bundle import MemoryStore tests](2026-08-24-hermetic-shared-world-content-bundle-import-memorystore-tests.md).
2. Keep import/backfill execution deferred until a driver-backed harness and
   backup policy exist.
3. Keep branching quest scripts and pack AI / synchronized respawn deferred.
