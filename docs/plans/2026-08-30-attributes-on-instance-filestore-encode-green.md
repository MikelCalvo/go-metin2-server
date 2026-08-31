# Attributes-on-instance FileStore + encode GREEN — 2026-08-30

## Objective

Land the first presence-aware per-instance attributes substrate after the
docs/spec freeze, mirroring owned instance sockets for account FileStore and
live encode preference without inventing durable ground/safebox rematerialize,
SQL tip companions, or attribute gameplay.

## Contract owned by this slice

1. `inventory.ItemInstance` carries optional `Attributes *AttributeValues`
   (`ItemAttributeCount == 7`) with `HasAttributes` / `CloneAttributes` /
   `EffectiveAttributes` (including explicit all-zero / type-zero).
2. Account FileStore inventory/equipment JSON round-trips presence-aware
   attributes deterministically; omitted attributes keep template fallback.
3. Bootstrap `ITEM_SET` / `ITEM_UPDATE`, open-presentation `SAFEBOX_SET`, guest
   MYSHOP browse stock encode, and active-shell exchange `ITEM_ADD` prefer
   instance attributes when present; otherwise keep template-authored
   attributes.
4. Live drop → pickup clones preserve instance attributes in-memory the same
   way they already preserve sockets.
5. Move / equip / unequip / clone paths that already clone sockets also clone
   attributes.

## Explicit non-goals

- durable ground FileStore attribute rematerialize
- durable safebox cell FileStore attribute rematerialize
- tip-`0003` / `0010` / `0015` SQL attribute companions
- attribute gameplay / apply formulas / combat recomputation
- refine catalysts / mall / SAFEBOX_CHANGE_PASSWORD / GD/DB MYSHOP_PRICELIST

## Proof shape

1. Catalog/store: `TestFileStoreSaveThenLoadRoundTripInstanceAttributesIncludingZero`
2. Model: `TestItemInstanceEffectiveAttributesPreferInstancePresenceIncludingZero`
3. Encode: `TestInventoryItemUpdatePrefersInstanceAttributesIncludingExplicitZero`
4. Player/session: `TestRuntimeExchangeItemAddDisplayPrefersInstanceEffectiveAttributesWithoutMutation`
   and `TestGameRuntimeItemExchangeItemAddPrefersInstanceEffectiveAttributesWithoutMutation`

## Status

GREEN on `lane/items`. Follow-on durable ground-item / safebox cell attribute
rematerialize, tip-`0003`/`0010`/`0015` SQL companions, and seeded hermetic
drills later landed; carried inventory/equipment daemon-restart rematerialize
is owned by [carried item instance-attributes daemon-restart rematerialize](2026-08-31-carried-item-instance-attributes-daemon-restart-rematerialize.md).
