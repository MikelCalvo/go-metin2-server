# Safebox Lifecycle CloseSafebox Companion — 2026-08-21

## Objective

Emit the same self-only `CHAT_TYPE_COMMAND` / `CloseSafebox` companion already owned by `/close_safebox` whenever the bootstrap safebox presentation is cleared by death-floor, transfer/warp rebootstrap, or same-socket `/phase_select` / `/quit` / `/logout` teardown, so TMP4 clients hide a stale safebox window instead of keeping UI open after the busy flag is gone.

## Contract owned by this slice

1. When practice-mob retaliation drives the selected owner to the bootstrap `0`-HP floor while `/open_safebox` is open, the floor burst appends one self-only `GC::CHAT` / `CHAT_TYPE_COMMAND` `CloseSafebox` after the owned merchant `GC::SHOP END` (when present) and before/with the exchange close frames, and clears the same-socket open flag. Remembered same-session in-memory safebox contents stay in the session until logout / leave / process end.
2. When exact-position transfer / warp rebootstrap begins while the safebox presentation is open, the runtime clears that open flag and prepends one self-only `CloseSafebox` command chat before the transfer burst (after any merchant `GC::SHOP END`, before exchange `END` when those shells are also closed by the same transfer).
3. Same-socket `/phase_select`, `/quit`, and `/logout` likewise prepend `CloseSafebox` when the presentation is open, beside the already-owned merchant `SHOP END` / exchange `END` teardown order.
4. Inventory, equipment, quickslots, gold, ground handles, and same-session in-memory safebox item rows stay unchanged by the close companion itself.
5. Spec/QA name these automatic closes as presentation-shell hygiene only: no password/load, no durable safebox persistence, no money/mall.

## What this is not yet

- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money / mall
- partner-side open player-shop / cube busy-window exchange rejects
- refine-dialog client-visible cancel companion beyond the already-owned silent busy-flag clear

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxDeath|Safebox.*Transfer|CloseSafebox|TransferTriggerClosesOpenSafebox|PracticeMobDeathClearsOpenSafebox|PhaseSelect.*Safebox|Quit.*Safebox' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep money / password / durable persistence deferred.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
