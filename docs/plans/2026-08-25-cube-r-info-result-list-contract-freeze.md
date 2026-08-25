# Cube `r_info` → `r_list` contract freeze — 2026-08-25

## Objective

Freeze the first lab cube recipe-list request seam before RED so TMP4 clients
can ask an already-open cube for the NPC's craftable result list without
inventing `cube add` / `delete` / `make` mutation or a binary cube packet.

## Why this exists

Track C's next named item after guest MYSHOP sell fail-closed is recipe
make/add/list. The oracle has **no** `HEADER_CG/GC_CUBE`; recipe UI traffic is
command-chat:

- client → server: `/cube r_info` (no args) requests the full result list
- server → client: `CHAT_TYPE_COMMAND` `cube r_list <npcVnum> <resultCount> <vnum,count/vnum,count/...>`

Material detail (`/cube r_info <index> [count]` → `cube m_info ...`) and
slot mutation (`add` / `delete` / `list` / `make`) stay deferred until this
list seam is owned.

The current lab `/open_cube` presentation emits `cube open <npcVnum>` but does
not yet remember that vnum in same-socket state. `r_list` needs that remembered
open NPC vnum (oracle uses `GetQuestNPC()->GetRaceNum()` after open).

## Contract to freeze (before RED)

1. **Remembered open NPC vnum**
   - successful `/open_cube` / `/open_cube <npcVnum>` remembers
     `activeCubeNPCVnum` beside `hasActiveCubeOpen`
   - close / lifecycle / floor / transfer clear both
   - already-open / busy-shell / invalid open paths leave remembered vnum
     unchanged (still closed → still `0`)
2. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen`:
   - talking-chat / slash `/cube r_info` with **no** extra args
   - malformed `/cube r_info` with non-digit extra args that are not the owned
     material-info shape stay recognized fail-closed-consume (no frames) once
     this family is owned; until material-info is owned, any `/cube r_info`
     with extra args stays silent fail-closed (no `m_info`, no mutation)
3. **Success**
   - one self-only `CHAT_TYPE_COMMAND`
     `cube r_list <activeCubeNPCVnum> <resultCount> <entryText>`
   - `entryText` is `vnum,count` entries joined by `/` with no trailing `/`
   - `resultCount` equals the number of authored results for that NPC vnum
   - no inventory / gold / quickslot / ground / cube-slot mutation
4. **Authored recipe source (bootstrap)**
   - one small file-backed / in-memory recipe snapshot keyed by NPC vnum
   - bootstrap fixture for default lab NPC `20022` with at least one
     `reward {vnum, count}` row (materials/gold may be present in the fixture
     for later `m_info` but are unused by this list-only seam)
   - missing NPC key or empty result list → silent fail-closed (oracle returns
     with no frame when `resultCount == 0`)
5. **Fail-closed**
   - cube not open → silent / no frames
   - zero-HP / no selected character → silent
   - oversize encoded `entryText` that would blow the chat command budget →
     silent fail-closed (oracle clears and logs; bootstrap does not invent a
     truncated partial list)
6. Spec/QA/roadmap/packet-matrix name this list seam beside owned cube
   open/close; `m_info`, `add`, `delete`, `list`, `make`, and quest-NPC distance
   gates stay deferred.

## Explicit non-goals

- `/cube r_info <index> [count]` → `cube m_info ...`
- `cube add` / `delete` / `list` / `cancel` / `make` / `make all`
- `cube.txt` full parser parity / complicated OR-material text beyond what the
  list fixture needs
- authored quest-NPC interact open (lab `/open_cube` remains the open harness)
- binary cube packet headers
- tax / empire / shop-bag / mall

## Proof shape for the later implementation slice

1. Store/catalog (if a dedicated recipe store lands): round-trip one NPC recipe
   list; reject malformed rows fail-closed; deterministic JSON.
2. Runtime/session: `/open_cube` → `/cube r_info` emits one
   `cube r_list 20022 N vnum,count/...` matching the fixture; account unchanged.
3. Negatives: no open → silent; empty/missing NPC recipes → silent; close then
   `r_info` → silent; oversize fixture (test-only) → silent.
4. Docs/spec/QA update only the cube bootstrap vertical.

## Status

Docs-first freeze only on `lane/items`. RED for remembered `activeCubeNPCVnum`
+ `/cube r_info` → `cube r_list` is intentionally deferred to the next
implementation run so `main` / lane stay green.
