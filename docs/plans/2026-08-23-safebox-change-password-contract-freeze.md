# Safebox Change-Password Contract Freeze — 2026-08-23

## Objective

Freeze the first `/safebox_change_password` durable-password mutation contract before opening RED, so warehouse password challenge ownership can grow into an operator/player-visible password change without inventing chat strings or store semantics mid-implementation.

## Why docs-first

Warehouse open now challenges password and matches durable optional `password` (default `000000`). Players still cannot change that password. Opening RED without freezing chat wording, validation bounds, and whether change requires an open presentation would invent policy. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Local slash `/safebox_change_password <old> <new>` (talking chat) while the selected character is in `GAME` and above the bootstrap zero-HP floor:
   - missing / empty / over-6-char old or new password → self-only `CHAT_TYPE_INFO` `You have entered the wrong password.`, no durable mutation, no ordinary talking-chat fallthrough
   - old password mismatch vs durable effective password → self-only `CHAT_TYPE_INFO` `You have entered the wrong password.`, no durable mutation
   - matching old + valid new → persist durable `password = <new>` for that login + character id (preserving cells), emit self-only `CHAT_TYPE_INFO` `The warehouse password has been changed.`, leave open/pending presentation state unchanged
2. Blank stored password still means effective default `000000` for the old-password check (same as open challenge).
3. Change-password does **not** require an open safebox presentation and does **not** open one. Pending `ShowMeSafeboxPassword` challenge, if any, is left untouched.
4. Spec/QA name this slash beside the owned warehouse password challenge; do not invent `SAFEBOX_CHANGE_PASSWORD` client/server packets in this bootstrap slice (oracle DB round-trip stays out of scope).
5. Money / mall / reopen cooldown / distance gates stay deferred.

## Locale / wording note

Oracle uses `LC_TEXT("<창고> 잘못된 암호를 입력하셨습니다.")` for malformed / mismatch and `LC_TEXT("<창고> 창고 비밀번호가 변경되었습니다.")` for success. This bootstrap freeze uses project-owned English `You have entered the wrong password.` (reuse) and `The warehouse password has been changed.`

## What this is not yet

- client packet `SAFEBOX_CHANGE_PASSWORD` / DB answer frames
- mall password change
- money / `SAFEBOX_MONEY_CHANGE`
- partner-side open player-shop / cube exchange busy rejects
- occupied-wear swap/replace

## TDD shape after the freeze lands

1. Catalog/store: `ReplaceCharacterPassword` already exists; prove change-password path persists new password and preserves cells / rejects invalid new password.
2. Runtime unit / session: malformed reject chat; old mismatch reject chat; success chat + durable rematch on later `/safebox_password`; open/pending state unchanged.
3. Minimal session: warehouse challenge → wrong old change fails → successful change → later open requires the new password.

## Status

Shipped on `lane/items`: `/safebox_change_password` persists durable password with success/wrong-password info chat; money/mall and client `SAFEBOX_CHANGE_PASSWORD` packets stay deferred.
