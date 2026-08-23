# Hermetic Interaction-Visibility MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `interactionstore.MemoryStore`, `itemstore.MemoryStore`, and `queststate.MemoryStore` into `internal/minimal/interaction_visibility_test.go` so operator preview / quest-gate visibility proofs no longer allocate disposable content FileStores or `QuestStateStorePath` for non-mutating preview assertions.

This closes the explicit follow-up from [hermetic quest-flag turn-in MemoryStore tests](2026-08-21-hermetic-quest-flag-turnin-memorystore-tests.md).

## Why now

- Kill-quest, PvE vertical, and focused `quest_flag_*` turn-in suites already inject content MemoryStores through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore`.
- Visibility tests still seed interaction / item / quest JSON FileStores even though they only assert read-only previews and non-mutation of quest-state.
- Constructor injection and MemoryStore seams already exist; no new persistence product decision is required.

## Contract frozen by this slice

1. All focused tests in `internal/minimal/interaction_visibility_test.go` construct content stores with MemoryStores:
   - `interactionstore.NewMemoryStore()` for authored interaction definitions
   - `itemcatalog.NewMemoryStore()` when item templates are required for shop / reward / consume previews
   - `queststate.NewMemoryStore()` when quest-flag seeds or non-mutation assertions are required
2. Tests that need a quest-state seed inject it through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore` with `staticstore.NewMemoryStore()` and do **not** set `QuestStateStorePath`.
3. Quest-state non-mutation assertions load through the same injected `questStore` handle instead of reopening a path-backed FileStore.
4. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for the broader item/interaction suites; this slice does not flip those helpers globally.
5. Login-ticket FileStores remain intentional for session bootstrapping in these visibility proofs.

## What this is not yet

- global MemoryStore conversion of `newInteractionDefinitionStore` / `newItemTemplateStore`
- broader filesystem decoupling of account / login-ticket stores in the same helpers
- production `NewGameRuntime` accepting injected content MemoryStores
- ground-item restart durability, SQL import/backfill, durable safebox persistence, or Docker LABEL workflow-run metadata

## TDD and validation

```bash
go test ./internal/minimal -run 'InteractionVisibility' -count=1
gofmt -w internal/minimal/interaction_visibility_test.go
git diff --check
```

## Follow-up options

1. ~~Optionally widen the same MemoryStore pattern to interaction-definition runtime create/upsert/remove suites.~~ Done: see [hermetic interaction-definitions runtime MemoryStore tests](2026-08-22-hermetic-interaction-definitions-runtime-memorystore-tests.md). ~~Optionally convert shared `newInteractionDefinitionStore` / `newItemTemplateStore` helpers to MemoryStore once neighboring item gameplay suites are ready for the same coupling reduction.~~ Done: see [hermetic shared interaction/item template helpers](2026-08-23-hermetic-shared-interaction-item-template-helpers.md).
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports drive recovery.
4. ~~Optional Docker `LABEL` workflow-run metadata remains deferred.~~ Done: see [Docker LABEL workflow-run metadata](2026-08-22-docker-label-workflow-run-metadata.md).
