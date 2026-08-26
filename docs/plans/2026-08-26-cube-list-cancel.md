# Cube `list` INFO dump + `cancel`/`close` alias — 2026-08-26

## Objective

Freeze then implement the remaining lab cube command-chat seams after
`percent = 0` always-fail:

- `/cube list` → self-only `CHAT_TYPE_INFO` dump of currently bound craft slots
- `/cube cancel` and `/cube close` → same presentation close path already owned
  by `/close_cube` (`cube close` command chat + clear busy bit / remembered NPC /
  bindings)

## Why this exists

Oracle `do_cube` first-letter dispatch maps `l` → `Cube_show_list` and `c` →
`Cube_close`. Help text lists both `cube list` and `cube cancel` beside
`cube close`. After make / make-all / percent seams, Track C still deferred
exactly these two presentation helpers.

Clean-room policy: do not copy oracle source. Re-derive from observed behavior:

- `Cube_show_list` requires an open cube; for each occupied cube index it emits
  one self-only INFO line shaped `cube[<cubeIndex>]: inventory[<invenCell>]: <name>`
- empty bindings emit nothing
- `Cube_close` cleans bindings, clears the cube NPC, and emits
  `CHAT_TYPE_COMMAND` `cube close` — the same burst `/close_cube` already owns

## Contract to freeze (before / with GREEN)

1. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen` with remembered `activeCubeNPCVnum`:
   - talking-chat `/cube list` (exact token; no extra args)
   - talking-chat `/cube cancel` or `/cube close` (exact tokens; no extra args)
   - wrong-arity / unknown `/cube …` variants that are not already owned stay
     unrecognized (ordinary talking-chat fallthrough) unless a dedicated
     recognizer already consumes them fail-closed
2. **`/cube list` success**
   - walk cube indices `0..CubeMaxNum-1` in order
   - for each bound inventory cell that still resolves to one live carried item,
     emit one self-only `CHAT_TYPE_INFO` with
     `message = cube[<i>]: inventory[<cell>]: <template.name>`
   - `<template.name>` comes from the loaded item-template snapshot for the live
     item `vnum`; missing/blank template name still emits the line with an empty
     name suffix (format preserved)
   - stale / unbound / empty live cells are skipped
   - **no** inventory / gold / quickslot / binding / busy-bit mutation
3. **`/cube list` fail-closed** (silent consume, no frames, no mutation)
   - cube not open / remembered vnum cleared / zero-HP / no selected character
   - extra args after `list`
4. **`/cube cancel` / `/cube close` success**
   - identical to owned `/close_cube`: clear open flag, remembered NPC vnum,
     craft-slot bindings, and peer-visible cube busy bit; emit one self-only
     `CHAT_TYPE_COMMAND` `cube close`
5. **`/cube cancel` / `/cube close` when already closed**
   - identical to already-closed `/close_cube`: silent consume, no frames
6. Spec/QA/packet-matrix/roadmap name these seams; complicated OR-materials and
   binary cube headers stay deferred.

## Explicit non-goals

- first-letter dispatch for arbitrary `/cube c…` / `/cube l…` typos beyond the
  exact tokens frozen above
- inventing item names when templates are absent beyond the empty-suffix rule
- durable cube-slot persistence
- quest-NPC distance gate beyond lab `/open_cube`
- binary cube packet headers
- complicated OR-material matching

## Proof shape

1. Runtime/session: `/open_cube` → bind bootstrap materials → `/cube list`
   emits ordered INFO lines using authored template names; account unchanged.
2. Empty open cube / closed cube `/cube list` → no frames.
3. `/cube cancel` (and `/cube close`) after open → one `cube close` command;
   later `/cube list` / `/cube add` stay closed-silent.
4. Already-closed cancel/close → no frames.

## Status

Implemented on `lane/items` together with this freeze.
