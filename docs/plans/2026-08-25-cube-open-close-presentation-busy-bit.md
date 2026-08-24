# Cube open/close presentation + busy bit — 2026-08-25

## Why this exists

Track C's next roadmap item names cube busy rejects for exchange / MYSHOP /
safebox / refine. The Go runtime still has **no** cube presentation seam, and
the MYSHOP precedent required an accepted open/close presentation plus a
peer-visible busy bit **before** START/ACCEPT/commit busy rejects could be
honest.

The external oracle has **no** `HEADER_CG/GC_CUBE`. Open/close is
`CHAT_TYPE_COMMAND` `cube open <npcRace>` / `cube close`, with
`IsCubeOpen() ≡ (pCubeNpc != NULL)`.

This slice owns **lab open/close presentation + peer-visible busy bit only**.
Exchange / MYSHOP / safebox / refine cube busy rejects, recipe make/add/list,
and guest sell-into-PC-shop stay deferred.

## Oracle summary (behavior reference only)

- Ingress: `do_cube` → `cube open` / `cube close` (`cmd_general.cpp`)
- Open success: `SetCubeNpc(npc)` then `ChatPacket(CHAT_TYPE_COMMAND, "cube open %d", npc->GetRaceNum())`
- Close: clear NPC + `ChatPacket(CHAT_TYPE_COMMAND, "cube close")`
- Already-open: info-chat `The Build window is already open.`
- Busy exchange/MYSHOP/shop/safebox on open: info-chat
  `You cannot build something while another trade/storeroom window is open.`
- Distance / quest-NPC validity gates exist in oracle open; lab harness may omit
  NPC distance until an authored cube NPC interact seam exists
- Battle/death closes via `Cube_close`

## Contract

1. Scope: selected character already in `GAME`, above the zero-HP floor.
2. Lab ingress (slash, talking chat), mirroring `/open_safebox` / `/close_myshop`:
   - `/open_cube` or `/open_cube <npcVnum>` where `<npcVnum>` is a positive
     decimal `uint32`; omitted vnum uses bootstrap default `20022`
     (oracle cube-opener quest NPC family). Extra args / `0` / non-decimal stay
     recognized fail-closed-consume (no frames / no open flag).
   - `/close_cube` clears an open presentation.
3. Already-open `/open_cube`: one self-only `CHAT_TYPE_INFO`
   `The Build window is already open.`; no second open command.
4. Busy-shell gate on open: when same-socket exchange / merchant / safebox /
   refine / MYSHOP is already open, one self-only `CHAT_TYPE_INFO`
   `You cannot build something while another trade/storeroom window is open.`;
   no `cube open` command and no busy bit.
5. Success open:
   - remember same-socket `hasActiveCubeOpen` + remembered npc vnum
   - publish peer-visible shared-world cube busy bit (`SetCubeWindowOpen`)
   - emit one self-only `CHAT_TYPE_COMMAND` `cube open <npcVnum>`
   - no inventory / gold / quickslot / ground / recipe mutation
6. Close (`/close_cube`, lifecycle `/quit|/logout|/phase_select`, practice-mob
   floor, transfer/warp teardown):
   - clear same-socket flag + shared-world busy bit
   - emit one self-only `CHAT_TYPE_COMMAND` `cube close` when the presentation
     was open; already-closed close stays silent consume
7. Ordering when multiple shells close together: merchant `SHOP END` → MYSHOP
   empty-sign / guest browse END → `cube close` → safebox `CloseSafebox` →
   exchange `END` (refine busy bit still clears silently as today).
8. Spec/QA/roadmap name this presentation seam; cube busy rejects for exchange
   START/ACCEPT/commit and for MYSHOP/safebox/refine open/confirm stay deferred
   to the next slice once the busy bit exists.

## Explicit non-goals

- `cube add` / `delete` / `list` / `make` / `r_info` / `m_info` / `cube.txt` recipes
- authored quest-NPC cube interact / distance gate beyond the lab slash
- exchange / MYSHOP / safebox / refine cube busy-window rejects (next slice)
- guest sell-into-PC-shop / tax / shop-bag consumption
- inventing a binary cube packet header

## Proof shape

1. Runtime/session: `/open_cube` → one `cube open <vnum>` command chat; account
   unchanged; shared-world busy bit set.
2. `/close_cube` → one `cube close`; busy bit cleared; already-closed silent.
3. Busy merchant/exchange/safebox/refine/MYSHOP → busy info-chat, no open.
4. Already-open → already-open info-chat.
5. Lifecycle / floor / transfer emit `cube close` when open.

## Status

Implemented on `lane/items`: lab `/open_cube` / `/close_cube` own command-chat
open/close presentation plus peer-visible cube busy bit and lifecycle close
companions. Exchange / MYSHOP / safebox / refine cube busy rejects and recipe
mutation stay deferred.
