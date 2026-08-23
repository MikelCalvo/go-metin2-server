# Hermetic Interaction-Definitions Runtime MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `interactionstore.MemoryStore` and `itemstore.MemoryStore` helpers into `internal/minimal/interaction_definitions_runtime_test.go` so create/upsert/remove / exact-lookup / sorted-snapshot proofs no longer allocate disposable path-isolated interaction or item-template FileStores.

This closes the next documented content-lane hermetic follow-up after [hermetic content-bundle runtime MemoryStore tests](2026-08-22-hermetic-content-bundle-runtime-memorystore-tests.md) and [hermetic interaction-visibility MemoryStore tests](2026-08-22-hermetic-interaction-visibility-memorystore-tests.md).

## Why now

- Track D bootstrap quest/NPC/regen authoring is otherwise closed.
- Interaction-definition runtime create/upsert/remove suites still seeded interaction / item JSON FileStores even though `newMemoryInteractionDefinitionStore` / `newMemoryItemTemplateStore` already exist in the same package.
- Persistence assertions in those suites already go through `Store.Load()` / `Store.Save()`, so MemoryStore keeps the same contract without filesystem coupling.
- Login-ticket FileStores remain intentional for runtime construction where those helpers still require a ticket store.

## Contract frozen by this slice

1. Focused tests in `internal/minimal/interaction_definitions_runtime_test.go` construct content stores with MemoryStores:
   - `newMemoryInteractionDefinitionStore(...)` for authored interaction definitions
   - `newMemoryItemTemplateStore(...)` when merchant catalog item templates are required
2. Post-mutation persistence assertions continue to load through the same injected store handles.
3. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for the broader item gameplay suites that still use them.
4. Login-ticket FileStores remain intentional for session/runtime construction.

## What this is not yet

- global MemoryStore conversion of `newInteractionDefinitionStore` / `newItemTemplateStore`
- broader filesystem decoupling of account / login-ticket stores
- production `NewGameRuntime` accepting injected content MemoryStores
- branching quest scripts / new NPC service kinds
- SQL import/backfill execution

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameRuntime(InteractionDefinitions|InteractionDefinition|CreateInteractionDefinition|CreateWarp|CreateShopPreview|CreateOpenSafebox|CreateQuestFlag|UpsertQuestFlag|UpsertInteractionDefinition|UpsertWarp|UpsertShopPreview|RemoveInteractionDefinition)' -count=1
gofmt -w internal/minimal/interaction_definitions_runtime_test.go
git diff --check
```

## Follow-up options

1. ~~Optionally convert shared `newInteractionDefinitionStore` / `newItemTemplateStore` helpers to MemoryStore once neighboring item gameplay suites are ready for the same coupling reduction.~~ Done: see [hermetic shared interaction/item template helpers](2026-08-23-hermetic-shared-interaction-item-template-helpers.md).
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep branching quest scripts deferred; multi-count regen authoring is now owned separately.
4. ~~Keep durable safebox persistence / password load deferred.~~ Done later; see `docs/plans/2026-08-23-open-safebox-npc-password-challenge-docs-sync.md`.
