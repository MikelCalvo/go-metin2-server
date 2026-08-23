# Safebox Password Challenge / SAFEBOX_WRONG_PASSWORD — 2026-08-23

## Objective

Own the first client-visible safebox password challenge so warehouse `open_safebox` no longer auto-opens the presentation, and a matching `/safebox_password` either opens with durable rematerialize or emits header-only `GC::SAFEBOX_WRONG_PASSWORD`.

## Why this slice

Durable same-account safebox cells rematerialize on open, but the warehouse NPC still skips the TMP4 password dialog and opens immediately. Manual QA cannot exercise wrong-password feedback or the password-gated open path. The slash `/open_safebox` harness stays as the no-password lab/debug opener so existing mutation proofs remain usable.

## Contract owned by this slice

1. Durable `safeboxstore.CharacterRow` may author optional `password` (omit / empty means bootstrap default `000000`). Round-trip / fail-closed validation / deterministic JSON keep the existing cell contract; malformed passwords fail closed.
2. Authored warehouse `open_safebox` `INTERACT` success no longer sets the open/busy presentation or emits `SAFEBOX_SIZE` / `SAFEBOX_SET`. It remembers a same-socket pending password challenge with the authored effective size, optionally emits authored info chat, then emits self-only `CHAT_TYPE_COMMAND` `ShowMeSafeboxPassword`. Merchant auto-close still prepends `GC::SHOP END` before those frames when a merchant window is open. Pending challenge does **not** make the socket busy for exchange `START`.
3. `/safebox_password <pwd>` (talking chat) while a pending challenge exists:
   - empty / missing / over-6-char password → self-only `CHAT_TYPE_INFO` `You have entered the wrong password.`, consume slash, clear pending, no open, no `SAFEBOX_WRONG_PASSWORD`
   - already-open presentation → self-only `CHAT_TYPE_INFO` `The warehouse is already open.`, consume slash, no mutation
   - password mismatch vs durable effective password → emit header-only `GC::SAFEBOX_WRONG_PASSWORD`, clear pending, leave presentation closed
   - password match → clear pending, open presentation with the remembered size, hydrate durable cells, emit `SAFEBOX_SIZE` + in-range `SAFEBOX_SET` (same as today's open seam), set exchange busy flag
4. Local slash `/open_safebox [1..3]` remains the no-password lab harness: it still opens immediately without challenging password (docs name this explicitly).
5. Leave / floor / transfer / phase_select / quit / logout clear any pending password challenge together with open presentation cleanup. Money / mall / change-password / SQL import stay deferred.

## Locale / wording note

Oracle uses `LC_TEXT("<창고> 잘못된 암호를 입력하셨습니다.")` / `LC_TEXT("<창고> 창고가 이미 열려있습니다.")` for malformed / already-open chat, and header-only `SAFEBOX_WRONG_PASSWORD` for durable mismatch. This bootstrap slice freezes project-owned English `You have entered the wrong password.` / `The warehouse is already open.` rather than inventing a locale table.

## What this is not yet

- `/safebox_change_password` / DB change-password answer chat
- safebox money / `SAFEBOX_MONEY_CHANGE`
- mall password / `MALL_OPEN`
- 10-second reopen cooldown / distance-from-warehouse gate
- partner-side open player-shop / cube exchange busy rejects
- occupied-wear swap/replace

## TDD and validation

Focused coverage:

- `go test ./internal/safeboxstore -run 'Password|RoundTrip|Deterministic|RejectsInvalid' -count=1`
- `go test ./internal/minimal -run 'SafeboxPassword|OpenSafeboxInteraction|InteractingWithOpenSafeboxActor' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Freeze then implement `/safebox_change_password`.
2. Keep money / mall deferred.
3. Optional later: reopen cooldown / warehouse distance gate when QA needs them.

## Status

Shipped on `lane/items`: warehouse password challenge + `/safebox_password` + durable optional password; slash `/open_safebox` stays no-password lab harness; money/mall/change-password stay deferred.
