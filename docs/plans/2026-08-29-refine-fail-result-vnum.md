# Refine fail-result downgrade (`refine_info.fail_result_vnum`) — 2026-08-29

## Objective

Freeze the next template-authored refine failure companion after
`keep_on_fail`: when a remembered `probability` in `1..99` injected roll fails
and the template authors a non-zero `fail_result_vnum` (with `keep_on_fail`
omitted/`false`), consume gold/materials, replace the source carried `vnum`
with `fail_result_vnum` in the same cell (preserving instance id), and emit
`CHAT_TYPE_COMMAND` `RefineFailed <type>` — without inventing scroll/catalyst
consumption, peer-facing refine notifications, or GD/DB substrate.

## Why this exists

Oracle scroll refine failures can either destroy (`bDestroyWhenFail`), keep the
source (no fail result), or replace the source with `result_fail_vnum` when a
fail result is authored. Bootstrap already owns destroy-on-fail and
template-authored `keep_on_fail`. The remaining client-visible template seam is
authored downgrade via `fail_result_vnum`.

## Contract to freeze (before RED)

1. **Store field**: optional `fail_result_vnum` uint32 on `refine_info`.
   - omitted / `0` → failed `1..99` rolls keep the already-owned destroy path
     when `keep_on_fail` is also omitted/`false`
   - non-zero is valid only when `refineable`, `probability` is in `1..99`, and
     `keep_on_fail` is omitted/`false`
   - `fail_result_vnum` equal to the source template `vnum` or to
     `result_vnum` fails closed at the item-template store boundary
   - `fail_result_vnum` with `probability` `0` or `100`, or together with
     `keep_on_fail = true`, fails closed at the item-template store boundary
2. **Preview**: unchanged; `REFINE_INFORMATION_NEW` does not surface the fail
   result (wire table has no fail-result field). Dialog remembers the full
   `refine_info` snapshot including `fail_result_vnum`.
3. **Confirm (`probability` in `1..99`)**:
   - `roll <= probability` → owned success burst / `RefineSuceeded <type>`
   - `roll > probability` and `keep_on_fail == true` → owned keep-failure burst
     (`docs/plans/2026-08-29-refine-keep-on-fail.md`)
   - `roll > probability`, `keep_on_fail == false`, and `fail_result_vnum != 0`
     → downgrade-failure burst:
     material `ITEM_UPDATE`/`ITEM_DEL` (+ material-removal `QUICKSLOT_DEL`),
     result-cell `ITEM_SET` with `fail_result_vnum` preserving instance id /
     count / cell, gold `PLAYER_POINT_CHANGE`, then `CHAT_TYPE_COMMAND`
     `RefineFailed <type>`; persist inventory/gold/quickslots after mutation
   - `roll > probability`, `keep_on_fail == false`, and `fail_result_vnum == 0`
     → owned destroy burst / `RefineFailed <type>` (unchanged)
4. **Result template guard**: accepted downgrade requires
   `fail_result_vnum` to resolve to a valid loaded item template (same style as
   success `result_vnum` resolution). Missing/invalid fail-result template fails
   closed with no frames/mutation and leaves the open dialog untouched.
5. **Deterministic paths**: `probability = 0` stays destroy-only;
   `probability = 100` stays success-only; neither reads `fail_result_vnum` at
   runtime (store already rejects non-zero on those probabilities).
6. Spec/QA/packet-matrix/roadmap name this beside owned refine confirm once
   GREEN. Do **not** invent catalysts, guild/money-only, peer refine frames, or
   SQL `fail_result_vnum` columns in the first GREEN (file-backed JSON
   round-trip is enough; SQL import continues to default the field to `0`).

## Locale / wording note

Reuse already-owned `RefineFailed <type>` command chat. No new English INFO
string. Do not copy oracle source comments or Korean keys into runtime code.

## Explicit non-goals

- scroll / hyuniron / musin / black-dragon catalyst consumption
- guild / money-only refine
- peer-facing refine notifications
- SQL migration column / import for `fail_result_vnum`
- changing `keep_on_fail` semantics already owned on `lane/items`

## Proof shape (for the later GREEN)

1. Catalog/store: round-trip `fail_result_vnum` with `probability` in `1..99`;
   reject coexistence with `keep_on_fail`, reject `0`/`100` probability pairings,
   reject equal-to-source / equal-to-result; deterministic JSON.
2. Player unit: `ApplyRefineWithRoll` fail + authored `fail_result_vnum`
   consumes gold/materials, replaces live source `vnum`, leaves persisted
   snapshot untouched until commit boundary.
3. Session: preview + queued fail roll → downgrade burst (`RefineFailed`,
   source cell shows fail-result `vnum`, materials/gold persisted); omit-field
   fail roll still destroys; `keep_on_fail` still wins over `fail_result_vnum`
   because store rejects coexistence.

## Status

Docs/spec freeze only on `lane/items`. RED/GREEN intentionally deferred until
the next items-lane run opens tests against this contract.
