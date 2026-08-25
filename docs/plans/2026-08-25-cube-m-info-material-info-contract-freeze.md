# Cube `r_info <index>` → `m_info` contract freeze — 2026-08-25

## Objective

Freeze the next lab cube material-info request seam before RED so TMP4 clients
can ask an already-open cube for the materials/gold of one (or a short range of)
craftable results without inventing `cube add` / `delete` / `make` mutation.

## Why this exists

`/cube r_info` → `cube r_list` is now owned. The oracle's next command-chat
family is material detail:

- client → server: `/cube r_info <index>` (default count `1`) or
  `/cube r_info <index> <count>`
- server → client: `CHAT_TYPE_COMMAND`
  `cube m_info <startIndex> <count> <infoText[@infoText...]>`

Craft slot mutation (`add` / `delete` / `list` / `make`) stays deferred until
this material-info seam is owned.

## Contract to freeze (before RED)

1. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen` with remembered `activeCubeNPCVnum`:
   - talking-chat `/cube r_info <index>` where `<index>` is a non-negative digit
     string → request material info starting at that result-list index with
     default `count = 1`
   - talking-chat `/cube r_info <index> <count>` where both args are digit
     strings → request that many consecutive results starting at `<index>`
   - bare `/cube r_info` remains the owned result-list path
   - non-digit extra args stay silent fail-closed-consume (no `m_info`, no
     mutation, no talking-chat fallthrough)
2. **Success**
   - one self-only `CHAT_TYPE_COMMAND`
     `cube m_info <startIndex> <requestCount> <entryText>`
   - `startIndex` / `requestCount` echo the parsed request args (oracle echo)
   - `entryText` is one or more recipe `infoText` values joined by `@` with no
     trailing `@`
   - bootstrap `infoText` for a simple (non-complicated) recipe is
     `vnum,count[&vnum,count...][/gold]` using the authored materials in order
     and appending `/<gold>` only when authored gold is non-zero
   - no inventory / gold / quickslot / ground / cube-slot mutation
3. **Authored source**
   - reuse `internal/cubestore` recipe rows already keyed by NPC vnum
   - materials + gold already round-trip in the store; this seam is the first
     runtime consumer of those fields
   - index addresses the NPC's ordered recipe list (same order as `r_list`)
4. **Fail-closed** (silent / no frames / no mutation)
   - cube not open / remembered vnum cleared / zero-HP / no selected character
   - start index past the end of the NPC recipe list
   - requested window yields no material text
   - oversize encoded `entryText` that would blow the chat command budget
     (`CHAT_MAX_LEN` / overhead reserve matching `r_list`)
5. Spec/QA/roadmap/packet-matrix name this material-info seam beside owned
   `r_list`; `add` / `delete` / `list` / `make` / quest-NPC distance gates stay
   deferred.

## Explicit non-goals

- complicated OR-material text (`vnum,count|vnum,count&...`) / name-level merge
  of alternate recipes into one result row
- `cube add` / `delete` / `list` / `cancel` / `make` / `make all`
- binary cube packet headers
- production FileStore path wiring beyond the existing bootstrap MemoryStore
  fallback (may land later if needed for operator authoring)
- tax / empire / shop-bag / mall

## Proof shape for the later implementation slice

1. Store helper (or pure formatter): simple recipe materials+gold encode to the
   expected `infoText`; oversize fail-closed.
2. Runtime/session: `/open_cube` → `/cube r_info 0` emits one
   `cube m_info 0 1 27002,2/100` matching the bootstrap fixture; account
   unchanged.
3. Negatives: closed cube → silent; `/cube r_info 99` past end → silent;
   `/cube r_info abc` → silent; oversize fixture (test-only) → silent.
4. Docs/spec/QA update only the cube bootstrap vertical.

## Status

Implemented on `lane/items` (`docs/plans/2026-08-25-cube-m-info-material-info-implementation.md`).
Add / delete / list / make stay deferred.
