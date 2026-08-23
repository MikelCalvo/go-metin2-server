# Safebox Open Walk-Away Auto-Close Contract Freeze — 2026-08-23

## Objective

Freeze the first already-open warehouse walk-away auto-close contract before opening RED, so the owned `/safebox_password` open-anchor distance gate can grow into oracle-shaped mid-open distance enforcement without inventing CloseSafebox ordering, cooldown arming, or pending-challenge interaction mid-implementation.

## Why docs-first

`/safebox_password` now rejects when `ApproxDistance(currentXY, openAnchorXY) > 1000`, but an already-open warehouse presentation still stays open if the player walks beyond that anchor. Opening RED without freezing whether MOVE/SyncPosition closes the presentation, whether that close arms the owned 10-second reopen cooldown, and whether pending challenges are cleared would invent policy. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Scope: only while the same-socket safebox presentation is already open (`hasActiveSafeboxOpen`). Pending-only `ShowMeSafeboxPassword` challenges stay on the already-owned `/safebox_password` distance reject path and are **not** auto-cleared by movement in this slice.
2. Trigger: after an accepted same-map `MOVE` or `SyncPosition` that updates the selected character's live `X`/`Y`, if an open-anchor is remembered and `ApproxDistance(currentXY, openAnchorXY) > 1000`, clear the open presentation with the same self-only `CHAT_TYPE_COMMAND` `CloseSafebox` companion already owned by slash/floor/transfer close paths.
3. Side effects of that auto-close:
   - clear open/busy presentation flag (exchange START busy guard lifts);
   - clear any pending password challenge that happens to exist;
   - arm the owned 10-second reopen cooldown exactly like slash `/close_safebox`;
   - leave durable safebox cells/password/money unchanged;
   - do **not** invent inventory/gold/point frames.
4. Lab `/open_safebox` that never recorded an open anchor does **not** auto-close on movement until a later warehouse challenge overwrites/sets an anchor; if a prior warehouse challenge left an anchor and the lab opener is later used, movement beyond that remembered anchor may still auto-close (docs name this explicitly rather than inventing a lab-only exempt path).
5. Transfer / warp / floor / phase-lifecycle closes keep their already-owned CloseSafebox companions; this slice does not duplicate those paths.
6. Spec/QA name walk-away auto-close beside the owned open-anchor password distance gate; mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.

## Locale / wording note

Successful walk-away close emits only the already-owned `CloseSafebox` command chat. No new info-chat string is introduced in this bootstrap slice.

## What this is not yet

- mall open/checkout / `MALL_*` runtime emission
- TMP4 CG `SAFEBOX_MONEY` request header
- client `SAFEBOX_CHANGE_PASSWORD` / DB answer frames
- partner-side open player-shop / cube exchange busy rejects
- auto-clear of pending-only password challenges on walk-away without an open presentation

## TDD shape after the freeze lands

1. Runtime/session: warehouse challenge → password open → walk so `ApproxDistance > 1000` → self-only `CloseSafebox`, busy flag cleared, reopen cooldown armed; durable cells unchanged.
2. Runtime/session: pending-only challenge + walk-away leaves pending intact (password path still owns distance reject); lab `/open_safebox` without a prior warehouse anchor does not auto-close on movement.
3. Negative: movement still inside `<= 1000` leaves the open presentation untouched.

## Status

Docs-first freeze only on `lane/items`. Implementation / RED remain the next items-lane step; mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.
