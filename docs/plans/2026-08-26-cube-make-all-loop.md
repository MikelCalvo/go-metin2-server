# Cube `/cube make all` loop — 2026-08-26

## Objective

Own the oracle's `/cube make all` companion so TMP4 clients can repeat the
already-owned one-attempt `/cube make` path while materials, gold, and
inventory capacity still allow a successful craft, stopping on the first
non-success outcome.

## Why now

- `/cube make` already owns deterministic `percent = 100` and injected-roll
  `percent` in `1..99` (success → `cube success`; fail roll → info + `cube fail`).
- Spec/QA still name `/cube make all` as silent recognized consume.
- Oracle `do_cube` (`cmd_general.cpp`) runs `while (true == Cube_make(ch))`
  whenever a non-empty second arg is present; `Cube_make` returns `true` only
  after a successful reward grant and `false` for unmatched materials,
  insufficient gold, closed cube, or a failed percent roll (after materials/gold
  were already consumed on the fail-roll path).

## Contract frozen by this slice

1. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen` with remembered `activeCubeNPCVnum`:
   - talking-chat `/cube make all` (exact three fields: `cube make all`)
   - other `/cube make …` extra args stay silent recognized consume
   - closed / death-floor / no-selected stay silent consume (unchanged)
2. **Loop policy** (oracle-shaped):
   - repeatedly run the already-owned one-attempt make path
   - on **craft success** (`percent = 100` or injected `roll <= percent`): append
     that attempt's owned success burst (including follow-up `cube info`) and
     continue while bindings still exact-match a craftable recipe under gold /
     placement guards
   - on **craft fail roll** (`roll > percent`): append that attempt's owned fail
     burst and **stop** (do not continue after a failed roll)
   - on **pre-mutation reject** (unmatched materials / insufficient gold /
     inventory-full / invalid reward template / out-of-range percent or roll):
     append that attempt's owned reject frames (or stay silent when the one-shot
     path is silent) and **stop**
3. **Material match** for make / make-all / `cube info` gold resolution uses
   oracle-shaped coverage (`boundCount[vnum] >= needCount[vnum]` for every
   required material vnum). Surplus bound counts and extra bound vnums are
   allowed; consume still subtracts only the recipe need counts.
4. **Persistence**: each successful or fail-roll attempt that mutates state
   commits through the same inventory/gold/quickslot persistence boundary as
   one-shot `/cube make` before the next loop iteration.
5. **Safety**: hard-cap loop iterations so a pathological recipe cannot spin
   forever (bootstrap bound: `inventory.CarriedInventorySlotCount`; stop silently
   if the cap is hit after the last appended attempt).
6. Spec/QA/packet-matrix/roadmap name `/cube make all` beside the owned one-shot
   make vertical; `cube list` / `cancel` and store-level `percent = 0` stay
   deferred.

## Explicit non-goals

- `cube list` INFO dump / `cube cancel`
- store-validated `percent = 0` always-fail recipes
- complicated OR-material matching
- durable cube-slot persistence
- binary cube packet headers
- inventing new locale strings beyond the owned make reject/fail chats

## Proof shape

1. Runtime/session success loop: bind enough materials for two `percent = 100`
   crafts → `/cube make all` emits two concatenated success bursts, persists two
   rewards / emptied materials / gold debit ×2, and stops when bindings no longer
   match (final unmatched info chat after the second success).
2. Runtime/session fail-roll stop: `percent = 75`, queue roll `76` → `/cube make
   all` emits one fail burst and does not continue.
3. Negatives: closed `/cube make all` stays silent; open `/cube make extra` stays
   silent; open matched + insufficient gold `/cube make all` emits the owned Yang
   info chat once with no mutation.

## Status

Implemented on `lane/items` together with this freeze. Store-level `percent = 0`
always-fail is owned in `docs/plans/2026-08-26-cube-make-percent-0-always-fail.md`.
Next Track C cube seam candidates: `cube list` / `cancel`.
