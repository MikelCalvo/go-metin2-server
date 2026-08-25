# MYSHOP / Safebox / Refine Cube Busy Rejects — 2026-08-25

## Objective

Once open cube already rejects exchange START/ACCEPT/commit, freeze the adjacent
presentation-shell busy rejects so an open lab cube also blocks:

1. host `CG::MYSHOP` open
2. guest MYSHOP browse open (`ON_CLICK` against an already-open private shop)
3. `/open_safebox` / password-open presentation
4. refine confirm (matching preview confirm while cube is open)

## Why this exists

Exchange now owns open-cube busy rejects
(`docs/plans/2026-08-25-exchange-cube-busy-window-reject-chat.md`), but MYSHOP
open, guest browse, safebox open, and refine confirm still ignore
`hasActiveCubeOpen`. The roadmap's next Track C item names those shells before
recipe make/add/list or guest sell-into-PC-shop.

## Contract to freeze (before RED)

Reuse already-owned busy strings / fail-closed shapes; do not invent cube-specific
chat for these shells:

1. **Host MYSHOP open** while `hasActiveCubeOpen`:
   - one self-only `CHAT_TYPE_INFO` using the already-owned requester busy string
     (`You cannot trade while another trade window is open.`)
   - no `GC::SHOP_SIGN`, no same-socket MYSHOP busy flag, no inventory/gold mutation
   - cube presentation remains open
2. **Guest MYSHOP browse open** (`ON_CLICK`) while the guest has open cube:
   - one self-only requester busy info-chat
   - no `GC::SHOP START`, no browse flag
3. **Safebox open** (`/open_safebox` or successful `/safebox_password`) while cube is open:
   - one self-only requester busy info-chat (same exchange/trade-window string)
   - no `SAFEBOX_SIZE` / money burst / open flag
   - cube stays open; pending password challenge (if any) is left for a later tiny
     clarification only if tests prove a conflict — prefer fail-closed no-open
4. **Refine confirm** while cube is open (matching preview already remembered):
   - fail closed with no refine mutation / no success or destroy frames
   - mirror the already-owned merchant/safebox/MYSHOP/exchange confirm busy gate
     (silent `Accepted: false` today)
5. Ordering / local-first stays unchanged; recipe mutation and guest sell stay
   deferred.

## Explicit non-goals

- no `cube add` / `delete` / `list` / `make` / recipe mutation
- no guest sell-into-PC-shop / tax / shop-bag consumption
- no authored overrides for busy strings
- no claim that cube crafting is playable
- no broadening into mall / TMP4 CG `SAFEBOX_MONEY`

## Proof shape for the implementation slice

1. Runtime/session: `/open_cube` → host `CG::MYSHOP` emits requester busy chat and
   no `SHOP_SIGN`.
2. Runtime/session: `/open_cube` on guest → guest browse `ON_CLICK` emits
   requester busy chat and no `SHOP START`.
3. Runtime/session: `/open_cube` → `/open_safebox` emits requester busy chat and
   no safebox open frames.
4. Runtime/session: `/open_cube` → refine preview → matching confirm stays
   fail-closed with no mutation; `/close_cube` then allows confirm.
5. Docs/spec/QA name these beside the owned exchange open-cube busy rejects.

## Status

Implemented on `lane/items`: host `CG::MYSHOP` open, guest MYSHOP browse
open, `/open_safebox` / successful `/safebox_password`, and matching refine
confirm all reject open lab cube with the already-owned requester busy
info-chat (or silent confirm fail-closed for refine) and no presentation /
mutation. Guest `SHOP SELL` / `SELL2` while browsing fail-closed is owned separately (`docs/plans/2026-08-25-myshop-guest-sell-while-browsing-fail-closed.md`); recipe make/add/list stay deferred.
