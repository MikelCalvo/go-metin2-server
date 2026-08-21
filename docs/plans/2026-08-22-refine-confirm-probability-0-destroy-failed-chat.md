# Refine Confirm Probability-0 Destroy + RefineFailed Chat — 2026-08-22

## Objective

Freeze the first deterministic lower-probability refine failure outcome for remembered `probability = 0` confirm-after-preview dialogs: consume gold/materials, destroy the source carried item, clear the refine dialog, persist, and emit self-only `CHAT_TYPE_COMMAND` `RefineFailed <type>` so TMP4 clients play the failure popup/sound.

## Contract to own

1. Preview may still open for authored `refine_info.probability` in `0..100` (already owned); only confirm behavior changes for `probability = 0`.
2. Matching confirm while a remembered dialog has `probability = 0` succeeds as a failure outcome only when the same busy/zero-HP/source-identity/template/gold/material/result-template guards already owned by the success path pass.
3. On that accepted failure path the runtime atomically:
   - deducts `refine_info.cost` from live gold;
   - consumes authored material counts from carried inventory (same stack-decrement / slot-clear ordering as success);
   - removes the source carried item entirely from its remembered cell (destroy; no result `vnum` placement);
   - clears the same-socket refine-dialog presentation and shared-world refine busy flag;
   - persists the selected-character account snapshot;
   - emits self-only frames in this order: material `ITEM_UPDATE` / `ITEM_DEL` refreshes as needed, then for each fully consumed material cell the owned item-removal `GC::QUICKSLOT_DEL`, then source-cell `ITEM_DEL`, then for any item quickslots bound to that destroyed source cell the same owned `GC::QUICKSLOT_DEL`, then `PLAYER_POINT_CHANGE` for `POINT_GOLD` with the negative cost amount and resulting gold value, then one self-only `CHAT_TYPE_COMMAND` with message `RefineFailed <type>` echoing the confirmed refine `type`.
4. `probability` values in `1..99` remain fail-closed with no frames/mutation until a later RNG/determinism policy owns them; `probability = 100` keeps the already-owned success path.
5. Spec/QA/packet-matrix name this narrow destroy + `RefineFailed` companion beside the owned `RefineSuceeded` success seam.

## What this is not yet

- random rolls for `1..99`
- catalyst / scroll / hyuniron / musin / black-dragon semantics
- keep-grade / downgrade / safe-refine failure variants
- peer-facing refine notifications
- guild / money-only fee variants

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/player -run 'ApplyRefineFailure|ApplyRefineDestroy' -count=1`
- `go test ./internal/minimal -run 'ItemRefineConfirmAfterPreviewProbability0|ItemRefineConfirmProbabilityBelow100|ItemRefineConfirmBusy' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Implement the GREEN path after this contract freeze.
2. Keep `1..99` RNG deferred until operators choose a deterministic test seam.
3. Keep catalyst / keep-grade failure variants deferred.
