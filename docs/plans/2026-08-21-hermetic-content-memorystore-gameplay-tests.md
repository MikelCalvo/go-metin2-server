# Hermetic Content MemoryStore Gameplay Tests — 2026-08-21

## Objective

Wire the already-landed hermetic `staticstore.MemoryStore`, `interactionstore.MemoryStore`, and `itemstore.MemoryStore` into kill-quest and PvE vertical gameplay tests so those content-lane proofs no longer depend on disposable path-isolated FileStores for import, persistence round-trips, and live interaction assertions.

Also close the remaining operator-preview coverage gap for gated `open_safebox` so warehouse mismatch previews match the already-owned warp / shop / talk / info gate-preview contract.

## Why now

- Quest-state constructor injection already lets kill-quest / PvE tests inject `queststate.MemoryStore`.
- Those same tests still allocate disposable JSON FileStores for static actors, interaction definitions, and item templates even though MemoryStore seams already exist and are proven by repository-export tests.
- `InteractionVisibility` already resolves gated `open_safebox` mismatch previews, but only warp / shop / talk / info had focused mismatch coverage.

## Contract frozen by this slice

1. Kill-quest / PvE gameplay tests in:
   - `internal/minimal/kill_quest_credit_test.go`
   - `internal/minimal/pve_vertical_authoring_test.go`
   construct content stores with:
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()`
   - `itemcatalog.NewMemoryStore()`
2. Bundle import, kill-quest credit, require gates, `quest_flag` turn-in, warehouse smoke, and reconnect assertions continue to exercise the same `Store` / `Load` / `Save` surface; only the backing implementation changes.
3. Empty MemoryStores still fail closed as missing snapshots (`ErrSnapshotNotFound`) until the first successful Save/import, matching FileStore empty-start behavior.
4. `TestGameRuntimeInteractionVisibilityReturnsQuestGatedOpenSafeboxMismatchPreviewWithoutMutatingQuestState` proves a gated warehouse mismatch preview returns `Quest requirements are not met.` without mutating quest-state.
5. No production runtime constructor signature change is required for this slice.

## What this is not yet

- broader filesystem decoupling of account / login-ticket stores in the same helpers
- production `NewGameRuntime` accepting injected content MemoryStores
- SQL-backed content repositories or import/backfill tooling
- branching quest scripts / new NPC service kinds

## TDD and validation

```bash
go test ./internal/minimal -run 'KillQuest|PveVerticalAuthoring|InteractionVisibilityReturnsQuestGatedOpenSafebox' -count=1
gofmt -w internal/minimal/kill_quest_credit_test.go internal/minimal/pve_vertical_authoring_test.go internal/minimal/interaction_visibility_test.go
git diff --check
```

## Follow-up options

1. Optionally widen the same MemoryStore pattern to neighboring content gameplay tests that still allocate disposable static/interaction/item FileStores.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep branching quest scripts deferred.
