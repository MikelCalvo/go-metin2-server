# Cube `r_info <index>` → `m_info` implementation — 2026-08-25

## Objective

Own the frozen lab cube material-info request seam:
while cube is open, `/cube r_info <index>` (default count `1`) or
`/cube r_info <index> <count>` emits one self-only `CHAT_TYPE_COMMAND`
`cube m_info <startIndex> <requestCount> <infoText[@...]>` from authored
`cubestore` materials/gold with no inventory/gold/slot mutation.

## Owned behavior

1. Bare `/cube r_info` remains the owned result-list path (`cube r_list`).
2. Digit `/cube r_info <index>` / `/cube r_info <index> <count>` while open
   emits one `cube m_info` echoing the parsed start index and request count.
3. Bootstrap simple-recipe `infoText` is `vnum,count[&vnum,count...][/gold]`
   (gold appended only when authored gold is non-zero). Multiple recipes in the
   requested window join with `@` and no trailing `@`.
4. Authored source reuses `internal/cubestore` recipe rows already keyed by NPC
   vnum; index addresses the same ordered list as `r_list`.
5. Fail-closed silent / no frames:
   - cube not open / remembered vnum cleared / zero-HP / no selected character
   - start index past the end of the NPC recipe list
   - empty materials / empty encoded window
   - oversize encoded entry text (`CHAT_MAX_LEN` + overhead reserve)
   - non-digit index/count or unexpected arity
6. No inventory / gold / quickslot / ground / cube-slot mutation.

## Proofs

- `internal/cubestore`: `FormatRecipeMaterialInfoText` /
  `FormatMaterialInfoCommand` bootstrap + multi-material + past-end / oversize
- `TestGameRuntimeCubeRInfoIndexEmitsAuthoredMaterialInfoWithoutMutation`
- `TestGameRuntimeCubeRInfoIndexFailsClosedWhenClosedPastEndOrMalformed`

## Explicit non-goals

- complicated OR-material text (`vnum,count|...`) / name-level merge of
  alternate recipes into one result row
- `cube add` / `delete` / `list` / `cancel` / `make` / `make all`
- binary cube packet headers
- production FileStore path wiring beyond the existing bootstrap MemoryStore
  fallback
- tax / empire / shop-bag / mall

## Status

Implemented on `lane/items`. Next Track C seam is now owned:
`/cube add` / `/cube del` → `cube info`
(`docs/plans/2026-08-25-cube-add-del-slot-binding-implementation.md`);
`list` / `cancel` / `make` stay deferred.
