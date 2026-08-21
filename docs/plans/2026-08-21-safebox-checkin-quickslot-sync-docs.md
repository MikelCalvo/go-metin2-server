# Safebox Check-in Quickslot Sync Docs Freeze — 2026-08-21

## Objective

Align the quickslot and packet-matrix contracts with the already-shipped open-presentation `SAFEBOX_CHECKIN` removal path so docs stop claiming safebox check-in leaves item quickslots stale.

## Contract frozen by this slice

1. Accepted open-presentation `SAFEBOX_CHECKIN` that fully clears a carried inventory cell deletes matching item quickslots for that cell, persists the inventory/quickslot account snapshot, and emits self-only `GC::QUICKSLOT_DEL(position)` after `ITEM_DEL` and before `GC::SAFEBOX_SET`.
2. Skill/command quickslots that happen to share the same byte payload remain unchanged.
3. Accepted `SAFEBOX_CHECKOUT` and same-session `SAFEBOX_ITEM_MOVE` do not delete or retarget carried-inventory item quickslots.
4. Spec/QA/packet-matrix name this beside the already-owned use / sell / drop / refine / exchange removal sync paths; runtime behavior is unchanged by this docs-only freeze.

## What this is not yet

- mall / timeout / destruction / belt quickslot synchronization
- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money
- partner-side open player-shop / cube busy-window exchange rejects

## TDD and validation

Focused coverage (already green; re-run as proof the docs match shipped behavior):

- `go test ./internal/minimal -run 'TestGameRuntimeSafeboxCheckinWhileOpenMovesItemToInMemorySafebox' -count=1`
- `git diff --check`

## Follow-up options

1. Keep mall / timeout / destruction / belt quickslot sync deferred.
2. Keep password / money / durable safebox persistence deferred.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
