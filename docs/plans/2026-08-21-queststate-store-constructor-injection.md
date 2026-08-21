# Quest-State Store Constructor Injection — 2026-08-21

## Objective

Extend the deepest game-runtime constructor so hermetic kill-quest / PvE gameplay tests can inject `queststate.MemoryStore` at construction time instead of construct-and-discard of a path-isolated `FileStore` followed by a post-hoc `runtime.questStateStore = ...` assignment.

## Why now

- `queststate.MemoryStore` and hermetic kill-quest / PvE gameplay coverage already landed.
- Those tests still force `ValidatePersistenceConfig` to allocate a disposable quest-state JSON path, construct a `FileStore`, then overwrite the field.
- Constructor injection is the follow-up called out by `2026-08-20-hermetic-queststate-memorystore-gameplay-tests.md`.

## Contract frozen by this slice

1. Keep `newGameRuntimeWithStoresAndTransferTriggersAndItemStore(...)` signature stable for the hundreds of existing call sites; it continues to default quest-state to `queststate.NewFileStore(serviceQuestStateStorePath(cfg))`.
2. Add `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(..., questState queststate.Store, transferTriggers ...)` as the deepest constructor:
   - non-nil `questState` is used as-is
   - nil `questState` keeps the ordinary FileStore default
3. Kill-quest and PvE vertical gameplay tests construct through the new helper with `queststate.NewMemoryStore()` and no disposable `QuestStateStorePath`.
4. `TestGameRuntimeExportsCharacterQuestStateThroughMemoryStoreSeam` likewise injects its pre-seeded MemoryStore at construction time.
5. Persistence-path validation still runs against the ordinary config defaults / remaining file stores; MemoryStore injection does not invent a fake quest-state path.

## What this is not yet

- hermetic static-content / interaction MemoryStore constructor injection
- broader filesystem decoupling of account / ticket / item stores in the same helpers
- SQL-backed quest-state repositories or import/backfill tooling
- production `NewGameRuntime` accepting an injected quest-state store

## TDD and validation

```bash
go test ./internal/minimal -run 'KillQuest|PveVerticalAuthoring|ExportsCharacterQuestStateThroughMemoryStoreSeam' -count=1
gofmt -w internal/minimal/factory.go internal/minimal/kill_quest_credit_test.go internal/minimal/pve_vertical_authoring_test.go internal/minimal/factory_test.go
git diff --check
```

## Follow-up options

1. Optionally add matching hermetic constructor injection for static/interaction stores once callers need the same coupling reduction.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep branching quest scripts deferred.
