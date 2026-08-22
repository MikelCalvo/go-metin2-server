# Hermetic Quest-Flag Turn-In MemoryStore Tests — 2026-08-21

## Objective

Wire the already-landed hermetic `interactionstore.MemoryStore`, `itemstore.MemoryStore`, `staticstore.MemoryStore`, and `queststate.MemoryStore` into the focused `quest_flag` turn-in gameplay tests so consume/reward proofs no longer depend on disposable path-isolated content FileStores.

## Why now

- Kill-quest / PvE vertical gameplay tests already inject content MemoryStores through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore`.
- The five `quest_flag_*` turn-in suites still allocate disposable interaction / item-template / quest-state JSON FileStores (plus a nil static store) even though constructor injection and MemoryStore seams already exist.
- Account and login-ticket FileStores stay intentional: those tests still assert persisted gold/inventory rematerialization through the account store.

## Contract frozen by this slice

1. The following focused gameplay tests construct content stores with MemoryStores and inject them through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore`:
   - `internal/minimal/quest_flag_consume_experience_test.go`
   - `internal/minimal/quest_flag_consume_gold_test.go`
   - `internal/minimal/quest_flag_consume_items_test.go`
   - `internal/minimal/quest_flag_reward_items_test.go`
   - `internal/minimal/quest_flag_reward_overflow_test.go`
2. Interaction definitions, item templates, and quest-flag seeds are Saved into the injected MemoryStores before runtime construction.
3. Quest-state assertions load through the same injected `questStore` handle instead of reopening a path-backed FileStore.
4. `config.Service` for these tests no longer sets `QuestStateStorePath`.
5. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for the broader item/interaction suites; this slice does not flip those helpers globally.
6. Account / login-ticket FileStores remain for gold/inventory persistence assertions.

## What this is not yet

- ~~hermetic MemoryStore migration for `interaction_visibility_test.go` quest-flag previews~~ Done: see [hermetic interaction-visibility quest MemoryStore tests](2026-08-22-hermetic-interaction-visibility-quest-memorystore-tests.md).
- broader filesystem decoupling of account / login-ticket stores in the same helpers
- production `NewGameRuntime` accepting injected content MemoryStores
- branching quest scripts / new NPC service kinds

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowStaticActorQuestFlag(Consume|Reward)' -count=1
gofmt -w internal/minimal/quest_flag_*.go
git diff --check
```

## Follow-up options

1. ~~Optionally widen the same MemoryStore pattern to quest-flag interaction-visibility preview tests that still allocate disposable content FileStores.~~ Done: see [hermetic interaction-visibility quest MemoryStore tests](2026-08-22-hermetic-interaction-visibility-quest-memorystore-tests.md).
2. Keep branching quest scripts deferred.
3. Keep new NPC service kinds deferred until accepted durable safebox/storage mutations exist beyond the owned bootstrap presentation.
