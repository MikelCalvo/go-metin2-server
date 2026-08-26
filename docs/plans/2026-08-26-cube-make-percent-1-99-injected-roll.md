# Cube `make` percent 1..99 injected roll — 2026-08-26

## Objective

Own the next lab cube craft-consumption seam after deterministic
`percent = 100`: while cube is open with matching craft-slot bindings, `/cube
make` against an authored recipe with `percent` in `1..99` draws one roll in
`1..100`, then either grants the reward (`cube success`) or keeps the consumed
materials/gold lost (`cube fail` + info chat), matching the refine lane's
staged probability rollout and the external oracle's `Cube_make` ordering.

## Why this exists

`/cube make` already owns deterministic `percent = 100` success. The oracle's
next behavior is the injected roll:

1. cube open + recipe match + gold gate (already owned)
2. materials removed + gold debited
3. `number(1,100)` compared to authored `percent`
4. success → `cube success <vnum> <count>` + AutoGiveItem
5. fail → info chat + `cube fail` (materials/gold already consumed; no reward)

Bootstrap keeps the already-owned inventory-full pre-mutation reject for any
path that can still succeed (`percent` in `1..100`), and keeps `percent = 100`
deterministic without drawing a roll.

## Contract to own

1. **Authored percent** stays `1..100` in `cubestore` (store still rejects `0`
   / `>100`). Bootstrap NPC `20022` keeps `percent: 100`.
2. **Ingress** remains talking-chat `/cube make` with no extra args while cube
   is open / remembered NPC is live / above zero-HP floor.
   `/cube make all` / extra args stay silent recognized consume.
3. **Preconditions (fail-closed, no mutation)** stay as owned for materials /
   gold / inventory-full / closed cube.
4. **Roll seam** for matched recipes with `percent` in `1..99`:
   - draw one roll in `1..100` via `takeCubeMakeRoll()` (`crypto/rand`
     production; `QueueCubeMakeRollForTest` for tests)
   - roll outside `1..100` → silent fail-closed, no mutation, cube stays open
   - `roll <= percent` → apply the already-owned success mutation/burst ending
     in `CHAT_TYPE_COMMAND` `cube success <vnum> <count>` + follow-up
     `cube info`
   - `roll > percent` → consume materials/gold (same consume path as success),
     clear emptied craft-slot bindings, persist inventory/gold/quickslots, and
     emit self-only frames in this order:
     1. material `ITEM_UPDATE` / `ITEM_DEL`
     2. emptied-cell `QUICKSLOT_DEL`
     3. gold `PLAYER_POINT_CHANGE` when gold changed
     4. `CHAT_TYPE_INFO` `You have failed to craft the item.`
     5. `CHAT_TYPE_COMMAND` `cube fail`
     6. follow-up `cube info <gold> 0 0` from remaining bindings
5. **`percent = 100`** keeps the already-owned deterministic success path and
   does not require a roll.
6. Spec/QA/packet-matrix/roadmap name this injected-roll seam beside the owned
   `percent = 100` make vertical; `make all`, `list`, `cancel`, and store-level
   `percent = 0` stay deferred.

## Explicit non-goals

- `/cube make all` loop
- store-validated `percent = 0` always-fail recipes
- `cube list` / `cancel`
- complicated OR-material matching
- durable cube-slot persistence
- binary cube packet headers
- inventing a TMP4 locale string beyond the frozen English info chat

## Proof shape

1. `cubestore`: `FormatCubeFailCommand` returns `cube fail`.
2. Runtime/session success: override recipe `percent = 75`, queue roll `75`,
   `/cube make` emits the owned success burst / persists reward.
3. Runtime/session fail: override recipe `percent = 75`, queue roll `76`,
   `/cube make` consumes materials/gold, emits info + `cube fail` + `cube info`,
   persists empty materials / reduced gold / no reward.
4. Negatives: out-of-range injected roll stays silent/no-mutation;
   `percent = 100` still succeeds without consuming a queued roll.
5. Docs/spec/QA update only the cube bootstrap vertical.

## Status

Implemented on `lane/items` together with this freeze. `/cube make all` is now
owned via `docs/plans/2026-08-26-cube-make-all-loop.md`. Next Track C cube seam
candidates: `cube list` / `cancel`, or store-level `percent = 0` always-fail.
