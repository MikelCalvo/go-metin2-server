# Hermetic Interaction-Visibility Quest MemoryStore Tests — 2026-08-22

## Objective

Wire the already-landed hermetic `interactionstore.MemoryStore`, `itemstore.MemoryStore`, `staticstore.MemoryStore`, and `queststate.MemoryStore` into the quest-gated / `quest_flag` interaction-visibility preview suites so operator-preview proofs no longer depend on disposable path-isolated content FileStores.

## Why now

- Focused `quest_flag` turn-in gameplay tests already inject content MemoryStores through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore`.
- The matching `InteractionVisibility` preview suites still allocated disposable interaction / item-template / quest-state JSON FileStores (plus `QuestStateStorePath`) even though constructor injection and MemoryStore seams already exist.
- Track D bootstrap quest/NPC/regen authoring is otherwise closed; this is the documented follow-up that keeps content-lane QA hermetic without inventing a new NPC service kind or endpoint-only surface.

## Contract frozen by this slice

1. Quest-gated mismatch and `quest_flag` preview tests in `internal/minimal/interaction_visibility_test.go` construct content stores with MemoryStores and inject them through `newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore`.
2. Interaction definitions, item templates, and quest-flag seeds are Saved into the injected MemoryStores before runtime construction.
3. No-mutation assertions load through the same injected `questStore` handle instead of reopening a path-backed FileStore.
4. `config.Service` for these tests no longer sets `QuestStateStorePath`.
5. Shared FileStore helpers (`newInteractionDefinitionStore`, `newItemTemplateStore`) remain unchanged for non-quest visibility suites and the broader item/interaction suites.
6. Login-ticket FileStores remain intentional for session bootstrap.

## What this is not yet

- hermetic MemoryStore migration for non-quest interaction-visibility suites (`info` / `talk` / service happy-path previews without quest state)
- broader filesystem decoupling of account / login-ticket stores in the same helpers
- production `NewGameRuntime` accepting injected content MemoryStores
- branching quest scripts / new NPC service kinds
- SQL-backed content repositories or import/backfill tooling

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameRuntimeInteractionVisibilityReturns(QuestGated|QuestFlag)' -count=1
gofmt -w internal/minimal/interaction_visibility_test.go
git diff --check
```

## Follow-up options

1. ~~Optionally widen the same MemoryStore pattern to the remaining non-quest interaction-visibility suites that still allocate disposable interaction/item FileStores.~~ Done via the broader interaction-visibility conversion and [hermetic shared interaction/item template helpers](2026-08-23-hermetic-shared-interaction-item-template-helpers.md).
2. Keep branching quest scripts deferred.
3. Keep new NPC service kinds deferred until accepted durable safebox/storage mutations exist beyond the owned bootstrap presentation.
4. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
