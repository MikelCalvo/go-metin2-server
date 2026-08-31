# Exchange Cube Busy-Window Reject Chat — 2026-08-25

## Objective

Once lab cube open/close presentation + peer-visible busy bit exist, freeze the
first cube busy-window reject so an open cube blocks `EXCHANGE START` /
`ACCEPT` / commit the same way merchant / safebox / refine / MYSHOP already do.

## Why this exists

Lab `/open_cube` now remembers a same-socket busy flag and publishes
`SetCubeWindowOpen`, but exchange START/ACCEPT/commit still ignore that bit.
The roadmap's next Track C item is cube busy rejects for exchange / MYSHOP /
safebox / refine presentation shells.

This freeze is **exchange busy-window reject chat for open cube**. It does not
invent recipe make/add/list, guest sell-into-PC-shop, or MYSHOP/safebox/refine
open/confirm cube rejects beyond naming them as adjacent follow-ons.

## Contract to freeze (before RED)

Own open-cube busy rejects beside the already-owned merchant/safebox/refine/MYSHOP busy gate:

1. Requester-side open cube (`hasActiveCubeOpen` / shared-world cube busy bit) rejects:
   - `EXCHANGE START`
   - `EXCHANGE ACCEPT` (first or second)
   - commit-time busy drift after a second-accept finalize plan
   with one self-only `CHAT_TYPE_INFO` using the already-owned requester busy string
   (`You cannot trade while another trade window is open.`).
2. Partner-side open cube rejects the same seams with one self-only
   `CHAT_TYPE_INFO` using the already-owned partner busy string
   (`That player cannot trade right now.`).
3. When both sides are busy, requester busy text wins (local-first).
4. No exchange frames / no pairing mutation on `START`; no accept marker / no
   finalize frames / no inventory-gold mutation on `ACCEPT`/commit; the cube
   presentation remains open.
5. Spec/QA name cube beside merchant/safebox/refine/MYSHOP in the exchange busy
   matrix; MYSHOP/safebox/refine open/confirm cube rejects and guest
   sell-into-PC-shop stay deferred.

## Explicit non-goals

- no `cube add` / `delete` / `list` / `make` / recipe mutation
- no MYSHOP open / safebox password-open / refine confirm cube busy rejects yet
  (may share the same busy bit in a later tiny slice)
- no authored/template-backed overrides for busy-window strings
- no claim that cube crafting is playable

## Proof shape for the implementation slice

1. Runtime/session: `/open_cube` on requester → `EXCHANGE START` emits requester
   busy chat and no pairing.
2. Runtime/session: `/open_cube` on partner → requester `EXCHANGE START` emits
   partner busy chat and no pairing; partner cube stays open.
3. ACCEPT / second-accept / commit-time busy drift mirror the same strings with
   no mutation.
4. Docs frozen here; recipe mutation remains untouched.

## Status

Implemented on `lane/items`: open cube rejects `EXCHANGE START` / `ACCEPT` /
commit-time busy drift with the already-owned merchant/safebox/refine/MYSHOP
busy chat strings. MYSHOP/safebox/refine open/confirm cube rejects and guest
sell-into-PC-shop were still deferred when this note landed; both busy-reject
families and recipe make/add/list/cancel are now owned
(`docs/plans/2026-08-25-myshop-safebox-refine-cube-busy-rejects.md`,
`docs/plans/2026-08-26-cube-list-cancel.md`); complicated OR-materials /
binary cube headers stay deferred.
