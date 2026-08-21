# Hermetic Quest-State MemoryStore Gameplay Tests — 2026-08-20

## Objective

Wire the already-landed hermetic `queststate.MemoryStore` into kill-quest and PvE vertical gameplay tests so those content-lane proofs no longer depend on the disk-backed quest-state snapshot for compare-and-set credit, gates, and turn-in assertions.

## Contract frozen by this slice

1. Kill-quest / PvE gameplay tests in:
   - `internal/minimal/kill_quest_credit_test.go`
   - `internal/minimal/pve_vertical_authoring_test.go`
   continue to construct the runtime through the ordinary store helper (which still creates a discarded path-isolated `FileStore` for constructor overlap safety).
2. Immediately after construction, each of those tests replaces `runtime.questStateStore` with `queststate.NewMemoryStore()`, matching the existing `ExportsCharacterQuestStateThroughMemoryStoreSeam` pattern.
3. Bundle import, kill-quest credit, require gates, `quest_flag` turn-in, and reconnect assertions continue to exercise the same `Store` / `ApplyTransition` surface; only the backing implementation changes.
4. No production runtime constructor signature change is required for this slice.

## What this is not yet

- ~~constructor-level optional quest-state store injection~~ Done: see [quest-state store constructor injection](2026-08-21-queststate-store-constructor-injection.md).
- ~~hermetic static-content / interaction MemoryStore repository seams in gameplay tests~~ Done: see [hermetic content MemoryStore gameplay tests](2026-08-21-hermetic-content-memorystore-gameplay-tests.md).
- SQL-backed quest-state repositories or import/backfill tooling
- broader filesystem decoupling of account / ticket stores in the same tests

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'KillQuest|PveVerticalAuthoring' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. ~~Add matching hermetic seams for static-content exports once callers need the same coupling reduction.~~ Done for kill-quest / PvE gameplay tests: see [hermetic content MemoryStore gameplay tests](2026-08-21-hermetic-content-memorystore-gameplay-tests.md).
2. ~~Optionally extend `newGameRuntimeWithStores...` to accept an injected `queststate.Store` so hermetic tests no longer construct-and-discard a path-isolated `FileStore`.~~ Done: see [quest-state store constructor injection](2026-08-21-queststate-store-constructor-injection.md).
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
