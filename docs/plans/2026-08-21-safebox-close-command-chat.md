# Safebox Close Command-Chat Companion — 2026-08-21

## Objective

Emit the legacy client-visible `CHAT_TYPE_COMMAND` / `CloseSafebox` companion when the bootstrap safebox presentation is closed, so TMP4-compatible clients hide the safebox window instead of leaving a stale UI after `/close_safebox` or the client `/safebox_close` slash.

## Contract owned by this slice

1. While the selected character already has the bootstrap `/open_safebox` presentation open, `/close_safebox` and `/safebox_close` clear that same-socket open flag and return exactly one self-only `GC::CHAT` / `CHAT_TYPE_COMMAND` with message `CloseSafebox`.
2. Remembered same-session in-memory safebox contents stay untouched; a later same-session reopen may still re-emit `SAFEBOX_SIZE` plus remembered `SAFEBOX_SET` rows.
3. When the presentation is already closed, both slash forms stay fail-closed-consume: accepted with no frames, no ordinary talking-chat fallthrough, and no inventory/gold/persistence mutation.
4. Spec/QA name this as presentation-shell hygiene only: no password/load, no durable safebox persistence, no money/mall, no `SAFEBOX_WRONG_PASSWORD`.

## What this is not yet

- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money / mall
- partner-side open player-shop / cube busy-window exchange rejects
- automatic close on death / transfer beyond already-owned leave/lifecycle clear of the open flag

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'CloseSafebox|OpenSafebox|SafeboxCheckin|SafeboxItemMove|RefineConfirmBusySafebox' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep money / password / durable persistence deferred.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
