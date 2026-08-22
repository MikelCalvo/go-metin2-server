# Hermetic Content-Bundle Runtime MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `staticstore.MemoryStore`, `interactionstore.MemoryStore`, `itemstore.MemoryStore`, and `queststate.MemoryStore` into `internal/minimal/content_bundle_runtime_test.go` so content-bundle export / import / summary / persistence proofs no longer allocate disposable path-isolated content FileStores or `QuestStateStorePath`.

This closes the remaining content-lane follow-up from [hermetic content MemoryStore gameplay tests](2026-08-21-hermetic-content-memorystore-gameplay-tests.md) after kill-quest, PvE vertical, quest-flag turn-in, and interaction-visibility suites already moved.

## Why now

- Track D bootstrap quest/NPC/regen authoring is otherwise closed.
- Content-bundle runtime tests still seeded static / interaction / item / quest JSON FileStores even though constructor injection and MemoryStore seams already exist.
- Persistence assertions in those suites already go through `Store.Load()` / `Store.Save()`, so MemoryStore keeps the same contract without filesystem coupling.

## Contract frozen by this slice

1. Focused tests in `internal/minimal/content_bundle_runtime_test.go` construct content stores with MemoryStores:
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()` / `newMemoryInteractionDefinitionStore(...)`
   - `itemcatalog.NewMemoryStore()` when item templates are required
   - `queststate.NewMemoryStore()` when quest-state seeds or post-import persistence assertions are required
2. Quest-state seeds inject through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore` and do **not** set `QuestStateStorePath`.
3. Post-import persistence assertions continue to load through the same injected store handles.
4. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for the broader item/interaction suites.
5. Login-ticket FileStores remain intentional for session bootstrap where those tests enter game.

## What this is not yet

- global MemoryStore conversion of `newInteractionDefinitionStore` / `newItemTemplateStore`
- broader filesystem decoupling of account / login-ticket stores
- production `NewGameRuntime` accepting injected content MemoryStores
- branching quest scripts / new NPC service kinds
- SQL import/backfill or `0013` combat-profile content-state tip

## TDD and validation

```bash
go test ./internal/minimal -run 'ContentBundle' -count=1
gofmt -w internal/minimal/content_bundle_runtime_test.go
git diff --check
```

## Follow-up options

1. ~~Optionally widen the same MemoryStore pattern to neighboring interaction-definition runtime create/upsert/remove suites.~~ Done: see [hermetic interaction-definitions runtime MemoryStore tests](2026-08-22-hermetic-interaction-definitions-runtime-memorystore-tests.md). Optionally convert shared `newInteractionDefinitionStore` / `newItemTemplateStore` helpers to MemoryStore once neighboring item gameplay suites are ready for the same coupling reduction.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep branching quest scripts and multi-count regen deferred.
4. Persistence/content migration follow-up for portable `combat_profiles[]` remains owned by [PvE vertical authoring 0012 export combat-profile gap](2026-08-22-pve-vertical-authoring-0012-export-combat-profile-gap.md) / [combat-profile content-state migration](2026-08-22-combat-profile-content-state-migration.md).
