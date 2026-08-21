# Refine Confirm Success Command Chat — 2026-08-21

## Objective

Emit the legacy client-visible `CHAT_TYPE_COMMAND` / `RefineSuceeded <type>` companion (intentional historical spelling) after an accepted `probability = 100` confirm-after-preview refine success burst, so TMP4-compatible clients play the success popup/sound instead of only applying silent inventory/gold refreshes.

## Contract owned by this slice

1. Accepted refine confirm still mutates inventory/gold through `ApplyRefineSuccess` first and keeps the already-owned material / material-removal quickslot / result / gold frame ordering.
2. After the gold `PLAYER_POINT_CHANGE`, the minimal runtime appends exactly one self-only `GC::CHAT` / `CHAT_TYPE_COMMAND` whose message is `RefineSuceeded <type>`, echoing the confirmed refine `type` byte.
3. Fail-closed confirm paths (busy windows, probability below `100`, mismatched identity, insufficient gold/materials, cancel `type = 255`) still emit no success command chat and perform no mutation.
4. Spec/QA name this companion beside the owned confirm-after-preview success seam; `RefineFailed <type>` and destroy/downgrade outcomes stay deferred.

## What this is not yet

- lower-probability refine failure/destroy outcomes and `CHAT_TYPE_COMMAND` `RefineFailed <type>`
- catalyst / scroll / socket-copy refine semantics
- peer-facing refine notifications
- mall / timeout / destruction / belt quickslot synchronization

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemRefineConfirmAfterPreviewProbability100|ItemRefineConfirmBusySafebox|ItemRefineConfirmProbabilityBelow100' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Freeze then implement the first lower-probability failure/destroy outcome with `RefineFailed <type>` once destroy-vs-keep-grade policy is chosen for bootstrap.
2. Keep mall / timeout / destruction / belt quickslot sync deferred.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
