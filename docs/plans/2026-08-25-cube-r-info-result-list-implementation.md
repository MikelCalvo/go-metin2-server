# Cube `r_info` → `r_list` implementation — 2026-08-25

## Objective

Own the frozen lab cube recipe-list request seam:
`/open_cube` remembers `activeCubeNPCVnum`, then `/cube r_info` emits one
self-only `CHAT_TYPE_COMMAND` `cube r_list <npcVnum> <resultCount> <vnum,count/...>`
from authored bootstrap recipes with no inventory/gold mutation.

## Owned behavior

1. Successful `/open_cube` / `/open_cube <npcVnum>` remembers `activeCubeNPCVnum`
   beside `hasActiveCubeOpen`; close / lifecycle / floor / transfer clear both.
2. Talking-chat `/cube r_info` (no extra args) while open emits one
   `cube r_list <activeCubeNPCVnum> <resultCount> <entryText>` matching the
   authored NPC recipe list.
3. Authored source is `internal/cubestore` (FileStore round-trip + hermetic
   MemoryStore). Runtime boot uses a deterministic bootstrap snapshot for lab
   NPC `20022` (`reward {27001,1}`) when no authored file is wired yet.
4. Fail-closed silent / no frames:
   - cube not open / remembered vnum cleared
   - missing or empty NPC recipe list
   - oversize encoded entry text (`CHAT_MAX_LEN` gate)
   - non-digit / unexpected-arity `/cube r_info` args (digit index/count owned by `m_info`)
5. No inventory / gold / quickslot / ground / cube-slot mutation.

## Proofs

- `internal/cubestore`: round-trip, malformed reject, empty deterministic JSON,
  bootstrap format helper, oversize fail-closed
- `TestGameRuntimeCubeRInfoEmitsAuthoredResultListWithoutMutation`
- `TestGameRuntimeCubeRInfoFailsClosedWhenClosedMissingOrOversize`

## Explicit non-goals

- `/cube r_info <index> [count]` → `cube m_info ...`
- `cube add` / `delete` / `list` / `cancel` / `make` / `make all`
- config path / FileStore wiring for production deployments beyond bootstrap
  MemoryStore fallback
- quest-NPC interact open / distance gate beyond lab `/open_cube`
- binary cube packet headers

## Status

Implemented on `lane/items`. Material-info follow-on is now owned
(`docs/plans/2026-08-25-cube-m-info-material-info-implementation.md`).
Next Track C seam: freeze then implement craft-slot `add` / `delete` / `list`
before inventing `make`.
