# Equip Empire / Min-Level Restriction Feedback — 2026-08-22

## Objective

Freeze that packet/slash equip already resolves selected-character empire anti-flags and `min_level` through the same `CanUseTemplate` / `equip_reject_message` path owned for job/sex and transfer guards, instead of leaving equipment docs/QA naming only job/sex.

## Contract owned by this slice

1. Authored equipment templates with `anti_empire_a` / `anti_empire_b` / `anti_empire_c` or `min_level` that the selected character fails continue to fail closed for packet `ITEM_MOVE` equip and slash `/equip_item` with no inventory/equipment/point/quickslot/persistence mutation.
2. When those same guards author non-empty `equip_reject_message`, the runtime emits one self-only `CHAT_TYPE_INFO` with that text (and still closes an open exchange shell before the chat when that presentation is active).
3. Spec/QA name empire anti-flags and `min_level` beside the already-owned job/sex / transfer-guard equip rejection matrix.

## What this is not yet

- new equip rejection packet families
- peer-facing equip rejection text
- broader equipment slot policy beyond the currently owned authored `equip_slot` match
- refine failure / safebox password / exchange player-shop busy seams

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'EquipItemWithTemplateRejectsSelectedCharacter' -count=1`
- `go test ./internal/minimal -run 'ItemMoveEquipRejectsTemplateAntiFlags|ItemMoveEquipRejects.*Empire|ItemMoveEquipRejects.*MinLevel|SlashEquipRejects.*Empire|SlashEquipRejects.*MinLevel' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optional: fold empire/`min_level` into the broader silent equip anti-flag matrix naming in packet-matrix if that table still says job/sex only.
2. Keep refine `probability < 100` failure / `RefineFailed` deferred.
3. Keep partner-side open player-shop / cube exchange busy rejects deferred until those presentation seams exist.
