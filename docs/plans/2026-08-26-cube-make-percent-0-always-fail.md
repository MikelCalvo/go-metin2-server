# Cube make store-level `percent = 0` always-fail — 2026-08-26

## Objective

Freeze then prepare the RED for authored cube recipes with `percent = 0` so
`/cube make` (and therefore `/cube make all`) can own the oracle always-fail
path without inventing a new fail burst or loosening the already-owned
`1..100` production roll seam.

## Why docs-first

- `cubestore` currently rejects `percent = 0` / `>100` fail-closed.
- Runtime one-attempt make silently consumes when `Percent == 0`.
- Oracle `Cube_make` always draws `number(1,100)` and treats
  `percent_number <= cube_proto->percent`; with `percent = 0` that comparison
  never succeeds, so materials/gold are still consumed and the owned fail burst
  (`You have failed to craft the item.` + `cube fail` + follow-up `cube info`)
  is emitted.
- Opening RED without freezing store acceptance vs runtime always-fail would
  invent whether `0` stays rejected at load time or becomes a valid always-fail
  recipe.

## Contract to freeze (before RED)

1. **Store**: `Recipe.Percent` accepts `0..100` (reject only `>100`).
   - bootstrap NPC `20022` keeps `percent: 100`
   - omitted percent remains `0` in Go zero-value terms but Make still requires
     an explicit authored percent field for craftable recipes? **Decision:**
     omitted / missing JSON percent stays `0` and is now a valid always-fail
     recipe once materials/gold match (same as explicit `0`). Deterministic JSON
     may omit zero percent for byte-compat with older snapshots that never
     authored the field — **no:** older snapshots that omitted percent were
     previously rejected at make time; after this slice omitted/`0` become
     always-fail. FileStore round-trip of explicit `0` must persist (do not omit
     `0` if that would collapse with "missing" in a way that breaks tests — prefer
     persisting explicit `0` when authored, and treat missing as `0` on load).
2. **Runtime `/cube make`**: when matched recipe `Percent == 0`:
   - do **not** draw a roll
   - consume materials/gold exactly like the owned fail-roll path
   - emit the owned fail burst (info + `cube fail` + follow-up `cube info`)
   - persist inventory/gold/quickslots
3. **`Percent` in `1..99` / `100`**: unchanged.
4. **`/cube make all`**: a `percent = 0` attempt is a non-success stop (same as
   fail-roll): append the fail burst and leave the loop.
5. Spec/QA/packet-matrix/roadmap name this always-fail seam; `cube list` /
   `cancel` stay deferred.

## Explicit non-goals

- `cube list` INFO dump / `cube cancel`
- complicated OR-material matching
- inventing a new locale string
- binary cube packet headers

## Proof shape (RED then GREEN)

1. Store: round-trip explicit `percent: 0`; reject `>100`; bootstrap still `100`.
2. Runtime/session: override recipe `percent = 0`, `/cube make` consumes
   materials/gold and emits fail burst with no reward / no roll consumption.
3. `/cube make all` with `percent = 0` emits one fail burst and stops.
4. Negatives: `percent = 100` / injected `1..99` stay unchanged.

## Status

Implemented on `lane/items` together with this freeze. `cubestore` accepts
`percent` in `0..100` (explicit `0` persists; omitted JSON percent loads as
`0`). `/cube make` / `/cube make all` treat `percent = 0` as always-fail without
drawing a roll. Next Track C cube seam candidates: `cube list` / `cancel`.
