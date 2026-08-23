# Equip Occupied Wear-Slot Swap / Replace Contract Freeze — 2026-08-23

## Objective

Freeze the first occupied-wear swap/replace mutation contract before opening RED, so `ITEM_MOVE` / `/equip_item` onto an already-occupied authored wear cell can stop at the owned reject chat and grow into oracle-shaped replace without inventing effect-invert ordering, source-cell placement, quickslot sync, or busy-window teardown mid-implementation.

## Why docs-first

Occupied wear currently emits self-only `CHAT_TYPE_INFO` `You are already wearing equipment.` and mutates nothing. The external EquipItem / MoveItem oracle auto-equip path unequips the worn item onto the carried source cell, lands the new wearable on the wear cell, inverts the old `equip_effect`, and applies the new one. Opening RED without freezing source-cell occupancy, effect invert/apply ordering, persistence boundaries, and busy-window teardown would invent policy. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Scope: only packet `ITEM_MOVE` and slash `/equip_item` that already pass the owned empty-slot equip guards (valid source carried wearable, authored `equip_slot` / wear cell, template restrictions / `equip_reject_message` fail-closed paths unchanged) and would currently hit the occupied-wear reject chat because the destination wear cell already has exactly one live occupant.
2. Success mutation (atomic live + persist):
   - remember previous worn item + previous/new template effects;
   - move the worn item onto the carried source cell (preserving worn item identity and count);
   - place the source wearable onto the destination wear cell (preserving source item identity and count);
   - invert the previous template `equip_effect` point deltas, then apply the new template `equip_effect` point deltas through the already-owned equip/unequip effect helpers / carrier bounds;
   - persist inventory/equipment/quickslots/points together;
   - emit self-only frames in this order: unequip/equip inventory+equipment refresh (`ITEM_SET` / `ITEM_DEL` as already owned by empty-slot equip/unequip), any point `PLAYER_POINT_CHANGE` frames from the net effect transition, and any already-owned item-removal / rebind quickslot sync required by the source-cell change;
   - write failure rolls back live inventory/equipment/points fail-closed with no frames.
3. Fail-closed (keep occupied reject chat or silent consume — do not partially swap):
   - destination wear cell empty → keep existing empty-slot equip success path;
   - destination wear cell has multiple/ambiguous occupants → keep occupied reject chat / no mutation;
   - source cell cannot accept the worn item (occupied incompatible / locked / over-max / out of carried range) → keep occupied reject chat `You are already wearing equipment.` with no mutation (do not invent a second swap-specific chat string in this bootstrap slice);
   - effect invert/apply would overflow/underflow the owned signed point carrier → fail-closed with no frames / no mutation;
   - death-floor / missing selected character → fail-closed as today;
   - active bootstrap exchange shell on success teardown: emit self/peer `GC::EXCHANGE END` before refresh frames, matching accepted equip success / occupied-reject teardown ordering already owned elsewhere.
4. Merchant / safebox / refine busy windows do not block this swap in this bootstrap slice beyond already-owned exchange teardown; do not invent partner player-shop / cube busy rejects here.
5. Spec/QA name swap/replace beside the occupied reject chat; template-authored override text for occupied reject and dragon-soul / belt / costume-only occupied policy stay deferred.

## Locale / wording note

Successful swap emits no new info-chat string. The occupied reject chat `You are already wearing equipment.` remains the only project-owned English string for non-swappable occupied destinations (including source-cell-cannot-accept-worn).

## What this is not yet

- template-authored override text for occupied-wear reject
- dragon-soul / belt / costume-only occupied policy beyond currently owned wear indices
- mall open/checkout / TMP4 CG `SAFEBOX_MONEY`
- client `SAFEBOX_CHANGE_PASSWORD` packets
- partner-side open player-shop / cube exchange busy rejects

## TDD shape after the freeze lands

1. Player unit: occupied wear + empty compatible source cell swaps items and applies net equip_effect transition; incompatible / locked source-cell destination for the worn item stays on occupied reject chat with no mutation; effect carrier overflow stays fail-closed.
2. Runtime/session: packet `ITEM_MOVE` and `/equip_item` emit the refresh/point/quickslot burst; active exchange shell closes before refresh on success; reconnect/restart rematerializes swapped equipment/inventory/points.
3. Negative: multi-occupant / missing template / death-floor remain fail-closed without partial swap.

## Status

Implemented on `lane/items`: occupied-wear swap/replace is owned for packet `ITEM_MOVE` / `/equip_item` with effect invert/apply, persistence, exchange teardown, and non-swappable reject chat. Mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.
