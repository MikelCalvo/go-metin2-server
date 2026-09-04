# Equip / occupied-replace equip-side presence preserve — 2026-09-04

## Objective

Freeze the next Track C honesty seam after whole-stack safebox relocate
presence preserve landed: empty wear-slot `EquipItem` / `EquipItemWithTemplate`
and the equip-side of occupied-wear `ReplaceOccupiedEquipItem*` must place an
**independent** presence-aware sockets/attributes clone of the carried source
onto the worn equipment cell (including explicit zero; omit→omit), matching the
already-owned unequip `WithInventorySlot` clone path and the carried/safebox/
drop clone helpers.

Today `equipItem` and the equip side of `replaceOccupiedEquipItem` assign
`equippedItem := sourceItem` (or `item := liveInventory[fromIndex]`) and only
rewrite slot/equipped metadata. Because sockets and attributes are pointer
fields, the worn cell can alias the pre-equip carried snapshot / helper input.
Unequip already clones via `WithInventorySlot`, so the asymmetry is
equip-direction only. Focused proofs still cover slot policy / effects /
occupied-reject without presence independence.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary equip path —
the next client-visible honesty twin after the safebox presence chain closed.
Opening RED without freezing:

- that empty wear-slot equip and occupied-replace equip-side are in scope,
- that unequip / replace unequip-side already clone via `WithInventorySlot`,
- how omitted vs explicit-zero presence behaves after equip,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred. Do not reopen safebox / exchange / drop preserve
contracts.

## Contract to freeze (before RED)

1. **Empty wear-slot equip**: when accepted `EquipItem` /
   `EquipItemWithTemplate` (or packet/slash equip onto an empty authored wear
   cell) removes a carried stack into equipment, the worn `ItemInstance` must:
   - keep the source item identity and count;
   - set equipped metadata / authored wear slot as already owned;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the worn cell
       (including explicit `{0,0,0}`) so later writes cannot alias the pre-equip
       carried snapshot;
     - if the source `HasAttributes()`, clone those attributes onto the worn
       cell (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the worn cell likewise
       omits them (later encode keeps template fallback).
2. **Occupied-wear replace equip-side**: when accepted
   `ReplaceOccupiedEquipItem` / `ReplaceOccupiedEquipItemWithTemplates` swaps a
   carried wearable onto an occupied wear cell, the newly worn `EquippedItem`
   must follow the same independent presence-clone contract. The unequipped
   carried destination continues to clone via `WithInventorySlot` as already
   owned.
3. **Source of truth**: the pre-equip carried stack already validated for equip;
   inventory removal / swap stays as already owned.
4. **Wire honesty**: equip-burst `ITEM_SET` / equipment refresh frames must
   project presence-aware instance sockets/attributes via ordinary
   `EffectiveSockets` / `EffectiveAttributes`.
5. **Persistence**: the selected-character account snapshot after successful
   equip / occupied replace must round-trip independent presence-aware fields on
   the worn cell (including explicit zero).
6. **Non-goals**: inventing new equip reject/effect policy, refine catalysts /
   mall / party ownership notices, safebox/exchange/drop reopen, or changing
   unequip clone behavior already owned by `WithInventorySlot`.

## Proof shape (for later RED → GREEN)

1. Unit: seed a carried wearable with authoritative instance presence (active /
   explicit-zero / omitted) → `EquipItemWithTemplate` onto an empty wear cell →
   worn identity preserved; worn presence is an independent clone; mutating the
   worn clone leaves the seed pointer unchanged; omitted stays omitted.
2. Unit twin: occupied-replace equip-side likewise clones independently while
   unequipped destination stays on `WithInventorySlot`.
3. Session: packet/slash equip of a presence-bearing wearable → equipment
   `ITEM_SET` + account snapshot carry preserved presence; omitted stays
   omitted / template-fallback encode.
4. Negatives: occupied reject without replace, anti-flag / slot-mismatch /
   irremovable rejects stay non-mutating as already owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/player/runtime.go` (`equipItem` / `replaceOccupiedEquipItem` equip
  side)
- `internal/player/runtime_inventory_test.go` or focused equip preserve twin
- `internal/minimal` session equip preserve twin if needed
- `spec/protocol/item-move-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/player -run 'EquipItem.*PreservesInstance|ReplaceOccupiedEquip.*PreservesInstance' -count=1
go test ./internal/minimal -run 'Equip.*PreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: empty wear-slot `EquipItem` / `EquipItemWithTemplate`
and occupied-replace equip-side clone presence-aware sockets/attributes
independently from the pre-equip live inventory pointer (including explicit
zero; omit→omit)
(`TestRuntimeEquipItemWithTemplatePreservesInstancePresenceIndependently`,
`TestRuntimeReplaceOccupiedEquipItemPreservesEquippedInstancePresenceIndependently`).
Session twins now also freeze slash empty-slot equip and packet occupied-replace
equipment `ITEM_SET` + account snapshot presence
(`TestGameRuntimeEquipPreservesInstanceSocketsAndAttributes`,
`TestGameRuntimeOccupiedReplaceEquipPreservesInstanceSocketsAndAttributes`).
Unequip/`WithInventorySlot` clone stays already owned. Refine catalysts / mall /
party ownership remain deferred.
