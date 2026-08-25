# Cube craft-slot `add` / `del` → `cube info` implementation — 2026-08-25

## Objective

Own the frozen lab cube craft-slot binding seam:
while cube is open, `/cube add <cubeIndex> <invenIndex>` and
`/cube del|delete <cubeIndex>` remember same-socket inventory-cell pointers and
emit one self-only `CHAT_TYPE_COMMAND` `cube info <gold> 0 0` from authored
simple-recipe material matching, with **no** inventory/gold/quickslot mutation.

## Owned behavior

1. Slot bounds: `cubeIndex` in `0..23`, `invenIndex` addresses a live carried
   inventory cell (`cell < 90`).
2. Successful add binds the live inventory cell; move-within-cube clears any
   previous slot that already pointed at the same cell; overwrite replaces the
   target slot binding.
3. Successful del clears an occupied craft slot; empty-slot del stays silent.
4. Gold resolution aggregates bound live `(vnum,count)` and exact-matches one
   authored simple recipe for `activeCubeNPCVnum`; otherwise `gold = 0`.
5. Close / lifecycle / floor / transfer clear all bindings with the busy flag /
   remembered NPC vnum.
6. Fail-closed silent / no frames / no binding change:
   - cube not open / remembered vnum cleared / zero-HP / no selected character
   - out-of-range cube/inventory index
   - empty inventory cell on add
   - malformed / wrong-arity args

## Proofs

- `internal/cubestore`: `MatchSimpleRecipeGold` / `FormatCubeInfoCommand`
- `TestGameRuntimeCubeAddDelEmitsAuthoredCubeInfoWithoutMutation`
- `TestGameRuntimeCubeAddDelFailsClosedWhenClosedOutOfRangeOrMalformed`

## Explicit non-goals

- `cube list` / `cancel` / `make` / `make all`
- complicated OR-material matching
- durable cube-slot persistence across reconnect
- inventory lock / anti-move while bound
- binary cube packet headers
- quest-NPC distance gate beyond lab `/open_cube`

## Status

Implemented on `lane/items`. Next Track C seam: freeze then implement
`cube make` (or `cube list` INFO dump) from bound slots + authored recipes;
`cancel` / complicated materials stay deferred.
