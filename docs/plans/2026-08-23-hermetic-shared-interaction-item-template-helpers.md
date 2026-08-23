# Hermetic Shared Interaction/Item Template Test Helpers — 2026-08-23

## Objective

Convert the shared `internal/minimal` test helpers
`newInteractionDefinitionStore` and `newItemTemplateStore` from disposable
JSON FileStores to hermetic MemoryStores, and collapse the duplicate
`newMemoryInteractionDefinitionStore` / `newMemoryItemTemplateStore` wrappers
onto those helpers.

This closes the remaining documented content-lane hermetic follow-up from:

- [hermetic interaction-definitions runtime MemoryStore tests](2026-08-22-hermetic-interaction-definitions-runtime-memorystore-tests.md)
- [hermetic content-bundle runtime MemoryStore tests](2026-08-22-hermetic-content-bundle-runtime-memorystore-tests.md)
- [hermetic spawn-homeward content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-homeward-content-bundle-memorystore-tests.md)
- [hermetic spawn-return content-bundle MemoryStore tests](2026-08-22-hermetic-spawn-return-content-bundle-memorystore-tests.md)
- [hermetic interaction-visibility MemoryStore tests](2026-08-22-hermetic-interaction-visibility-memorystore-tests.md)

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring and negative fixtures
  are otherwise closed.
- Focused content suites already inject MemoryStores explicitly; the broader
  item/interaction gameplay suites still entered through shared FileStore
  helpers even though persistence assertions only use `Store.Load()` /
  `Store.Save()`.
- Direct `interactionstore.NewFileStore` / `itemcatalog.NewFileStore` call sites
  that intentionally exercise path-isolated rematerialize, backup, or restart
  recovery remain unchanged.

## Contract frozen by this slice

1. `newInteractionDefinitionStore(...)` constructs
   `interactionstore.NewMemoryStore()`, seeds via `Save`, and returns the
   `Store` interface.
2. `newItemTemplateStore(...)` constructs `itemcatalog.NewMemoryStore()`, seeds
   via `Save`, and returns the `Store` interface.
3. `newMemoryInteractionDefinitionStore` / `newMemoryItemTemplateStore` become
   thin aliases over those shared helpers so older call sites keep compiling
   without duplicating seed logic.
4. Login-ticket / account FileStores remain intentional where tests still assert
   filesystem rematerialize or ticket lifecycle.
5. Explicit FileStore construction for restart/backup/path-isolated proofs stays
   on those dedicated suites.

## What this is not yet

- converting every remaining direct FileStore construction in item/restart/
  proximity suites
- production `NewGameRuntime` accepting injected content MemoryStores by default
- branching quest scripts / pack AI / new NPC service kinds
- SQL import/backfill execution

## TDD and validation

```bash
gofmt -w internal/minimal/shared_world_test.go internal/minimal/interaction_visibility_test.go
go test ./internal/minimal -run 'TestGameRuntime(InteractionDefinitions|InteractionDefinition|CreateInteractionDefinition|CreateWarp|CreateShopPreview|CreateOpenSafebox|CreateQuestFlag|UpsertQuestFlag|UpsertInteractionDefinition|UpsertWarp|UpsertShopPreview|RemoveInteractionDefinition|InteractionVisibility)' -count=1
go test ./internal/minimal -run 'TestGameRuntime(ItemUse|ItemDrop|ItemMove|ItemGive|ShopBuy|Merchant)' -count=1
git diff --check
```

## Follow-up options

1. Optionally convert remaining direct disposable static/interaction/item
   FileStore constructions in non-rematerialize suites when those proofs do not
   require filesystem coupling.
2. Keep import/backfill execution deferred until a driver-backed harness and
   backup policy exist.
3. Keep branching quest scripts and pack AI / synchronized respawn deferred.
