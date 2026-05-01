# Equipment-appearance bootstrap

This document freezes the first minimal link between owned equipped-item state and visible character appearance in `go-metin2-server`.

The goal of this slice is narrow:
- project equipped item state into the already-owned `parts` arrays carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- keep the behavior deterministic for the selected-character bootstrap burst and the current peer-visibility reuse path
- avoid claiming live equip/unequip fanout or final costume semantics too early

It does **not** yet define the full compatibility-grade appearance system.

## Scope

This first appearance slice currently applies only to:
- the selected character during the normal `ENTERGAME` bootstrap burst
- peer-visibility bursts that reuse the same visible-character packet builders
- visible part refresh values carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`

It does **not** yet apply to:
- live peer fanout when `/equip_item` or `/unequip_item` succeeds after bootstrap
- `CHARACTER_ADD`
- costume / transmutation semantics
- mount, affect, or combat-side appearance transitions

## Parts layout

The project already owns the visible-parts order carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`:
- `parts[0]` — armor/body
- `parts[1]` — weapon
- `parts[2]` — head
- `parts[3]` — hair

## Source-of-truth rules

The first owned bootstrap appearance projection uses only data that already exists on the selected character snapshot:
- base body appearance starts from `character.MainPart`
- base hair appearance starts from `character.HairPart`
- base weapon and head appearance start at `0`
- equipped item state comes from `character.Equipment`

The projection precedence is:
1. initialize visible parts from the persisted character base snapshot
2. if a valid equipped item occupies `body`, set `parts[0] = equipped_item.Vnum`
3. if a valid equipped item occupies `weapon`, set `parts[1] = equipped_item.Vnum`
4. if a valid equipped item occupies `head`, set `parts[2] = equipped_item.Vnum`
5. keep `parts[3]` pinned to `character.HairPart` in this first slice

This first slice deliberately keeps the data path simple:
- no item-template lookup is required
- no extra appearance metadata is required
- the equipped item `vnum` is written directly into the visible part slot for `body`, `weapon`, and `head`

## Packet impact

When a character has equipped `body`, `weapon`, or `head` items in the persisted bootstrap snapshot:
- `CHAR_ADDITIONAL_INFO` must expose those projected part values
- `CHARACTER_UPDATE` must expose the same projected part values
- both self-bootstrap and peer-visibility bursts must agree because they reuse the same projection helper

`CHARACTER_ADD` remains unchanged in this slice.

## Explicit non-goals

This slice does **not** yet freeze:
- live appearance fanout after bootstrap-time `/equip_item` / `/unequip_item`
- `hair` equipped-item projection over `parts[3]`
- shield, arrow, unique-slot, necklace, bracelet, or shoes appearance semantics
- costume, transmutation, refine-glow, or affect overlays
- validation or repair behavior for manually-corrupted snapshots containing duplicate equipped slots

## Success definition

After this slice, the repository should be able to say:
- bootstrap visible-character packets no longer ignore equipped `body`, `weapon`, and `head` items
- self-bootstrap and peer-visibility bursts project the same deterministic appearance values from the persisted equipped-item snapshot
- the repo owns an explicit written contract for what the current bootstrap runtime does before live appearance fanout slices land
