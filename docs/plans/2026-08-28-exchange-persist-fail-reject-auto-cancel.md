# Exchange persist-fail reject chat + auto-cancel — 2026-08-28

## Objective

Close the remaining oracle Cancel-on-failure gap on mutual-accept finalize when
account persistence fails: emit dual-sided `CHAT_TYPE_INFO` `Unknown error`
(same Other wording), then tear down with self + peer `GC::EXCHANGE END`,
instead of silent fail-closed that leaves the shell cancellable.

## Why this exists

Check/Space/gold-overflow/Other and busy/gold-carrier mutual-accept rejects
already chat-then-`END`. Origin-save and partner-save failures during
`applyExchangeFinalize` still emit no frames and leave the trade window open for
manual cancel. The TMP4 oracle treats DB-cache dead as dual `Unknown error` →
`goto EXCHANGE_END` → `Cancel()`. Track C priority #2 prefers finishing
Cancel-on-failure for this persist class without inventing `LESS_GOLD`
auto-cancel, new subheaders, or MYSHOP/refine follow-ons.

## Contract to freeze (before RED)

1. **Scope**: mutual-accept finalize when either account persistence write fails
   (origin save or partner save, including the already-owned partner-save
   rollback of an already-written origin snapshot) inside `applyExchangeFinalize`
   after a second-accept finalize plan was built.
2. **Frame ordering**: dual-sided `CHAT_TYPE_INFO` `Unknown error` (`vid = 0`,
   same string as Other), then:
   - second accepter / finalize apply caller self: one trailing `GC::EXCHANGE END`
   - paired peer: one queued `GC::EXCHANGE END`
3. **Shell teardown**: clear pairing / display / accept / gold display with no
   inventory / equipment / quickslot / gold / ground / trade mutation from the
   cancel itself; account/live snapshots remain pre-trade (owned rollbacks stay
   in force).
4. **Still leave shell open / silent** (explicit non-goals):
   - requester `LESS_GOLD` accept-time displayed-gold exception
   - first-side silent stale-item / no-frame accept rejects
   - inventing distinct persist-fail English beyond `Unknown error`
   - MYSHOP_PRICELIST / quest-running / bag-missing INFO
   - refine keep-grade / catalyst; mall; OR-materials; binary cube headers
5. Spec/QA/roadmap rename persist-fail from “silent / shell stays cancellable”
   to dual `Unknown error` then self/peer `END` once GREEN.
6. Do **not** invent new `GC::EXCHANGE` result subheaders or authored overrides.

## Locale / wording note

Reuse the already-owned Other string `Unknown error` and ordinary
`GC::EXCHANGE END`. Do not copy oracle source comments or Korean keys into
runtime code.

## Explicit non-goals

- auto-cancel on `LESS_GOLD`
- first-side silent/no-frame accept rejects
- distinct persist-fail chat strings beyond `Unknown error`
- optional authored/template-backed overrides
- MYSHOP_PRICELIST / quest-running / bag-missing INFO
- refine keep-grade / catalyst; mall; OR-materials; binary cube headers

## Proof shape

1. Runtime/session: forced partner-account Save failure on second accept → dual
   `Unknown error` then self/peer `END`; later `CANCEL` fails closed; inventories
   / gold / quickslots unchanged on both sides.
2. Runtime/session: forced origin-account Save failure on second accept → same
   chat-then-`END` teardown + no durable trade mutation.
3. Negative: `LESS_GOLD` still leaves the shell cancellable; Check/Space /
   gold-overflow / Other / busy / gold-carrier remain chat-then-`END`
   (regression).

## Status

GREEN on `lane/items`: origin-save and partner-save failures during mutual-accept
finalize emit dual-sided `CHAT_TYPE_INFO` `Unknown error` then self/peer
`GC::EXCHANGE END` and clear the shell; `LESS_GOLD` still leaves the shell
cancellable.
