# Item Use Confirm-When-Use Ordinary Path — 2026-08-22

## Objective

Reclassify template-authored `confirm_when_use` from a server-side fail-closed gate into the already-owned ordinary consumable `ITEM_USE` / `/use_item` success path, matching TMP4 client evidence that the confirm dialog is client-local and the follow-up packet is the same `ITEM_USE`.

## Contract to own

1. `confirm_when_use = true` is a client presentation hint projected into `ITEM_SET.flags` (`ITEM_FLAG_CONFIRM_WHEN_USE`). It is not a server ack / second-packet protocol in this bootstrap.
2. After the client-local confirm dialog, a matching `ITEM_USE` / `/use_item` against a carried cell whose loaded template authors `confirm_when_use` plus a valid non-equippable `use_effect` follows the already-owned consumable success path (`use_effect` point mutation, stack/quickslot sync, persistence, optional `SPECIAL_EFFECT` / info message).
3. Existing transfer / selected-character / `use_reject_message` / busy merchant-exchange teardown guards stay unchanged and still apply before mutation.
4. `quest_use`, `quest_use_multiple`, and `applicable` remain fail-closed for direct consumable use until those seams are owned.
5. Spec/QA stop claiming that `confirm_when_use` itself rejects direct use; they name it as client-local confirm + ordinary server use.

## What this is not yet

- a server-owned confirm request/ack packet family
- quest-use / applicable acceptance
- peer-facing use notifications beyond already-owned exchange teardown

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/player -run 'UseItem.*ConfirmWhenUse|ConfirmWhenUse' -count=1`
- `go test ./internal/minimal -run 'ItemUse.*ConfirmWhenUse|ConfirmWhenUse' -count=1` (if a session proof is added)
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep `quest_use` / `quest_use_multiple` / `applicable` fail-closed until those seams are owned.
2. Optional later: operator/QA note that client UI must show the confirm dialog when `ITEM_FLAG_CONFIRM_WHEN_USE` is set.

## Status

Docs/spec contract freeze. RED/GREEN implementation follows in the next items-lane step.
