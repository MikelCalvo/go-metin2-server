# Safebox Reopen Cooldown / Distance-From-Open-Anchor Contract Freeze — 2026-08-23

## Objective

Freeze the first warehouse reopen cooldown and distance-from-open-anchor gates before opening RED, so password-gated warehouse open can match the oracle reopen/walk-away policy without inventing chat strings, lab-harness exemptions, or which close paths arm the cooldown mid-implementation.

## Why docs-first

Warehouse password challenge + durable money are owned, but a player can still close the warehouse and immediately re-challenge / re-open, or walk arbitrarily far from the challenge position and still submit `/safebox_password`. The external behavior oracle gates `ReqSafeboxLoad` with a 10-second post-close pulse window and `GetDistanceFromSafeboxOpen() > 1000` (same `DISTANCE_APPROX` / `1000` bound already owned for exchange), while `SetSafeboxOpenPosition()` records the challenge-time XY. Opening RED without freezing lab-harness exemption, which paths arm the cooldown, and English reject chat would invent policy. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Open-anchor: successful warehouse `open_safebox` `INTERACT` that arms a pending `ShowMeSafeboxPassword` challenge also remembers the selected character's current live `X`/`Y` as the same-socket open anchor. Pending clear without open (wrong / malformed password, teardown) does **not** clear that anchor; a later fresh challenge overwrites it.
2. Distance gate: `/safebox_password <pwd>` that would otherwise proceed past already-open / pending / password-shape checks must fail closed when a remembered open anchor exists and `ApproxDistance(currentXY, openAnchorXY) > 1000` (reuse the owned exchange distance helper / bound). Emit self-only `CHAT_TYPE_INFO` `You are too far from the warehouse to open it.`, leave pending challenge intact so a closer retry can succeed, and perform no open / no durable mutation.
3. Reopen cooldown: every path that clears an **open** presentation with `CloseSafebox` (slash `/close_safebox` / `/safebox_close`, practice-mob floor, transfer/warp rebootstrap, `/phase_select` / `/quit` / `/logout`, and the shared prepend/append close helpers) arms a same-socket 10-second reopen cooldown from session `now`. `/safebox_password` that would otherwise open must fail closed while `now < closedAt + 10s` with self-only `CHAT_TYPE_INFO` `You cannot open the warehouse again so soon after closing it.`, leave pending intact, and perform no open. After the window elapses, the same pending password may open normally.
4. Gate ordering on `/safebox_password`: death-floor / no-selected → already-open chat → no-pending consume → cooldown reject chat → distance reject chat → malformed/wrong-password paths → success open. Cooldown/distance do **not** clear pending.
5. Lab slash `/open_safebox [1..3]` stays exempt: it never consults cooldown or open-anchor distance (docs name this explicitly so existing mutation proofs remain usable).
6. Warehouse `INTERACT` that only arms the password challenge does **not** itself reject on cooldown/distance; those gates run on `/safebox_password` only (matching oracle `SetSafeboxOpenPosition` vs `ReqSafeboxLoad`).
7. Spec/QA name these gates beside the owned password challenge; mall / TMP4 CG `SAFEBOX_MONEY` / client `SAFEBOX_CHANGE_PASSWORD` / player-shop/cube busy rejects stay deferred.

## Locale / wording note

Oracle uses `LC_TEXT("<창고> 창고를 닫은지 10초 안에는 열 수 없습니다.")` and `LC_TEXT("<창고> 거리가 멀어서 창고를 열 수 없습니다.")`. This bootstrap freeze uses project-owned English `You cannot open the warehouse again so soon after closing it.` and `You are too far from the warehouse to open it.`

## What this is not yet

- mall open/checkout / `MALL_*` runtime emission
- TMP4 CG `SAFEBOX_MONEY` request header
- client `SAFEBOX_CHANGE_PASSWORD` / DB answer frames
- partner-side open player-shop / cube exchange busy rejects
- auto-close of an already-open warehouse when the player walks beyond the anchor (oracle load gate only; walk-away close stays deferred)

## TDD shape after the freeze lands

1. Runtime/session: warehouse challenge → open → close → immediate re-challenge + `/safebox_password` emits cooldown info chat and stays closed; after advancing session clock past 10s the same password opens.
2. Runtime/session: warehouse challenge records anchor → walk so `ApproxDistance > 1000` → `/safebox_password` emits too-far info chat with pending preserved → walk back inside gate → password opens.
3. Negative: lab `/open_safebox` still opens during cooldown and far from any prior anchor; `/safebox_change_password` remains unaffected.

## Status

Implemented on `lane/items`: `/safebox_password` honors the 10s post-close reopen cooldown and `ApproxDistance > 1000` open-anchor distance gate with the frozen info-chat strings; lab `/open_safebox` stays exempt; mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.
