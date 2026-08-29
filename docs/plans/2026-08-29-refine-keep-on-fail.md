# Refine keep-on-fail (`refine_info.keep_on_fail`) — 2026-08-29

## Objective

Own the first template-authored keep-grade failure companion for remembered
`refine_info.probability` in `1..99`: when `keep_on_fail` is authored and the
injected roll fails, consume gold/materials, leave the source carried item in
place, and emit `CHAT_TYPE_COMMAND` `RefineFailed <type>` — without inventing
scroll/catalyst consumption, downgrade `fail_result_vnum`, or peer-facing refine
notifications.

## Why this exists

Track C's live next line prefers evidence-backed refine keep-grade after
exchange Cancel-on-failure and MYSHOP bag companions. Oracle scroll failures can
keep the source (non-destroy path) while still charging cost/materials and
signaling `RefineFailed`. Bootstrap already owns destroy-on-fail for `1..99`
rolls; this slice makes the keep outcome template-authored and client-visible.

## Contract to freeze (before / with GREEN)

1. **Store field**: optional `keep_on_fail` bool on `refine_info`.
   - omitted / `false` → failed `1..99` rolls keep the already-owned whole-source
     destroy path
   - `true` is valid only when `refineable` and `probability` is in `1..99`
   - `keep_on_fail` with `probability` `0` or `100` fails closed at the
     item-template store boundary
2. **Preview**: unchanged; `REFINE_INFORMATION_NEW` does not surface the flag
   (wire table has no keep bit). Dialog still remembers the full `refine_info`
   snapshot including `keep_on_fail`.
3. **Confirm (`probability` in `1..99`)**:
   - `roll <= probability` → owned success burst / `RefineSuceeded <type>`
   - `roll > probability` and `keep_on_fail == false` → owned destroy burst /
     `RefineFailed <type>` (unchanged)
   - `roll > probability` and `keep_on_fail == true` → keep-failure burst:
     material `ITEM_UPDATE`/`ITEM_DEL` (+ material-removal `QUICKSLOT_DEL`),
     gold `PLAYER_POINT_CHANGE`, then `CHAT_TYPE_COMMAND` `RefineFailed <type>`;
     **no** source `ITEM_DEL` / source quickslot clear; source identity/vnum/count
     remain; persist inventory/gold/quickslots after material/gold mutation
4. **Deterministic paths**: `probability = 0` stays destroy-only;
   `probability = 100` stays success-only; neither reads `keep_on_fail` at
   runtime (store already rejects `true` on those probabilities).
5. Spec/QA/packet-matrix/roadmap name this beside owned refine confirm. Do **not**
   invent catalysts, downgrade `fail_result_vnum`, or guild/money-only in this
   slice (file-backed JSON round-trip is enough for the items-lane GREEN). SQL
   migration column / import for `keep_on_fail` is now Done on `lane/persistence`
   via additive `0021` — see [item-template refine keep-on-fail migration](2026-08-29-item-template-refine-keep-on-fail-migration.md).

## Locale / wording note

Reuse already-owned `RefineFailed <type>` command chat. No new English INFO
string. Do not copy oracle source comments or Korean keys into runtime code.

## Explicit non-goals

- scroll / hyuniron / musin / black-dragon catalyst consumption
- downgrade / `fail_result_vnum` / safe-refine variants
- guild / money-only refine
- peer-facing refine notifications
- ~~SQL migration column / import for `keep_on_fail`~~ Done on `lane/persistence`
  via additive `0021` — see [item-template refine keep-on-fail migration](2026-08-29-item-template-refine-keep-on-fail-migration.md); `fail_result_vnum` SQL stays deferred
- GD/DB `MYSHOP_PRICELIST` / quest-running / bag-missing INFO

## Proof shape

1. Catalog/store: round-trip `keep_on_fail: true` with `probability` in `1..99`;
   reject `keep_on_fail` with `probability` `0` / `100`; deterministic JSON.
2. Player unit: `ApplyRefineWithRoll` fail + `KeepOnFail` consumes gold/materials,
   keeps source live, leaves persisted snapshot untouched until commit boundary.
3. Session: preview + queued fail roll → keep burst (`RefineFailed`, source
   still present, materials/gold persisted); omit-flag fail roll still destroys.

## Status

GREEN on `lane/items`: template-authored `refine_info.keep_on_fail` owns the
keep-on-fail `1..99` roll outcome. SQL migration column / import for
`keep_on_fail` is now Done on `lane/persistence` via additive `0021` — see
[item-template refine keep-on-fail migration](2026-08-29-item-template-refine-keep-on-fail-migration.md).
Catalysts / downgrade / `fail_result_vnum` SQL stay deferred.
