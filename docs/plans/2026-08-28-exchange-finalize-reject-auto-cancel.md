# Exchange finalize reject auto-cancel — 2026-08-28

## Objective

Close the remaining oracle gap on mutual-accept finalization rejects: after the
already-owned dual-sided Check / Space / gold-overflow / Other info-chat, tear
down the bootstrap exchange shell with self + peer `GC::EXCHANGE END` instead of
leaving it cancellable. Matches external `CExchange::Accept` `goto EXCHANGE_END`
→ `Cancel()` without inventing new result subheaders, per-reason Other strings,
or auto-cancel on busy / gold-carrier / `LESS_GOLD` / persistence-failure paths.

## Why this exists

Bootstrap already owns client-visible dual-sided finalize reject chats and keeps
the shell open so QA can cancel manually. The TMP4 oracle always `Cancel()`s
after a failed mutual-accept check path, so both clients close the trade window
immediately. Manual QA therefore sees a stuck open shell after Space / Check /
gold-overflow / Unknown-error rejects that the oracle would have closed. Track C
priority #2 (staged EXCHANGE fail-closed finalization) prefers this cancel-on-
failure companion over inventing MYSHOP pricelist / quest-running / bag-missing
INFO / refine keep-grade.

## Contract to freeze (before RED)

1. **Scope**: only mutual-accept finalize reject classes that already emit owned
   dual-sided (or dual same) info-chat:
   - Check-shaped displayed item/gold drift
   - Space (receiver inventory capacity)
   - gold-overflow
   - Other (`Unknown error`)
   Applies on second-accept (`AcceptExchange` when the partner had already
   accepted) and on `CommitExchangeFinalize` revalidation of those same classes.
2. **Frame ordering**: emit the owned reject info-chat first, then close:
   - reject requester / commit requester self: `[info-chat…]` then one
     `GC::EXCHANGE END`
   - paired peer queued: `[info-chat…]` then one `GC::EXCHANGE END`
3. **Shell teardown**: clear in-memory pairing / display / accept / gold display
   state with no inventory / equipment / quickslot / gold / ground / trade
   mutation from the cancel itself (same cancel semantics as ordinary
   `CANCEL` / death / walk-away teardown).
4. **Still leave shell open** (explicit non-goals for this slice):
   - first-side `ACCEPT` busy / gold-carrier / silent stale-item rejects
   - second-accept / commit-time busy-window and gold-carrier-cap rejects
   - requester `LESS_GOLD` accept-time displayed-gold exception
   - mutual-accept persistence-failure silent fail-closed
   - display-lock / ordinary cancel / death / walk-away / lifecycle teardowns
5. Spec/QA/roadmap name Cancel-on-failure beside the owned finalize reject chats
   once GREEN; until then this freeze is the source of truth for the next RED.
6. Do **not** invent new `GC::EXCHANGE` result subheaders, per-reason Other
   strings beyond `Unknown error`, authored overrides, or auto-cancel on the
   still-open paths listed above.

## Locale / wording note

No new English reject string. Reuse the already-owned Check / Space /
gold-overflow / Other chats and the ordinary `GC::EXCHANGE END` companion. Do
not copy oracle source comments or Korean keys into runtime code.

## Explicit non-goals

- auto-cancel on busy-window / gold-carrier-cap / `LESS_GOLD` / persist-fail
- first-side accept marker rejects
- distinct per-reason Other chat strings
- optional authored/template-backed overrides
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers
- MYSHOP pricelist / quest-running / bag-missing INFO

## Proof shape

1. Runtime/session: second-accept Space / Check / gold-overflow / Other reject
   emits owned dual-sided info-chat then self/peer `END`; shell gone (later
   `CANCEL` fails closed); no trade mutation / persistence change.
2. Shared-world commit: post-plan Check/Space/gold-overflow/Other drift emits the
   same chat-then-`END` teardown and rolls back any already-written finalize
   snapshots (already owned); shell cleared.
3. Negative regression: busy / gold-carrier / `LESS_GOLD` still
   leave the shell cancellable as already owned for this slice's
   non-goals. Persist-fail Cancel-on-failure is owned separately
   (`docs/plans/2026-08-28-exchange-persist-fail-reject-auto-cancel.md`).

## Status

GREEN on `lane/items`: second-accept and commit-time Check/Space/gold-overflow/Other
rejects emit owned dual-sided info-chat then self/peer `GC::EXCHANGE END` and clear
the shell; busy / gold-carrier / `LESS_GOLD` paths still leave the shell cancellable
in this plan's original non-goals (busy/gold-carrier Cancel-on-failure and
persist-fail Cancel-on-failure are owned by companion plans).
