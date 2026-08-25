# Item cube bootstrap

This note freezes the first clean-room lab cube boundary for the bootstrap item
lane.

The goal is deliberately conservative:

- own command-chat open/close presentation plus a peer-visible busy bit
- own open-cube busy rejects for exchange / MYSHOP / safebox / refine shells
- own the first recipe **result-list** request (`/cube r_info` →
  `cube r_list`) from remembered open NPC vnum + authored `cubestore` recipes
- own the first recipe **material-info** request (`/cube r_info <index> [count]`
  → `cube m_info`) from the same authored materials/gold rows
- own the first craft-slot **binding** seam (`/cube add` / `/cube del` →
  `cube info`) as same-socket inventory pointers + gold hint, with no inventory
  mutation until a later `make` slice
- keep `list` / `cancel` / `make` deferred until a later cube slice owns those
  mutation semantics; `/cube make` (`percent = 100`) is now contract-frozen
  separately

This is not a completed cube / craft system.

## Evidence

The TMP4-compatible client has **no** dedicated `HEADER_CG/GC_CUBE`. The
external behavior oracle opens and closes the build window with
`CHAT_TYPE_COMMAND` `cube open <npcRace>` / `cube close`, and requests craftable
results with talking-chat `/cube r_info` (no args). The server replies with
`CHAT_TYPE_COMMAND` `cube r_list <npcVnum> <resultCount> <vnum,count/...>`.

## Owned lab open/close presentation

See `docs/plans/2026-08-25-cube-open-close-presentation-busy-bit.md`.

- `/open_cube` / `/open_cube <npcVnum>` → one self-only `cube open <npcVnum>`
  (default `20022`) + same-socket busy flag + peer-visible shared-world cube
  busy bit
- `/close_cube` / lifecycle / practice-mob floor / transfer → one self-only
  `cube close` when open
- already-open / busy merchant|safebox|refine|MYSHOP|exchange → owned info-chat
  strings; no second open

Successful open must remember the opened NPC vnum beside the busy flag so later
recipe-list responses can echo that vnum. The remembered vnum clears with the
busy flag.

## Owned open-cube busy rejects

Open cube rejects exchange `START` / `ACCEPT` / commit, host `CG::MYSHOP` open,
guest MYSHOP browse open, `/open_safebox` / successful `/safebox_password`, and
refine confirm with the already-owned requester/partner busy chat strings or
silent refine confirm fail-closed.

See `docs/plans/2026-08-25-exchange-cube-busy-window-reject-chat.md` and
`docs/plans/2026-08-25-myshop-safebox-refine-cube-busy-rejects.md`.

## Frozen recipe result-list request

While the cube presentation is open:

| Direction | Command chat | Policy |
| --- | --- | --- |
| client → server | `/cube r_info` (no args) | request craftable result list for the remembered open NPC vnum |
| server → client | `cube r_list <npcVnum> <resultCount> <vnum,count/vnum,count/...>` | self-only; no inventory/gold/slot mutation |

Fail-closed (no frames / no mutation):

- cube not open
- missing/empty authored recipe list for the remembered NPC vnum
- oversize encoded list that would exceed the chat command budget

Authored bootstrap recipes are keyed by NPC vnum through `internal/cubestore`.
Runtime boot falls back to a deterministic lab snapshot for default NPC `20022`
(`reward {27001,1}`, materials `{27002,2}`, gold `100`) until an explicit
FileStore path is wired.

Successful `/open_cube` remembers `activeCubeNPCVnum` beside the busy flag so
`r_list` / `m_info` can echo that NPC's authored rows. The remembered vnum
clears with the busy flag.

See `docs/plans/2026-08-25-cube-r-info-result-list-contract-freeze.md` and
`docs/plans/2026-08-25-cube-r-info-result-list-implementation.md`.

## Frozen recipe material-info request

While the cube presentation is open:

| Direction | Command chat | Policy |
| --- | --- | --- |
| client → server | `/cube r_info <index>` | request materials for one result at that index (default count `1`) |
| client → server | `/cube r_info <index> <count>` | request that many consecutive results starting at `<index>` |
| server → client | `cube m_info <startIndex> <requestCount> <infoText[@...]>` | self-only; echoes parsed args; no inventory/gold/slot mutation |

Bootstrap simple-recipe `infoText` is `vnum,count[&vnum,count...][/gold]`
(gold appended only when authored gold is non-zero). Multiple recipes in the
requested window join with `@` and no trailing `@`.

Fail-closed (no frames / no mutation):

- cube not open / remembered vnum cleared / zero-HP / no selected character
- start index past the end of the NPC recipe list
- empty materials / empty encoded window
- oversize encoded entry text (`CHAT_MAX_LEN` + overhead reserve)
- non-digit index/count or unexpected arity

See `docs/plans/2026-08-25-cube-m-info-material-info-contract-freeze.md` and
`docs/plans/2026-08-25-cube-m-info-material-info-implementation.md`.

## Owned craft-slot binding + `cube info`

While the cube presentation is open:

| Direction | Command chat | Policy |
| --- | --- | --- |
| client → server | `/cube add <cubeIndex> <invenIndex>` | bind a live carried inventory cell into craft slot `0..23` |
| client → server | `/cube del <cubeIndex>` / `/cube delete <cubeIndex>` | clear that craft-slot binding when present |
| server → client | `cube info <gold> 0 0` | self-only gold hint after successful add/del; no inventory/gold mutation |

Gold resolution aggregates currently bound live `(vnum,count)` cells and
exact-matches one authored simple recipe for `activeCubeNPCVnum`
(order-insensitive). Otherwise `gold = 0`. Close / lifecycle / floor / transfer
clear all bindings with the busy flag / remembered NPC vnum.

Fail-closed (no frames / no binding change unless noted):

- cube not open / remembered vnum cleared / zero-HP / no selected character
- out-of-range cube index / inventory index
- empty inventory cell on add
- del on already-empty craft slot
- non-digit / wrong-arity args

See `docs/plans/2026-08-25-cube-add-del-slot-binding-contract-freeze.md` and
`docs/plans/2026-08-25-cube-add-del-slot-binding-implementation.md`.

### Still deferred

- complicated OR-material text (`vnum,count|...`) / name-level merge of
  alternate recipes into one result row
- `/cube make` deterministic `percent = 100` success (contract-frozen:
  `docs/plans/2026-08-25-cube-make-percent-100-contract-freeze.md`; not yet GREEN)
- `cube list` / `cancel` / `make all` / fail rolls (`percent` in `0..99`)
- quest-NPC interact open / distance gate beyond lab `/open_cube`
- binary cube packet headers
- full `cube.txt` complicated-material parity

## Related docs

- `docs/plans/2026-08-25-cube-open-close-presentation-busy-bit.md`
- `docs/plans/2026-08-25-cube-r-info-result-list-contract-freeze.md`
- `docs/plans/2026-08-25-cube-r-info-result-list-implementation.md`
- `docs/plans/2026-08-25-cube-m-info-material-info-contract-freeze.md`
- `docs/plans/2026-08-25-cube-m-info-material-info-implementation.md`
- `docs/plans/2026-08-25-cube-add-del-slot-binding-contract-freeze.md`
- `docs/plans/2026-08-25-cube-add-del-slot-binding-implementation.md`
- `docs/plans/2026-08-25-cube-make-percent-100-contract-freeze.md`
- `docs/qa/manual-client-checklist.md` section 4.5.16
- `spec/protocol/packet-matrix.md` (command-chat cube family note)
