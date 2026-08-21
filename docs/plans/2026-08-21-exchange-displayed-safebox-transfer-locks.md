# Exchange-Displayed Safebox Transfer Locks — 2026-08-21

## Objective

Freeze the missing safebox half of the exchange display-lock matrix so `SAFEBOX_CHECKIN` of an already-displayed carried source cell and `SAFEBOX_CHECKOUT` into an already-displayed carried destination cell fail closed with no frames while the exchange shell stays open/cancellable.

## Contract owned by this slice

1. While a carried item identity remains displayed in the bootstrap exchange shell, same-socket `SAFEBOX_CHECKIN` of that displayed source cell fails closed with no frames, leaves inventory/safebox/quickslot/gold/persistence unchanged, and leaves the exchange shell open and cancellable.
2. While a carried destination cell remains displayed in the bootstrap exchange shell, same-socket `SAFEBOX_CHECKOUT` into that displayed destination fails closed with no frames, leaves inventory/safebox/gold/persistence unchanged, and leaves the exchange shell open and cancellable.
3. Non-displayed safebox check-in/out while an exchange shell is open keep the already-owned close-on-success path (`self/peer GC::EXCHANGE END` before inventory/safebox refresh frames).
4. Spec/QA/packet-matrix name safebox check-in/out beside the already-owned display locks (`ITEM_USE`, move/equip/unequip, `ITEM_USE_TO_ITEM`, drop, merchant sell).

## What this is not yet

- password / DB load / durable safebox persistence
- safebox money / mall
- new exchange finalize semantics
- partner-side open player-shop / cube busy-window rejects

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxCheckinOfDisplayedExchangeItem|SafeboxCheckoutIntoDisplayedExchangeCell' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optional: fold the new safebox display-lock cases into the broader `TestGameRuntimeDisplayedExchangeItemMutationsFailClosedWithoutClosingShell` matrix once that test needs a safebox reopen harness.
2. Keep money / password / durable persistence deferred.
3. Next items-lane candidates remain anti-flag / refine probability / ownership seams that are still explicitly deferred.
