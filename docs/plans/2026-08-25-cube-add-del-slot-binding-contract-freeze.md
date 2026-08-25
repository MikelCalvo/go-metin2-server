# Cube craft-slot `add` / `del` + `cube info` contract freeze — 2026-08-25

## Objective

Freeze the first lab cube craft-slot binding seam before RED so TMP4 clients
can place / remove carried inventory cells into the open cube window and receive
the oracle's `cube info` gold hint without inventing `make` consumption.

## Why this exists

`/cube r_info` → `r_list` and `/cube r_info <index>` → `m_info` are now owned.
The oracle's next command-chat family used by the TMP4 cube UI is craft-slot
binding:

- client → server: `/cube add <cubeIndex> <invenIndex>`
- client → server: `/cube del <cubeIndex>` (TMP4 wire; server also accepts
  `delete` via first-letter dispatch)
- server → client: `CHAT_TYPE_COMMAND` `cube info <gold> 0 0`

Oracle evidence: add/del only remember pointers into inventory (no inventory
mutation until `make`). After each successful add/del, `FN_update_cube_status`
emits `cube info <recipeGold> 0 0` when the bound multiset matches a recipe for
the open NPC, otherwise `cube info 0 0 0`. TMP4 `uicube.py` drives need-money
UI from that gold field.

`cube list` (test-server style INFO dump), `cancel`, and `make` / `make all`
stay deferred.

## Contract to freeze (before RED)

1. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen` with remembered `activeCubeNPCVnum`:
   - talking-chat `/cube add <cubeIndex> <invenIndex>` where both args are
     non-negative digit strings
   - talking-chat `/cube del <cubeIndex>` or `/cube delete <cubeIndex>` where
     `<cubeIndex>` is a non-negative digit string
   - non-digit / wrong-arity args stay silent fail-closed-consume (no frames,
     no binding mutation, no talking-chat fallthrough)
2. **Slot bounds**
   - `cubeIndex` must be in `0..23` (`CUBE_MAX_NUM = 24`)
   - `invenIndex` must address a live carried inventory cell
     (`WindowInventory`, `cell < 90` for the owned lab inventory)
3. **Success add**
   - bind the live inventory item identity / cell into `cubeIndex`
   - if that same live inventory cell was already bound in another cube slot,
     clear the previous slot first (oracle move-within-cube behavior)
   - overwriting an occupied `cubeIndex` replaces the previous binding
   - emit one self-only `CHAT_TYPE_COMMAND` `cube info <gold> 0 0`
   - **no** inventory / gold / quickslot / ground mutation
4. **Success del**
   - clear the binding at `cubeIndex` when present; empty slot stays silent
     fail-closed (no `cube info`)
   - when a binding was cleared, emit one self-only `cube info <gold> 0 0`
   - **no** inventory / gold / quickslot / ground mutation
5. **Gold resolution for `cube info`**
   - build a multiset of `(vnum, count)` from currently bound live inventory
     cells (missing/stale cells are ignored / treated as unbound)
   - if that multiset exactly matches one authored simple recipe's materials
     for `activeCubeNPCVnum` (order-insensitive; no complicated OR-materials),
     `gold` is that recipe's authored gold (may be `0`)
   - otherwise `gold = 0` → `cube info 0 0 0`
6. **Lifecycle**
   - `/close_cube` / lifecycle teardown / practice-mob floor / transfer clear
     all cube-slot bindings together with the busy flag / remembered NPC vnum
   - bindings are same-socket only (not durable across reconnect)
7. **Fail-closed** (silent / no frames / no binding change unless noted)
   - cube not open / remembered vnum cleared / zero-HP / no selected character
   - out-of-range cube index / inventory index
   - empty inventory cell on add
   - del on already-empty cube slot
8. Spec/QA/roadmap/packet-matrix name this binding + `cube info` seam beside
   owned `r_list` / `m_info`; `list` / `cancel` / `make` / complicated materials
   stay deferred.

## Explicit non-goals

- `cube make` / `make all` / success/fail craft mutation
- `cube list` INFO dump / `cube cancel`
- complicated OR-material matching (`vnum,count|...`)
- durable cube-slot persistence across reconnect
- inventory lock / anti-move while bound (may land with `make` if needed)
- binary cube packet headers
- quest-NPC distance gate beyond lab `/open_cube`

## Proof shape for the later implementation slice

1. Runtime/session: `/open_cube` → put bootstrap materials into inventory →
   `/cube add 0 <cell>` / `/cube add 1 <cell>` matching `{27002,2}` emits
   `cube info 100 0 0`; account inventory/gold unchanged.
2. `/cube del 0` then remaining incomplete set emits `cube info 0 0 0`.
3. Negatives: closed cube → silent; `/cube add 99 0` → silent;
   `/cube add 0 abc` → silent; empty inventory cell → silent.
4. Close clears bindings so a later reopen starts empty.
5. Docs/spec/QA update only the cube bootstrap vertical.

## Status

Docs-first freeze only on `lane/items`. RED for `/cube add` / `/cube del` →
`cube info` is intentionally deferred to the next implementation run so `main`
/ lane stay green.
