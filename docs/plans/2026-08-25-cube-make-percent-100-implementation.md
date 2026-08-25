# Cube `make` percent-100 implementation — 2026-08-25

## Objective

Own the frozen lab cube craft-consumption seam:
while cube is open with matching craft-slot bindings, `/cube make` consumes
bound materials + recipe gold for authored `percent = 100`, grants the reward,
persists inventory/gold/quickslots, and emits self-only refresh frames ending in
`cube success <vnum> <count>` plus a follow-up `cube info`.

## Owned behavior

1. Authored `cubestore.Recipe.Percent` round-trips (`1..100`); bootstrap NPC
   `20022` authors `percent: 100`.
2. Ingress: talking-chat `/cube make` with no extra args while cube is open /
   remembered NPC is live / above zero-HP floor.
3. Fail-closed before mutation:
   - closed cube / floor / no remembered NPC → silent
   - `/cube make all` / extra args → silent recognized consume
   - unmatched bindings → `You do not have enough materials.`
   - insufficient gold → `Not enough Yang or the item is not in place.`
   - reward cannot place → `You have too many items.`
   - non-100 percent → silent until a later roll slice
4. Success (`percent = 100`):
   - consume exact counts from bound live cells (cube-slot order)
   - clear emptied craft-slot bindings
   - debit recipe gold
   - grant reward through ordinary carried placement/merge
   - persist inventory + gold + quickslots
   - emit material UPDATE/DEL → emptied-cell QUICKSLOT_DEL → reward SET/UPDATE →
     gold POINT_CHANGE → `cube success` → follow-up `cube info`
5. Busy shells: prepend owned SHOP-before-exchange teardown when present.

## Proofs

- `internal/cubestore`: percent round-trip / reject `0` / `>100`;
  `MatchSimpleRecipe` / `FormatCubeSuccessCommand`
- `TestGameRuntimeCubeMakePercent100ConsumesGrantsPersistsAndEmitsBurst`
- `TestGameRuntimeCubeMakeFailsClosedWhenClosedUnmatchedGoldOrInventoryFull`
- `TestGameRuntimeCubeMakeRejectsInventoryFullWithoutMutation`

## Explicit non-goals

- `/cube make all` loop
- `percent` in `0..99` injected/fail rolls + `cube fail`
- `cube list` / `cancel`
- complicated OR-material matching
- durable cube-slot persistence
- quest-NPC distance gate beyond lab `/open_cube`
- binary cube packet headers

## Status

Implemented on `lane/items`. Next Track C cube seam candidates:
fail-roll `percent` in `0..99`, `make all`, or `cube list` / `cancel`.
