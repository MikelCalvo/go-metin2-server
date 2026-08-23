# Open-Safebox INTERACT Merchant Auto-Close — 2026-08-21

## Objective

Close an active same-socket bootstrap merchant window before the client-visible frames of a successful authored `open_safebox` warehouse `INTERACT`, matching the already-owned non-merchant interaction close rule used by `info` / talk / quest-flag / failure deliveries and the accepted safebox-mutation merchant auto-close path.

## Why now

- `open_safebox` is live through ordinary `INTERACT` and is part of the authored PvE vertical fixture beside `Merchant`.
- Spec already said non-merchant static-actor `INTERACT` should prepend `GC::SHOP END`, but the successful warehouse path skipped `prependMerchantCloseFrame`.
- Leaving the merchant open after warehouse open made later `SHOP BUY` / `SHOP END` behave as if the window were still valid and broke the composed PvE fixture once warehouse was added after merchant unlock.

## Contract frozen by this slice

1. When a successful `open_safebox` `INTERACT` applies and the same socket still has an active bootstrap merchant buy window, the runtime emits self-only `GC::SHOP END` first, clears that merchant presentation, then emits any optional authored warehouse info chat plus the warehouse password-prompt frames (`CHAT_TYPE_COMMAND` `ShowMeSafeboxPassword`). Matching `/safebox_password` later opens with `GC::SAFEBOX_SIZE` and durable rematerialized `SAFEBOX_SET` / money frames; this slice only owns the merchant close ordering before the warehouse challenge frames.
2. Later explicit `SHOP END` / packet `SHOP BUY` on that same socket fail closed until the merchant is opened again.
3. Gold / inventory / persistence stay unchanged by the presentation close itself.
4. Spec/QA name warehouse `INTERACT` as part of the non-merchant merchant-window close family.

## What this is not yet

- ~~password / DB load and `SAFEBOX_WRONG_PASSWORD`~~ Done later: warehouse password challenge + durable optional password.
- ~~durable safebox persistence / money / mall~~ Durable rematerialize + money are owned later; mall stays deferred.
- transfer / MOVE / phase-select CloseSafebox client companion frames beyond the already-owned silent busy-flag clear paths
- branching quest scripts

## TDD and validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowInteractingWithOpenSafeboxActorClosesOpenMerchantWindowBeforeSafeboxSize|TestGameSessionFlowStaticActorOpenSafeboxInteractionReturnsSafeboxSize|TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn' -count=1
gofmt -w internal/minimal/factory.go internal/minimal/shared_world_test.go internal/minimal/pve_vertical_authoring_test.go
git diff --check
```

## Follow-up options

1. ~~Keep durable safebox persistence / password load deferred.~~ Done later; see `docs/plans/2026-08-23-open-safebox-npc-password-challenge-docs-sync.md`.
2. Optional: emit client-visible `CloseSafebox` on transfer / MOVE-out-of-range / phase_select if TMP4 clients leave a stale warehouse window after those boundaries.
3. Keep branching quest scripts deferred.
