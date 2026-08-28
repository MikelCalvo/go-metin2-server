# Exchange busy / gold-carrier reject auto-cancel — 2026-08-28

## Objective

Extend oracle Cancel-on-failure to the remaining mutual-accept reject chats that
still leave the bootstrap exchange shell cancellable after
`docs/plans/2026-08-28-exchange-finalize-reject-auto-cancel.md`: second-accept
and commit-time **busy-window** and **gold-carrier-cap** rejects should emit the
already-owned self-only info-chat, then tear down with self + peer
`GC::EXCHANGE END`.

## Why this exists

Check/Space/gold-overflow/Other mutual-accept rejects already auto-cancel.
Busy-window and gold-carrier-cap rejects still keep the shell open after their
owned self-only chats, so QA sees a stuck trade window the TMP4 oracle would
have closed via `goto EXCHANGE_END` → `Cancel()`. This is the narrow companion
that finishes Cancel-on-failure for every owned mutual-accept reject chat class
without inventing new subheaders, strings, or auto-cancel on `LESS_GOLD` /
persist-fail / first-side silent accepts.

## Contract to freeze (before RED)

1. **Scope**: second-accept (`AcceptExchange` when the partner had already
   accepted, and also first-side accept busy/gold-carrier rejects that already
   emit the owned self-only chat) plus `CommitExchangeFinalize` busy-window and
   gold-carrier-cap drift.
2. **Frame ordering**: owned self-only busy or gold-carrier info-chat first,
   then:
   - reject / commit requester self: one trailing `GC::EXCHANGE END`
   - paired peer: one queued `GC::EXCHANGE END`
3. **Shell teardown**: clear pairing / display / accept / gold display with no
   inventory / equipment / quickslot / gold / ground / trade mutation from the
   cancel itself (ordinary cancel semantics).
4. **Still leave shell open** (explicit non-goals):
   - requester `LESS_GOLD` accept-time displayed-gold exception
   - mutual-accept persistence-failure silent fail-closed
   - first-side silent stale-item / no-frame accept rejects
   - Check/Space/gold-overflow/Other (already auto-cancel GREEN)
5. Spec/QA/roadmap rename busy / gold-carrier reject paths from
   "shell stays cancellable" to chat-then-`END` once GREEN; until then this
   freeze is the source of truth for the next RED.
6. Do **not** invent new `GC::EXCHANGE` result subheaders, new English strings,
   authored overrides, MYSHOP pricelist / quest-running / bag-missing INFO,
   refine keep-grade/catalyst, mall, OR-materials, or binary cube headers.

## Locale / wording note

No new English reject string. Reuse the already-owned busy and gold-carrier
chats plus ordinary `GC::EXCHANGE END`. Do not copy oracle source comments or
Korean keys into runtime code.

## Explicit non-goals

- auto-cancel on `LESS_GOLD`
- auto-cancel on persistence-failure silent fail-closed
- first-side silent/no-frame accept rejects
- new result subheaders or strings
- MYSHOP_PRICELIST / quest-running / bag-missing INFO
- refine keep-grade / catalyst; mall; OR-materials; binary cube headers

## Proof shape

1. Runtime/session: open exchange + requester/partner busy (merchant / safebox /
   refine / MYSHOP / cube) on `ACCEPT` / second accept → owned busy chat then
   self/peer `END`; later `CANCEL` fails closed; no trade mutation.
2. Runtime/session: gold-carrier-cap on `ACCEPT` / second accept → owned
   gold-carrier chat then self/peer `END`; shell cleared.
3. Shared-world commit: post-plan busy or gold-carrier drift → same
   chat-then-`END` teardown + owned rollbacks; shell cleared.
4. Negative: `LESS_GOLD` still leaves the shell cancellable;
   Check/Space/gold-overflow/Other remain chat-then-`END` (regression).
   Persist-fail Cancel-on-failure is owned separately
   (`docs/plans/2026-08-28-exchange-persist-fail-reject-auto-cancel.md`).

## Status

GREEN on `lane/items`: ACCEPT and commit-time busy-window / gold-carrier-cap
rejects emit owned self-only info-chat then self/peer `GC::EXCHANGE END` and clear
the shell; `LESS_GOLD` still leaves the shell cancellable. Persist-fail
Cancel-on-failure is owned by
`docs/plans/2026-08-28-exchange-persist-fail-reject-auto-cancel.md`.
