# Safebox Money / SAFEBOX_MONEY_CHANGE Contract Freeze — 2026-08-23

## Objective

Freeze the first durable warehouse-money mutation contract before opening RED, so open-presentation safebox ownership can grow into deposit/withdraw without inventing store schema, slash wording, open-burst emission, or gold overflow policy mid-implementation.

## Why docs-first

`SAFEBOX_MONEY_CHANGE` (`0x0834`) is already codec-owned as signed `money int32 LE`, but the shipped runtime never emits it and `safeboxstore` has no durable money field. The TMP4-era CG header set owns check-in/out/item-move/mall-checkout and does **not** expose a client `SAFEBOX_MONEY` request header beside them, while the external behavior oracle still models warehouse gold with SAVE/WITHDRAW states and persists `dwGold` with safebox rows. Opening RED without freezing slash ingress, durable schema, open-burst emission, and carrier/overflow bounds would invent policy. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Durable schema: optional `money` on each `safeboxstore` character row.
   - omitted / zero means `0` warehouse gold;
   - persisted JSON omits the field when zero so existing password/cell-only snapshots stay byte-compatible;
   - store validation rejects negative money and values above `math.MaxInt32` (wire is signed `int32`);
   - `ReplaceCharacterCells` / `ReplaceCharacterPassword` preserve existing money; a new `ReplaceCharacterMoney` upserts money while preserving password + cells.
2. Open-burst emission: on every successful open-presentation path (`/open_safebox` lab opener and matching `/safebox_password` success), after `GC::SAFEBOX_SIZE` and in-range `SAFEBOX_SET` rows, emit one self-only `GC::SAFEBOX_MONEY_CHANGE` whose `money` is the durable effective warehouse gold for that login + character id (default `0`). Closed presentation / pending password challenge alone must not emit money frames.
3. Local slash deposit `/safebox_money_save <amount>` (talking chat) while the selected character is in `GAME`, above the bootstrap zero-HP floor, and the same-socket safebox presentation is already open:
   - missing / non-positive / non-uint32-parseable / over-`MaxInt32` amount → fail-closed-consume (no ordinary talking-chat fallthrough, no frames, no mutation);
   - insufficient carried gold → fail-closed-consume with no frames / no mutation;
   - durable warehouse money overflow past `MaxInt32` → fail-closed-consume with no frames / no mutation;
   - success → deduct carried gold, add that amount to durable warehouse money (preserving password + cells), persist account inventory/gold snapshot together with the safebox write, and emit self-only gold `PLAYER_POINT_CHANGE` plus `GC::SAFEBOX_MONEY_CHANGE` for the new warehouse total; write failure rolls back live gold and durable money fail-closed with no frames.
4. Local slash withdraw `/safebox_money_withdraw <amount>` under the same open-presentation / alive / GAME guards:
   - missing / non-positive / non-uint32-parseable / over-`MaxInt32` amount → fail-closed-consume;
   - insufficient durable warehouse money → fail-closed-consume;
   - carried gold overflow past the already-owned exchange gold carrier max (`1<<31-1`) → fail-closed-consume (reuse the existing gold-overflow safety bound; do not invent a new chat string in this bootstrap slice);
   - success → subtract warehouse money, credit carried gold, persist both stores, emit self-only gold `PLAYER_POINT_CHANGE` plus `GC::SAFEBOX_MONEY_CHANGE` for the new warehouse total; write failure rolls back fail-closed with no frames.
5. Closed presentation, pending-only password challenge, death-floor, and unrecognized extra args stay fail-closed-consume for both slashes (no ordinary talking-chat fallthrough).
6. Spec/QA name these slashes beside the owned open/password seams; do **not** invent a TMP4 CG `SAFEBOX_MONEY` request header in this bootstrap slice (oracle DB round-trip / mall money stay out of scope).
7. Mall / client change-password packets / reopen cooldown / distance gates stay deferred.

## Locale / wording note

This bootstrap freeze intentionally keeps deposit/withdraw rejects silent/no-frame (matching the owned closed-presentation storage mutation style) rather than inventing new info-chat strings. Open-burst `SAFEBOX_MONEY_CHANGE` is the only new client-visible status frame.

## What this is not yet

- TMP4 CG `SAFEBOX_MONEY` request packet / header ownership
- mall money / mall open-checkout mutation
- client `SAFEBOX_CHANGE_PASSWORD` / DB answer frames
- partner-side open player-shop / cube exchange busy rejects
- occupied-wear swap/replace
- authored reject-chat text for insufficient gold / warehouse overflow

## TDD shape after the freeze lands

1. Catalog/store: round-trip optional `money`; reject negative / over-`MaxInt32`; prove `ReplaceCharacterMoney` preserves password/cells and cell/password helpers preserve money; deterministic JSON omits zero money.
2. Runtime unit / session: open burst emits `SAFEBOX_SIZE` (+ SET) + `SAFEBOX_MONEY_CHANGE`; closed/pending paths emit no money frame; save/withdraw success mutate gold + durable money and emit both refresh frames; insufficient / overflow / closed / death-floor stay fail-closed.
3. Minimal session: deposit → reconnect/restart reopen rematerializes the same warehouse money via open-burst `SAFEBOX_MONEY_CHANGE`; withdraw restores carried gold; second character on the same account does not see the first character's warehouse money.

## Status

Implemented on `lane/items`: durable optional `money`, open-burst `SAFEBOX_MONEY_CHANGE`, and `/safebox_money_save` / `/safebox_money_withdraw` with fail-closed guards. Mall / TMP4 CG `SAFEBOX_MONEY` / client change-password packets stay deferred.
