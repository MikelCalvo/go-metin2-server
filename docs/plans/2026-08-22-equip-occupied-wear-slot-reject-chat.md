# Equip Occupied Wear-Slot Reject Chat — 2026-08-22

## Objective

Make `ITEM_MOVE` / `/equip_item` into an already-occupied authored wear cell client-visible by emitting one self-only `CHAT_TYPE_INFO` instead of the older silent fail-closed path, without yet owning occupied-wear swap / replace mutation.

## Why this slice

Empty-slot equip, template guards, and unequip-to-empty are already owned. Dragging a second wearable onto the same wear cell still mutates nothing and emits no frames, which blocks ordinary gear-swap QA. The external MoveItem oracle rejects an occupied equipment destination with info-chat before mutation; the separate EquipItem auto-equip swap path stays deferred until a later replace contract freeze.

## Contract owned by this slice

1. When packet `ITEM_MOVE` or slash `/equip_item` targets an owned wear cell that already has exactly one (or more) live occupant, and the request would otherwise reach the empty-slot equip mutation after the already-owned template/source guards, the runtime emits one self-only `CHAT_TYPE_INFO` with the deterministic project-owned text `You are already wearing equipment.` (`vid = 0`).
2. No inventory/equipment/quickslot/point/appearance/persistence mutation occurs; the worn item and carried source stay unchanged.
3. If a bootstrap exchange shell is open on the same socket, emit self/peer `GC::EXCHANGE END` before that occupied reject chat, matching the already-owned equip-reject-feedback teardown ordering.
4. Empty-slot equip success, template-authored `equip_reject_message`, silent omit-text restriction rejects, locked/missing/mismatched source rejects, and unequip paths stay unchanged.
5. Occupied-wear swap/replace (invert old `equip_effect`, land worn item on the source carried cell, apply new effect, quickslot sync) remains deferred. Do not invent a new `GC::EXCHANGE`-style result packet or a template-authored override field for this occupied string in this slice.

## Locale / wording note

The MoveItem oracle emits `LC_TEXT("이미 장비를 착용하고 있습니다.")`. The reference English locale pair for that Korean key is incorrectly mapped to Dragon Stone wording, so this bootstrap slice freezes the project-owned English fallback `You are already wearing equipment.` rather than copying that mismatched locale pair.

## What this is not yet

- occupied-wear swap/replace mutation through MoveItem or EquipItem
- dragon-soul / belt / costume-only occupied policy beyond currently owned wear indices
- template-authored override text for the occupied-wear reject
- safebox password / money / mall
- partner-side open player-shop / cube exchange busy rejects

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'EquipItemRejectsOccupiedWearSlot|EquipItemWithTemplateRejectsOccupiedWearSlot' -count=1`
- `go test ./internal/minimal -run 'ItemMoveEquipRejectsOccupiedWearSlot|SlashEquipRejectsOccupiedWearSlot|ItemMoveEquipOccupiedRejectClosesActiveExchangeShell' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Freeze then implement occupied-wear swap/replace through the EquipItem oracle path (unequip worn → source cell, equip new, effect invert/apply).
2. Keep safebox password challenge as a separate docs-first storage slice.
3. Keep partner-side open player-shop / cube exchange busy rejects deferred until those presentation seams exist.

## Status

Shipped on `lane/items`: packet `/equip_item` and `ITEM_MOVE` occupied-wear reject chat with exchange teardown; swap/replace stays deferred.
