# Exchange LESS_GOLD accept-time auto-cancel — 2026-08-29

## Objective

Close the last owned Cancel-on-failure gap on bootstrap exchange accepts: when
active-shell `EXCHANGE ACCEPT` finds the requester's previously displayed gold
is now above live gold, keep emitting self-only `GC::EXCHANGE LESS_GOLD`, then
tear the shell down with self + peer `GC::EXCHANGE END` instead of leaving the
trade window cancellable.

## Why this exists

Busy / gold-carrier / Check / Space / gold-overflow / Other / persist-fail
mutual-accept rejects already chat-then-`END` (or status-then-`END`). The
requester accept-time stale displayed-gold branch still returns only
`LESS_GOLD` and leaves the shell open for manual cancel. That is the remaining
Track C priority-#2 Cancel-on-failure companion after
`docs/plans/2026-08-28-exchange-item-add-instance-sockets.md`, without inventing
GD/DB `MYSHOP_PRICELIST`, quest-running, bag-missing INFO, or refine keep-grade.

## Contract to freeze (before RED)

1. **Scope**: requester accept-time displayed-gold revalidation inside
   `AcceptExchange` when remembered `exchangeGold[origin]` is non-zero and
   greater than the accept requester's current live gold (`availableGold`).
   This covers first-side accept and second-accept paths that hit that branch
   before partner Check / finalization preconditions.
2. **Frame ordering**:
   - accept requester self: one `GC::EXCHANGE LESS_GOLD`, then one trailing
     `GC::EXCHANGE END`
   - paired peer: one queued `GC::EXCHANGE END`
3. **Shell teardown**: clear pairing / display / accept / gold display with no
   inventory / equipment / quickslot / gold / ground / trade mutation from the
   cancel itself (ordinary cancel semantics). Later `CANCEL` fails closed.
4. **Unchanged**:
   - over-budget active-shell `ELK_ADD` / gold-add still emits only self
     `LESS_GOLD` with no peer frame and **does not** auto-cancel the shell
   - first-side silent stale-item / no-frame accept rejects stay silent and
     cancellable
   - partner stale-gold second-accept stays dual-sided Check chat then `END`
     (already owned)
5. Spec/QA/roadmap rename accept-time `LESS_GOLD` from “shell stays cancellable”
   to `LESS_GOLD` then self/peer `END` once GREEN; until then this freeze is the
   source of truth for the next RED.
6. Do **not** invent new `GC::EXCHANGE` result subheaders, new English strings,
   authored overrides, GD/DB `MYSHOP_PRICELIST`, quest-running, bag-missing INFO,
   or refine keep-grade/catalyst.

## Locale / wording note

No new English reject string. Reuse already-owned `LESS_GOLD` plus ordinary
`GC::EXCHANGE END`. Do not copy oracle source comments or Korean keys into
runtime code.

## Explicit non-goals

- auto-cancel on over-budget `ELK_ADD` / gold-add `LESS_GOLD`
- first-side silent/no-frame accept rejects
- new result subheaders or strings
- GD/DB `MYSHOP_PRICELIST` / quest-running / bag-missing INFO
- refine keep-grade / catalyst; mall; OR-materials; binary cube headers

## Proof shape

1. Session: active-shell gold display, then live gold drops below displayed
   amount, then `ACCEPT` → self `LESS_GOLD` + self `END`, peer queued `END`;
   later `CANCEL` fails closed; no trade mutation.
2. Negative: over-budget `ELK_ADD` still emits only self `LESS_GOLD` and leaves
   the shell cancellable; busy / Check / Space / gold-overflow / Other /
   persist-fail remain chat-then-`END` (regression).

## Status

GREEN on `lane/items`: accept-time requester `LESS_GOLD` emits self
`GC::EXCHANGE LESS_GOLD` then self/peer `GC::EXCHANGE END` and clears the shell;
over-budget `ELK_ADD` `LESS_GOLD` still leaves the shell cancellable. GD/DB
`MYSHOP_PRICELIST` / quest-running / bag-missing INFO / refine keep-grade stay
deferred.
