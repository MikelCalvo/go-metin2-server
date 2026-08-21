# Refine Confirm Material Quickslot Sync — 2026-08-21

## Objective

When a `probability = 100` refine confirm fully consumes a material carried cell, clear matching item quickslots for that cell with the same removal synchronization already owned by use / sell / drop / exchange finalize, so the client and persisted snapshot cannot keep a stale binding to an empty inventory cell.

## Contract frozen by this slice

1. Accepted refine confirm still mutates inventory/gold through `ApplyRefineSuccess` first; quickslot deletion remains a caller-owned sync after those live material removals, matching other item-removal paths.
2. For each refine material change with `ItemRemoved = true`, the minimal runtime runs `SyncItemQuickslotsForItemRemoval` on that carried cell and appends self-only `GC::QUICKSLOT_DEL` frames after the material `ITEM_DEL` / `ITEM_UPDATE` refreshes and before the result-cell `ITEM_SET` and gold `PLAYER_POINT_CHANGE`.
3. Partial material stack decrements leave that cell's item quickslots unchanged.
4. Unrelated skill/command quickslots that happen to share the same byte payload remain unchanged.
5. Spec/QA name refine-confirm material quickslot deletion beside the already-owned use / sell / drop / exchange removal sync paths.

## What this is not yet

- refine `probability < 100` failure/destroy outcomes
- catalyst / scroll / socket-copy refine semantics
- mall / timeout / destruction quickslot deletion beyond the currently owned removal set
- peer-facing refine notifications

Accepted open-presentation `SAFEBOX_CHECKIN` source-cell quickslot deletion is now owned beside this refine path; see `docs/plans/2026-08-21-safebox-checkin-quickslot-sync-docs.md` and `spec/protocol/quickslot-bootstrap.md`.

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemRefineConfirmDeletesMaterialItemQuickslots|ItemRefineConfirmAfterPreviewProbability100' -count=1`
- `go test ./internal/player -run 'ApplyRefineSuccessProbability100' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep lower-probability refine outcomes deferred until failure/destroy contracts are frozen; accepted `probability = 100` confirm now also emits self-only `CHAT_TYPE_COMMAND` `RefineSuceeded <type>` (`docs/plans/2026-08-21-refine-confirm-success-command-chat.md`).
2. Keep mall / timeout / destruction quickslot sync deferred; accepted `SAFEBOX_CHECKIN` removal sync is already owned.
3. ACCEPT and commit-time busy-window reject info-chat now reuse the START requester/partner strings; optional authored/template-backed overrides remain deferred.
